package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/nusaid/apps/api/internal/http"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

type dummyPinger struct{}

func (d *dummyPinger) PingContext(ctx context.Context) error {
	return nil
}

type dummyKeyStore struct {
	keys map[string]*store.APIKey
}

func newDummyKeyStore() *dummyKeyStore {
	return &dummyKeyStore{
		keys: make(map[string]*store.APIKey),
	}
}

func (d *dummyKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	key.ID = "dummy-id-1"
	d.keys[key.KeyHash] = key
	return nil
}

func (d *dummyKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	k, ok := d.keys[keyHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return k, nil
}

func (d *dummyKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	return nil, nil
}

func (d *dummyKeyStore) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	return nil
}

func (d *dummyKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	return nil
}

func TestNewRouter(t *testing.T) {
	kStore := newDummyKeyStore()
	gen, _ := apikey.Generate(apikey.EnvLive)
	_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:          "dummy-id-1",
		OrgID:       "org-1",
		KeyHash:     gen.KeyHash,
		Scopes:      []string{"ocr:write"},
		Environment: "live",
	})

	router := internalhttp.NewRouter(&dummyPinger{}, kStore)

	tests := []struct {
		name           string
		method         string
		path           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ready endpoint",
			method:         http.MethodGet,
			path:           "/ready",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "openapi endpoint",
			method:         http.MethodGet,
			path:           "/openapi",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "openapi.yaml endpoint",
			method:         http.MethodGet,
			path:           "/openapi.yaml",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "docs endpoint",
			method:         http.MethodGet,
			path:           "/docs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "auth verify without key returns 401",
			method:         http.MethodGet,
			path:           "/api/v1/auth/verify",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "auth verify with valid key returns 200",
			method:         http.MethodGet,
			path:           "/api/v1/auth/verify",
			authHeader:     "Bearer " + gen.Plaintext,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unknown endpoint returns 404",
			method:         http.MethodGet,
			path:           "/unknown",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d for %s %s, got %d", tc.expectedStatus, tc.method, tc.path, rec.Code)
			}

			// Verify X-Request-ID header is always emitted on responses
			if reqID := rec.Header().Get("X-Request-ID"); reqID == "" {
				t.Error("expected X-Request-ID header to be present on response")
			}
		})
	}
}

func TestNewRouter_NilKeyStore(t *testing.T) {
	router := internalhttp.NewRouter(&dummyPinger{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
