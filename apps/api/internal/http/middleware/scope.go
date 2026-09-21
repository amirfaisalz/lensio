package middleware

import (
	"fmt"
	"net/http"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

const (
	ScopeAdmin = "admin"
	ScopeAll   = "*"
)

// HasScope verifies whether the given key scopes satisfy all required scopes.
//
// ScopeAll ("*") is the only wildcard. ScopeAdmin ("admin") is deliberately NOT
// a wildcard: it is granted to every organization owner at login, so treating it
// as one turned "owner of any org" into "authorized for every scope on the
// platform". It is matched literally, like any other scope name.
func HasScope(keyScopes []string, required ...string) (bool, string) {
	if len(required) == 0 {
		return true, ""
	}

	scopeSet := make(map[string]struct{}, len(keyScopes))
	for _, s := range keyScopes {
		if s == ScopeAll {
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
			user := GetOIDCUser(r.Context())
			if key == nil && user == nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Unauthenticated request",
				)
				return
			}

			var candidateScopes []string
			if key != nil {
				candidateScopes = key.Scopes
			} else if user != nil {
				candidateScopes = user.Roles
			}

			ok, missing := HasScope(candidateScopes, requiredScopes...)
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
