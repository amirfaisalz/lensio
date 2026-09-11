package middleware

import "net/http"

// Deprecation injects standard Deprecation and Sunset headers according to PRD Section 6.
func Deprecation(deprecated bool, sunsetDate string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if deprecated {
				w.Header().Set("Deprecation", "true")
				if sunsetDate != "" {
					w.Header().Set("Sunset", sunsetDate)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
