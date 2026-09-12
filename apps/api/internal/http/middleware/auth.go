package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type ctxKeyAPIKey struct{}

var apiKeyContextKey = ctxKeyAPIKey{}

// WithAPIKey injects the authenticated APIKey into the request context.
func WithAPIKey(ctx context.Context, key *store.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyContextKey, key)
}

// GetAPIKey retrieves the authenticated APIKey from the request context.
func GetAPIKey(ctx context.Context) *store.APIKey {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(apiKeyContextKey).(*store.APIKey); ok {
		return v
	}
	return nil
}

// Authenticate returns a middleware that validates API keys via Authorization: Bearer <token>.
func Authenticate(keyStore store.APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Missing or malformed Authorization header. Expected 'Bearer <api_key>'",
				)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"API key token is empty",
				)
				return
			}

			tokenHash := apikey.Hash(token)
			key, err := keyStore.GetAPIKeyByHash(r.Context(), tokenHash)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					response.ErrorWithRequest(
						w,
						r,
						http.StatusUnauthorized,
						response.CodeInvalidAPIKey,
						"Invalid or unrecognized API key",
					)
					return
				}

				response.ErrorWithRequest(
					w,
					r,
					http.StatusInternalServerError,
					response.CodeInternalError,
					"Failed to verify API key",
				)
				return
			}

			if key.RevokedAt != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"API key has been revoked",
				)
				return
			}

			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"API key has expired",
				)
				return
			}

			// Asynchronously touch last_used_at without blocking the critical path
			// ponytail: unbuffered async DB touch per request; upgrade to batched channel / worker pool in Phase 4 if throughput exceeds 500 QPS
			go func(id string) {
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
				defer cancel()
				_ = keyStore.TouchAPIKeyLastUsed(ctx, id, time.Now())
			}(key.ID)

			ctx := WithAPIKey(r.Context(), key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
