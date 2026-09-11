package response

import (
	"encoding/json"
	"net/http"
)

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

// Error writes a standardized JSON error response.
func Error(w http.ResponseWriter, status int, code string, message string, requestID string) {
	JSON(w, status, ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}
