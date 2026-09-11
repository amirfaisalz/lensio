package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

type mockKeyStore struct {
	mu           sync.Mutex
	keysByHash   map[string]*store.APIKey
	lastUsedByID map[string]time.Time
	err          error
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{
		keysByHash:   make(map[string]*store.APIKey),
		lastUsedByID: make(map[string]time.Time),
	}
}

func (m *mockKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keysByHash[key.KeyHash] = key
	return nil
}

func (m *mockKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	key, ok := m.keysByHash[keyHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return key, nil
}

func (m *mockKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	return nil, nil
}

func (m *mockKeyStore) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	return nil
}

func (m *mockKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastUsedByID[keyID] = lastUsed
	return nil
}

func TestAuthenticate_Success(t *testing.T) {
	kStore := newMockKeyStore()
	gen, _ := apikey.Generate(apikey.EnvLive)

	key := &store.APIKey{
		ID:          "key-123",
		OrgID:       "org-456",
		Name:        "Test Key",
		KeyHash:     gen.KeyHash,
		Prefix:      gen.Prefix,
		Scopes:      []string{"ocr:write"},
		Environment: "live",
	}
	_ = kStore.CreateAPIKey(context.Background(), key)

	var capturedKey *store.APIKey
	handler := middleware.Authenticate(kStore)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = middleware.GetAPIKey(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+gen.Plaintext)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if capturedKey == nil || capturedKey.ID != "key-123" {
		t.Fatalf("expected key-123 in context, got %+v", capturedKey)
	}

	// Wait briefly for asynchronous touch
	time.Sleep(50 * time.Millisecond)
	kStore.mu.Lock()
	_, touched := kStore.lastUsedByID["key-123"]
	kStore.mu.Unlock()
	if !touched {
		t.Error("expected key last_used_at to be touched asynchronously")
	}
}

func TestAuthenticate_Failures(t *testing.T) {
	kStore := newMockKeyStore()
	genValid, _ := apikey.Generate(apikey.EnvLive)
	genRevoked, _ := apikey.Generate(apikey.EnvLive)
	genExpired, _ := apikey.Generate(apikey.EnvLive)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:        "revoked-key",
		KeyHash:   genRevoked.KeyHash,
		RevokedAt: &now,
	})

	_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:        "expired-key",
		KeyHash:   genExpired.KeyHash,
		ExpiresAt: &past,
	})

	tests := []struct {
		name       string
		authHeader string
		storeErr   error
		wantCode   int
		wantSub    string
	}{
		{
			name:       "missing authorization header",
			authHeader: "",
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "wrong auth scheme (basic)",
			authHeader: "Basic dXNlcjpwYXNz",
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "empty bearer token",
			authHeader: "Bearer   ",
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "unknown key token",
			authHeader: "Bearer " + genValid.Plaintext,
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "revoked key token",
			authHeader: "Bearer " + genRevoked.Plaintext,
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "expired key token",
			authHeader: "Bearer " + genExpired.Plaintext,
			wantCode:   http.StatusUnauthorized,
			wantSub:    response.CodeInvalidAPIKey,
		},
		{
			name:       "database error",
			authHeader: "Bearer " + genValid.Plaintext,
			storeErr:   errors.New("db connection failure"),
			wantCode:   http.StatusInternalServerError,
			wantSub:    response.CodeInternalError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kStore.mu.Lock()
			kStore.err = tc.storeErr
			kStore.mu.Unlock()

			handler := middleware.Authenticate(kStore)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d", tc.wantCode, rec.Code)
			}

			var envelope response.ErrorEnvelope
			if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
				t.Fatalf("failed to decode error body: %v", err)
			}
			if envelope.Error.Code != tc.wantSub {
				t.Errorf("expected error code %q, got %q", tc.wantSub, envelope.Error.Code)
			}
		})
	}
}

func TestGetAPIKey_EdgeCases(t *testing.T) {
	if got := middleware.GetAPIKey(nil); got != nil {
		t.Errorf("GetAPIKey(nil) = %+v, want nil", got)
	}

	ctx := context.Background()
	if got := middleware.GetAPIKey(ctx); got != nil {
		t.Errorf("GetAPIKey(empty ctx) = %+v, want nil", got)
	}
}
