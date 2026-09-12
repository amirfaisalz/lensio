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

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/authz"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
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

type mockAuditStore struct {
	recorded []*store.AuditLog
}

func (m *mockAuditStore) RecordAuditLog(ctx context.Context, log *store.AuditLog) error {
	m.recorded = append(m.recorded, log)
	return nil
}

func (m *mockAuditStore) ListAuditLogsByOrg(ctx context.Context, orgID string) ([]*store.AuditLog, error) {
	return m.recorded, nil
}

func TestCreateAPIKeyHandler_Success(t *testing.T) {
	s := newMockStore()
	audit := &mockAuditStore{}
	handler := handlers.CreateAPIKeyHandler(s, audit, nil, handlers.DefaultOrgID)

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
	handler := handlers.CreateAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID)

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
	handler := handlers.CreateAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID)

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
	handler := handlers.CreateAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID)

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
	handler := handlers.CreateAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID)

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
		Prefix:      "lensio_live_1234",
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
	if resp.Data[0].MaskedKey != "lensio_live_1234••••••••" {
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
		Prefix:      "lensio_live_1234",
		Environment: "live",
	}
	_ = s.CreateAPIKey(context.Background(), k)

	mux := http.NewServeMux()
	audit := &mockAuditStore{}
	mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, audit, nil, handlers.DefaultOrgID))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/k-revoke-me", nil)

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if len(audit.recorded) != 1 || audit.recorded[0].Action != "api_key.revoke" {
		t.Errorf("expected 1 api_key.revoke audit log, got %+v", audit.recorded)
	}
}

func TestRevokeAPIKeyHandler_NotFound(t *testing.T) {
	s := newMockStore()
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID))

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
		handler := handlers.RevokeAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID)

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
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, nil, handlers.DefaultOrgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/some-id", nil)

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestCreateAPIKeyHandler_SpiceDBAuthorization(t *testing.T) {
	orgID := "00000000-0000-0000-0000-000000000001"
	projRes := authz.NewResource("project", orgID)

	t.Run("missing actor returns 401", func(t *testing.T) {
		s := newMockStore()
		authorizer := authz.NewMockAuthorizer()
		handler := handlers.CreateAPIKeyHandler(s, nil, authorizer, orgID)

		body, _ := json.Marshal(map[string]any{"name": "No Actor Key"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		var env response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&env)
		if env.Error.Code != response.CodeInvalidAPIKey {
			t.Errorf("expected code %s, got %s", response.CodeInvalidAPIKey, env.Error.Code)
		}
	})

	t.Run("unauthorized actor returns 403", func(t *testing.T) {
		s := newMockStore()
		authorizer := authz.NewMockAuthorizer()
		authorizer.SetDefaultAllow(false)
		handler := handlers.CreateAPIKeyHandler(s, nil, authorizer, orgID)

		body, _ := json.Marshal(map[string]any{"name": "Unauthorized Key"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))
		ctx := middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "unauthorized-user"})
		req = req.WithContext(ctx)

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		var env response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&env)
		if env.Error.Code != response.CodePermissionDenied {
			t.Errorf("expected code %s, got %s", response.CodePermissionDenied, env.Error.Code)
		}
	})

	t.Run("authorizer failure returns 500", func(t *testing.T) {
		s := newMockStore()
		authorizer := authz.NewMockAuthorizer()
		authorizer.SetCheckError(errors.New("spicedb connection timeout"))
		handler := handlers.CreateAPIKeyHandler(s, nil, authorizer, orgID)

		body, _ := json.Marshal(map[string]any{"name": "Error Key"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))
		ctx := middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "admin"})
		req = req.WithContext(ctx)

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Error, got %d", rec.Code)
		}
	})

	t.Run("authorized actor via OIDC user succeeds and writes relationships", func(t *testing.T) {
		s := newMockStore()
		audit := &mockAuditStore{}
		authorizer := authz.NewMockAuthorizer()

		adminSubject := authz.NewSubject("user", "admin_lensio_dev")
		authorizer.Allow(projRes, "manage_api_keys", adminSubject)

		handler := handlers.CreateAPIKeyHandler(s, audit, authorizer, orgID)

		body, _ := json.Marshal(map[string]any{
			"name":        "Production Service Key",
			"environment": "live",
			"scopes":      []string{"ocr:write"},
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{
			Subject: "admin_lensio_dev",
			Email:   "admin@lensio.dev",
			Roles:   []string{"admin"},
		}))

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp handlers.CreateKeyResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding create key response: %v", err)
		}
		if resp.ID == "" || resp.Key == "" {
			t.Fatalf("expected valid id and key in response: %+v", resp)
		}

		// Verify SpiceDB relationships were written
		relationships := authorizer.GetWrittenRelationships()
		if len(relationships) != 2 {
			t.Fatalf("expected 2 relationships written to SpiceDB, got %d: %+v", len(relationships), relationships)
		}

		var foundProject, foundCreator bool
		for _, rel := range relationships {
			if rel.Resource.Type == "api_key" && rel.Resource.ID == resp.ID {
				if rel.Relation == "project" && rel.Subject.Type == "project" && rel.Subject.ID == orgID {
					foundProject = true
				}
				if rel.Relation == "creator" && rel.Subject.Type == "user" && rel.Subject.ID == "admin_lensio_dev" {
					foundCreator = true
				}
			}
		}
		if !foundProject {
			t.Error("expected api_key->project relationship recorded in SpiceDB")
		}
		if !foundCreator {
			t.Error("expected api_key->creator relationship recorded in SpiceDB")
		}

		// Verify audit log recorded actor
		if len(audit.recorded) != 1 || audit.recorded[0].ActorID != "user:admin_lensio_dev" {
			t.Errorf("expected audit actor 'user:admin_lensio_dev', got %+v", audit.recorded)
		}
	})

	t.Run("arbitrary X-Actor-ID or X-User-ID header without context authentication is rejected with 401", func(t *testing.T) {
		s := newMockStore()
		authorizer := authz.NewMockAuthorizer()
		authorizer.Allow(projRes, "manage_api_keys", authz.NewSubject("user", "lead_dev"))
		handler := handlers.CreateAPIKeyHandler(s, nil, authorizer, orgID)

		body, _ := json.Marshal(map[string]any{"name": "Dev Key"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(body))
		req.Header.Set("X-User-ID", "lead_dev")
		req.Header.Set("X-Actor-ID", "lead_dev")

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for untrusted headers, got %d", rec.Code)
		}
	})
}

