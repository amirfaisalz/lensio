package middleware

import (
	"net/http"
	"strings"
)

// NewCORSMiddleware returns a CORS middleware enforcing explicit allowed origins.
// When an allowed origin matches, Access-Control-Allow-Credentials is set to true.
// Wildcard "*" allows any origin without credentials (per OWASP / W3C CORS specifications).
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	cleanOrigins := make(map[string]bool)
	allowAll := false
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAll = true
		} else if o != "" {
			cleanOrigins[o] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			var matchedOrigin string
			var allowCredentials bool

			if origin == "" {
				if allowAll {
					matchedOrigin = "*"
				}
			} else if allowAll {
				matchedOrigin = "*"
				allowCredentials = false
			} else if cleanOrigins[origin] {
				matchedOrigin = origin
				allowCredentials = true
			}

			if matchedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", matchedOrigin)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Accept, Cookie, Idempotency-Key")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After, Idempotent-Replayed")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				if origin != "" && matchedOrigin == "" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORS wraps an http.Handler with default CORS configuration (allowing local dev origins).
func CORS(next http.Handler) http.Handler {
	return NewCORSMiddleware([]string{"http://localhost:5173", "http://localhost:3000", "http://localhost:8080"})(next)
}

