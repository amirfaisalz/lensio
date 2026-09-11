package middleware

import (
	"fmt"
	"net/http"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
)

const (
	ScopeAdmin = "admin"
	ScopeAll   = "*"
)

// HasScope verifies whether the given key scopes satisfy all required scopes.
// Supports ScopeAdmin ("admin") and ScopeAll ("*") wildcards.
func HasScope(keyScopes []string, required ...string) (bool, string) {
	if len(required) == 0 {
		return true, ""
	}

	scopeSet := make(map[string]struct{}, len(keyScopes))
	for _, s := range keyScopes {
		if s == ScopeAdmin || s == ScopeAll {
			return true, ""
		}
		scopeSet[s] = struct{}{}
	}

	for _, req := range required {
		if _, ok := scopeSet[req]; !ok {
			return false, req
		}
	}

	return true, ""
}

// RequireScope verifies that the authenticated API key in the request context possesses
// all required scopes before delegating to the next HTTP handler.
func RequireScope(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetAPIKey(r.Context())
			if key == nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Unauthenticated request",
				)
				return
			}

			ok, missing := HasScope(key.Scopes, requiredScopes...)
			if !ok {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusForbidden,
					response.CodeInsufficientScope,
					fmt.Sprintf("API key lacks required scope: %s", missing),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
