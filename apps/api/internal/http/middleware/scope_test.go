package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

func TestRequireScope_Success(t *testing.T) {
	tests := []struct {
		name           string
		keyScopes      []string
		requiredScopes []string
	}{
		{
			name:           "exact match single scope",
			keyScopes:      []string{"ocr:write"},
			requiredScopes: []string{"ocr:write"},
		},
		{
			name:           "superset match",
			keyScopes:      []string{"ocr:read", "ocr:write", "usage:read"},
			requiredScopes: []string{"ocr:write"},
		},
		{
			name:           "multiple required all present",
			keyScopes:      []string{"ocr:read", "ocr:write", "usage:read"},
			requiredScopes: []string{"ocr:read", "ocr:write"},
		},
		{
			name:           "wildcard star grants all",
			keyScopes:      []string{"*"},
			requiredScopes: []string{"ocr:write", "usage:read"},
		},
		{
			name:           "admin scope grants all",
			keyScopes:      []string{"admin"},
			requiredScopes: []string{"ocr:write", "keys:manage"},
		},
		{
			name:           "empty required scopes always passes",
			keyScopes:      []string{"ocr:read"},
			requiredScopes: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.RequireScope(tc.requiredScopes...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			key := &store.APIKey{ID: "key-1", Scopes: tc.keyScopes}
			req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
			req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
		})
	}
}

func TestRequireScope_Failures(t *testing.T) {
	t.Run("unauthenticated request", func(t *testing.T) {
		handler := middleware.RequireScope("ocr:write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}

		var envelope response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&envelope)
		if envelope.Error.Code != response.CodeInvalidAPIKey {
			t.Errorf("expected code %q, got %q", response.CodeInvalidAPIKey, envelope.Error.Code)
		}
	})

	t.Run("missing required scope", func(t *testing.T) {
		handler := middleware.RequireScope("ocr:write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		key := &store.APIKey{ID: "key-1", Scopes: []string{"ocr:read"}}
		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}

		var envelope response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&envelope)
		if envelope.Error.Code != response.CodeInsufficientScope {
			t.Errorf("expected code %q, got %q", response.CodeInsufficientScope, envelope.Error.Code)
		}
	})
}

func BenchmarkScopeCheck(b *testing.B) {
	keyScopes := []string{"ocr:read", "ocr:write", "usage:read", "billing:read"}
	required := []string{"ocr:write", "usage:read"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = middleware.HasScope(keyScopes, required...)
	}
}
