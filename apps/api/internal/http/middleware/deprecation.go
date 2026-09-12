package middleware

import "net/http"

// DeprecationConfig configures deprecation, sunset, and link headers according to PRD Section 6 and ADR-007.
type DeprecationConfig struct {
	Deprecated bool
	Sunset     string // RFC 8594 Sunset date or ISO 8601 string
	Link       string // RFC 8288 Link header pointing to migration documentation or successor endpoint
}

// DeprecationWithConfig injects standard Deprecation, Sunset, and Link headers based on DeprecationConfig.
func DeprecationWithConfig(cfg DeprecationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Deprecated {
				w.Header().Set("Deprecation", "true")
				if cfg.Sunset != "" {
					w.Header().Set("Sunset", cfg.Sunset)
				}
				if cfg.Link != "" {
					w.Header().Set("Link", cfg.Link)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Deprecation injects standard Deprecation and Sunset headers according to PRD Section 6.
func Deprecation(deprecated bool, sunsetDate string) func(http.Handler) http.Handler {
	return DeprecationWithConfig(DeprecationConfig{
		Deprecated: deprecated,
		Sunset:     sunsetDate,
	})
}
