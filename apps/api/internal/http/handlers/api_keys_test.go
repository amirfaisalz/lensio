package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

type mockAPIKeyStore struct {
	keys       map[string]*store.APIKey
	createErr  error
	listErr    error
	revokeErr  error
	touchedIDs []string
}

func newMockStore() *mockAPIKeyStore {
	return &mockAPIKeyStore{
		keys: make(map[string]*store.APIKey),
	}
}

func (m *mockAPIKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	if m.createErr != nil {
		return m.createErr
	}
	if key.ID == "" {
		key.ID = "generated-uuid-1"
	}
	key.CreatedAt = time.Now().UTC()
	m.keys[key.ID] = key
	return nil
}

func (m *mockAPIKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	for _, k := range m.keys {
		if k.KeyHash == keyHash {
			return k, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockAPIKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var res []*store.APIKey
	for _, k := range m.keys {
		if k.OrgID == orgID {
			res = append(res, k)
		}
	}
	return res, nil
}

func (m *mockAPIKeyStore) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	k, ok := m.keys[keyID]
	if !ok || k.OrgID != orgID || k.RevokedAt != nil {
		return store.ErrNotFound
	}
	now := time.Now().UTC()
	k.RevokedAt = &now
	return nil
}

func (m *mockAPIKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	m.touchedIDs = append(m.touchedIDs, keyID)
	return nil
}

func TestCreateAPIKeyHandler_Success(t *testing.T) {
	s := newMockStore()
	handler := handlers.CreateAPIKeyHandler(s, handlers.DefaultOrgID)

	payload := map[string]any{
		"name": "Production Service",
	}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp handlers.CreateKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if resp.Name != "Production Service" {
		t.Errorf("expected name 'Production Service', got %q", resp.Name)
	}
	if resp.Environment != "live" {
		t.Errorf("expected environment 'live', got %q", resp.Environment)
	}
	if len(resp.Scopes) != 3 {
		t.Errorf("expected 3 default scopes, got %d", len(resp.Scopes))
	}
	if resp.OrgID != handlers.DefaultOrgID {
		t.Errorf("expected default org %s, got %s", handlers.DefaultOrgID, resp.OrgID)
	}
	if _, valid := apikey.ValidateFormat(resp.Key); !valid {
		t.Errorf("expected valid API key plaintext format, got %q", resp.Key)
	}
}

func TestCreateAPIKeyHandler_CustomScopesAndEnv(t *testing.T) {
	s := newMockStore()
	handler := handlers.CreateAPIKeyHandler(s, handlers.DefaultOrgID)

	payload := map[string]any{
		"name":        "Test Runner",
		"environment": "test",
		"scopes":      []string{"ocr:read"},
		"org_id":      "org-custom-123",
	}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp handlers.CreateKeyResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Environment != "test" {
		t.Errorf("expected test environment, got %s", resp.Environment)
	}
	if len(resp.Scopes) != 1 || resp.Scopes[0] != "ocr:read" {
		t.Errorf("expected ['ocr:read'], got %+v", resp.Scopes)
	}
	if resp.OrgID != "org-custom-123" {
		t.Errorf("expected org-custom-123, got %s", resp.OrgID)
	}
}

func TestCreateAPIKeyHandler_AuthKeyOrgResolution(t *testing.T) {
	s := newMockStore()
	handler := handlers.CreateAPIKeyHandler(s, handlers.DefaultOrgID)

	payload := map[string]any{
		"name": "Service Key",
	}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))
	req = req.WithContext(middleware.WithAPIKey(req.Context(), &store.APIKey{
		ID:    "parent-key",
		OrgID: "org-from-auth-context",
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp handlers.CreateKeyResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.OrgID != "org-from-auth-context" {
		t.Errorf("expected org from context, got %s", resp.OrgID)
	}
}

func TestCreateAPIKeyHandler_ValidationErrors(t *testing.T) {
	s := newMockStore()
	handler := handlers.CreateAPIKeyHandler(s, handlers.DefaultOrgID)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid json",
			body:       "{malformed",
			wantStatus: http.StatusBadRequest,
			wantCode:   response.CodeInvalidRequest,
		},
		{
			name:       "missing name",
			body:       `{"name": "   "}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   response.CodeInvalidRequest,
		},
		{
			name:       "invalid environment",
			body:       `{"name": "test", "environment": "production"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   response.CodeInvalidRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewBufferString(tc.body))

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
			var envelope response.ErrorEnvelope
			_ = json.NewDecoder(rec.Body).Decode(&envelope)
			if envelope.Error.Code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, envelope.Error.Code)
			}
		})
	}
}

func TestCreateAPIKeyHandler_StoreError(t *testing.T) {
	s := newMockStore()
	s.createErr = errors.New("db disk full")
	handler := handlers.CreateAPIKeyHandler(s, handlers.DefaultOrgID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewBufferString(`{"name": "test"}`))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestListAPIKeysHandler(t *testing.T) {
	s := newMockStore()
	k1 := &store.APIKey{
		ID:          "k1",
		OrgID:       handlers.DefaultOrgID,
		Name:        "Key 1",
		Prefix:      "nusa_live_1234",
		Scopes:      []string{"ocr:write"},
		Environment: "live",
	}
	_ = s.CreateAPIKey(context.Background(), k1)

	handler := handlers.ListAPIKeysHandler(s, handlers.DefaultOrgID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data []handlers.APIKeyListItem `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding list response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data))
	}
	if resp.Data[0].MaskedKey != "nusa_live_1234••••••••" {
		t.Errorf("unexpected masked key: %s", resp.Data[0].MaskedKey)
	}
}

func TestListAPIKeysHandler_Error(t *testing.T) {
	s := newMockStore()
	s.listErr = errors.New("db error")
	handler := handlers.ListAPIKeysHandler(s, handlers.DefaultOrgID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRevokeAPIKeyHandler_Success(t *testing.T) {
	s := newMockStore()
	k := &store.APIKey{
		ID:          "k-revoke-me",
		OrgID:       handlers.DefaultOrgID,
		Name:        "To Revoke",
		KeyHash:     "hash123",
		Prefix:      "nusa_live_1234",
		Environment: "live",
	}
	_ = s.CreateAPIKey(context.Background(), k)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, handlers.DefaultOrgID))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/k-revoke-me", nil)

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeAPIKeyHandler_NotFound(t *testing.T) {
	s := newMockStore()
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, handlers.DefaultOrgID))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/non-existent-id", nil)

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRevokeAPIKeyHandler_Errors(t *testing.T) {
	t.Run("missing path value", func(t *testing.T) {
		s := newMockStore()
		handler := handlers.RevokeAPIKeyHandler(s, handlers.DefaultOrgID)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/", nil)

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		s := newMockStore()
		s.revokeErr = errors.New("db error")
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, handlers.DefaultOrgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/some-id", nil)

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}