func TestRevokeAPIKeyHandler_SpiceDBAuthorization(t *testing.T) {
	orgID := "00000000-0000-0000-0000-000000000001"
	keyID := "key-rebac-to-revoke"

	setupKey := func() *mockAPIKeyStore {
		s := newMockStore()
		_ = s.CreateAPIKey(context.Background(), &store.APIKey{
			ID:          keyID,
			OrgID:       orgID,
			Name:        "Key to Revoke",
			KeyHash:     "hash_rebac",
			Prefix:      "lensio_live_test",
			Environment: "live",
		})
		return s
	}

	t.Run("missing actor returns 401", func(t *testing.T) {
		s := setupKey()
		authorizer := authz.NewMockAuthorizer()
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("arbitrary X-Actor-ID header on revoke is rejected with 401 without auth context", func(t *testing.T) {
		s := setupKey()
		authorizer := authz.NewMockAuthorizer()
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		req.Header.Set("X-Actor-ID", "admin")
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for spoofed X-Actor-ID header, got %d", rec.Code)
		}
	})

	t.Run("unauthorized actor returns 403", func(t *testing.T) {
		s := setupKey()
		authorizer := authz.NewMockAuthorizer()
		authorizer.SetDefaultAllow(false)
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "intruder"}))
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("authorizer failure returns 500", func(t *testing.T) {
		s := setupKey()
		authorizer := authz.NewMockAuthorizer()
		authorizer.SetCheckError(errors.New("spicedb network error"))
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "admin"}))
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Error, got %d", rec.Code)
		}
	})

	t.Run("key creator has revoke permission and succeeds", func(t *testing.T) {
		s := setupKey()
		audit := &mockAuditStore{}
		authorizer := authz.NewMockAuthorizer()

		creatorSubject := authz.NewSubject("user", "creator-uuid-1")
		_ = authorizer.WriteRelationship(context.Background(), authz.NewRelationship(
			authz.NewResource("api_key", keyID),
			"creator",
			creatorSubject,
		))

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, audit, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{
			Subject: "creator-uuid-1",
			Email:   "creator@lensio.dev",
		}))
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if len(audit.recorded) != 1 || audit.recorded[0].ActorID != "user:creator-uuid-1" {
			t.Errorf("unexpected audit log: %+v", audit.recorded)
		}
	})

	t.Run("project admin fallback succeeds", func(t *testing.T) {
		s := setupKey()
		authorizer := authz.NewMockAuthorizer()

		adminSubject := authz.NewSubject("user", "admin-user")
		// Project admin has manage_api_keys on the parent project
		authorizer.Allow(authz.NewResource("project", orgID), "manage_api_keys", adminSubject)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(s, nil, authorizer, orgID))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+keyID, nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "admin-user"}))
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK via project admin fallback, got %d", rec.Code)
		}
	})
}

