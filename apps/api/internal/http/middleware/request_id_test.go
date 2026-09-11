package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
)

func TestRequestID_GeneratesNew(t *testing.T) {
	var capturedID string
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler.ServeHTTP(rec, req)

	if !strings.HasPrefix(capturedID, "req_") {
		t.Errorf("expected request_id to start with 'req_', got %q", capturedID)
	}

	headerID := rec.Header().Get("X-Request-ID")
	if headerID != capturedID {
		t.Errorf("header X-Request-ID %q does not match context %q", headerID, capturedID)
	}
}

func TestRequestID_ReusesValidExisting(t *testing.T) {
	existingID := "req_custom_123456789"
	var capturedID string
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", existingID)

	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected reused request ID %q, got %q", existingID, capturedID)
	}
	if rec.Header().Get("X-Request-ID") != existingID {
		t.Errorf("expected header %q, got %q", existingID, rec.Header().Get("X-Request-ID"))
	}
}

func TestRequestID_RejectsMalformedExisting(t *testing.T) {
	malformedIDs := []string{
		"req_with spaces",
		"req_with_invalid$char",
		strings.Repeat("a", 129), // too long
	}

	for _, badID := range malformedIDs {
		t.Run(badID, func(t *testing.T) {
			var capturedID string
			handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedID = middleware.GetRequestID(r)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Request-ID", badID)

			handler.ServeHTTP(rec, req)

			if capturedID == badID {
				t.Fatalf("expected malformed ID %q to be replaced, but it was accepted", badID)
			}
			if !strings.HasPrefix(capturedID, "req_") {
				t.Errorf("expected new req_ ID, got %q", capturedID)
			}
		})
	}
}

func TestGetRequestID_NilRequest(t *testing.T) {
	if got := middleware.GetRequestID(nil); got != "" {
		t.Errorf("GetRequestID(nil) = %q, want empty", got)
	}
}
