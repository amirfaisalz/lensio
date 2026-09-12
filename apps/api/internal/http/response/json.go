package response

import (
	"context"
	"encoding/json"
	"net/http"
)

// Standard error codes defined by Lensio PRD Section 20.
const (
	CodeInvalidRequest      = "invalid_request"
	CodeInvalidAPIKey       = "invalid_api_key"
	CodeInsufficientScope   = "insufficient_scope"
	CodeRateLimitExceeded   = "rate_limit_exceeded"
	CodeQuotaExceeded       = "quota_exceeded"
	CodeInvalidDocument     = "invalid_document"
	CodeUnsupportedDocument = "unsupported_document"
	CodeOCRFailed           = "ocr_failed"
	CodeLowConfidence       = "low_confidence"
	CodeInternalError       = "internal_error"
	CodeRequestInProgress   = "request_in_progress"
	CodeIdempotencyMismatch = "idempotency_key_mismatch"
	CodePermissionDenied    = "permission_denied"
	CodeEmailNotVerified    = "EMAIL_NOT_VERIFIED"
)

type ctxKeyRequestID struct{}

// RequestIDContextKey is the context key used to store and retrieve the request ID.
var RequestIDContextKey = ctxKeyRequestID{}

// ErrorDetail represents the standardized error body.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorEnvelope wraps ErrorDetail into the root "error" property.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// JSON writes a JSON payload to the ResponseWriter with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error writes a standardized JSON error response with an explicit requestID.
func Error(w http.ResponseWriter, status int, code string, message string, requestID string) {
	JSON(w, status, ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

// GetRequestID retrieves the request ID from the context if present.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return v
	}
	return ""
}

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, reqID)
}

// ErrorWithRequest writes an error extracting the request_id from r.Context() or header.
func ErrorWithRequest(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	var reqID string
	if r != nil {
		reqID = GetRequestID(r.Context())
		if reqID == "" {
			reqID = r.Header.Get("X-Request-ID")
		}
	}
	Error(w, status, code, message, reqID)
}