type mockAPIKeyAndAccountStore struct {
	*mockAPIKeyStore
	orgs     map[string]*store.Organization
	userOrgs map[string]*store.Organization
}

func (m *mockAPIKeyAndAccountStore) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	if org, ok := m.orgs[orgID]; ok {
		return org, nil
	}
	return nil, store.ErrNotFound
}

func (m *mockAPIKeyAndAccountStore) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	return nil
}

func (m *mockAPIKeyAndAccountStore) GetOrganizationMembers(ctx context.Context, orgID string) ([]store.User, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*store.User, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) GetUserByEmail(ctx context.Context, email string) (*store.UserWithAuth, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) GetUserByID(ctx context.Context, userID string) (*store.User, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) VerifyUserEmail(ctx context.Context, email, token string) error {
	return nil
}

func (m *mockAPIKeyAndAccountStore) CreateOrganization(ctx context.Context, name, slug, planCode string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAPIKeyAndAccountStore) AssignUserToOrg(ctx context.Context, userID, orgID, role string) error {
	return nil
}

func (m *mockAPIKeyAndAccountStore) GetUserOrganization(ctx context.Context, userID string) (*store.Organization, error) {
	if org, ok := m.userOrgs[userID]; ok {
		return org, nil
	}
	return nil, store.ErrNotFound
}

func TestAPIKeyHandlers_WithAccountStore(t *testing.T) {
	baseStore := newMockStore()
	validOrg := &store.Organization{
		ID:   "org-uuid-123",
		Name: "Test Company",
		Slug: "test-co",
	}
	as := &mockAPIKeyAndAccountStore{
		mockAPIKeyStore: baseStore,
		orgs: map[string]*store.Organization{
			"org-uuid-123": validOrg,
		},
		userOrgs: map[string]*store.Organization{
			"user-uuid-999": validOrg,
		},
	}

	t.Run("CreateAPIKey - resolves org from user when req.OrgID is empty", func(t *testing.T) {
		body := map[string]any{
			"name":        "OIDC Session Key",
			"environment": "live",
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(b))
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "user-uuid-999"}))
		rec := httptest.NewRecorder()

		handlers.CreateAPIKeyHandler(as, nil, nil, "")(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp handlers.CreateKeyResponse
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		if resp.OrgID != "org-uuid-123" {
			t.Errorf("expected org ID %q, got %q", "org-uuid-123", resp.OrgID)
		}
	})

	t.Run("CreateAPIKey - returns 400 when explicit org not found in account store", func(t *testing.T) {
		body := map[string]any{
			"name":        "Invalid Org Key",
			"environment": "live",
			"org_id":      "org-non-existent",
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(b))
		rec := httptest.NewRecorder()

		handlers.CreateAPIKeyHandler(as, nil, nil, "")(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateAPIKey - returns 400 when org cannot be resolved", func(t *testing.T) {
		body := map[string]any{
			"name":        "No Org Key",
			"environment": "live",
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(b))
		// user with no organization
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "orphan-user"}))
		rec := httptest.NewRecorder()

		handlers.CreateAPIKeyHandler(as, nil, nil, "")(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ListAPIKeys - resolves org from user when query param empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "user-uuid-999"}))
		rec := httptest.NewRecorder()

		handlers.ListAPIKeysHandler(as, "")(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ListAPIKeys - returns empty list when org cannot be resolved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "orphan-user"}))
		rec := httptest.NewRecorder()

		handlers.ListAPIKeysHandler(as, "")(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		var result map[string][]handlers.APIKeyListItem
		_ = json.NewDecoder(rec.Body).Decode(&result)
		if len(result["data"]) != 0 {
			t.Errorf("expected empty data list, got %d items", len(result["data"]))
		}
	})

	t.Run("RevokeAPIKey - resolves org from user when query param empty", func(t *testing.T) {
		// First create a key in org-uuid-123
		key := &store.APIKey{
			OrgID:       "org-uuid-123",
			Name:        "To Revoke",
			KeyHash:     "hash-revoke-1",
			Prefix:      "lensio_live_rev1",
			Environment: "live",
		}
		_ = as.CreateAPIKey(context.Background(), key)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(as, nil, nil, ""))

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+key.ID, nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{Subject: "user-uuid-999"}))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})
}

