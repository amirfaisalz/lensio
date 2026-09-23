package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

// RequireBearer admits only requests carrying "Authorization: Bearer <token>".
// It guards operator endpoints such as /metrics that sit on the public ingress.
func RequireBearer(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "A valid bearer token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
