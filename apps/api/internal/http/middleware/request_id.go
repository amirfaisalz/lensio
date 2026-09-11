package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

const (
	HeaderXRequestID = "X-Request-ID"
	maxRequestIDLen  = 128
)

// RequestID ensures every HTTP request has a unique request_id.
// If an incoming X-Request-ID header is present and valid, it is reused.
// Otherwise, a cryptographically random req_<hex> identifier is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get(HeaderXRequestID))
		if reqID == "" || len(reqID) > maxRequestIDLen || !isValidRequestID(reqID) {
			reqID = generateRequestID()
		}

		ctx := response.WithRequestID(r.Context(), reqID)
		w.Header().Set(HeaderXRequestID, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from the context.
func GetRequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	return response.GetRequestID(r.Context())
}

func generateRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback deterministic pseudo-random in improbable failure
		return "req_000000000000000000000000"
	}
	return "req_" + hex.EncodeToString(b)
}

func isValidRequestID(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Allow alphanumeric, dash, and underscore
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
