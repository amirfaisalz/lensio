package response_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	response.JSON(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status 'ok', got '%s'", body["status"])
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()

	response.Error(rec, http.StatusBadRequest, response.CodeInvalidRequest, "Missing required field", "req_12345")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}

	if envelope.Error.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %q, got %q", response.CodeInvalidRequest, envelope.Error.Code)
	}
	if envelope.Error.Message != "Missing required field" {
		t.Fatalf("expected message 'Missing required field', got '%s'", envelope.Error.Message)
	}
	if envelope.Error.RequestID != "req_12345" {
		t.Fatalf("expected request_id 'req_12345', got '%s'", envelope.Error.RequestID)
	}
}

func TestErrorWithRequest(t *testing.T) {
	t.Run("from context", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := response.WithRequestID(req.Context(), "req_ctx_999")
		req = req.WithContext(ctx)

		response.ErrorWithRequest(rec, req, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Invalid API key")

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		var envelope response.ErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatalf("failed to decode error envelope: %v", err)
		}
		if envelope.Error.RequestID != "req_ctx_999" {
			t.Errorf("expected request_id 'req_ctx_999', got %q", envelope.Error.RequestID)
		}
		if envelope.Error.Code != response.CodeInvalidAPIKey {
			t.Errorf("expected code %q, got %q", response.CodeInvalidAPIKey, envelope.Error.Code)
		}
	})

	t.Run("from header fallback", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "req_hdr_888")

		response.ErrorWithRequest(rec, req, http.StatusForbidden, response.CodeInsufficientScope, "Insufficient scope")

		var envelope response.ErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatalf("failed to decode error envelope: %v", err)
		}
		if envelope.Error.RequestID != "req_hdr_888" {
			t.Errorf("expected request_id 'req_hdr_888', got %q", envelope.Error.RequestID)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		response.ErrorWithRequest(rec, nil, http.StatusInternalServerError, response.CodeInternalError, "Internal error")

		var envelope response.ErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
			t.Fatalf("failed to decode error envelope: %v", err)
		}
		if envelope.Error.RequestID != "" {
			t.Errorf("expected empty request_id, got %q", envelope.Error.RequestID)
		}
	})
}

func TestGetRequestID_EdgeCases(t *testing.T) {
	if got := response.GetRequestID(nil); got != "" {
		t.Errorf("GetRequestID(nil) = %q, want empty", got)
	}

	ctx := context.Background()
	if got := response.GetRequestID(ctx); got != "" {
		t.Errorf("GetRequestID(ctx without key) = %q, want empty", got)
	}

	// Non-string value in context
	badCtx := context.WithValue(ctx, response.RequestIDContextKey, 12345)
	if got := response.GetRequestID(badCtx); got != "" {
		t.Errorf("GetRequestID(badCtx) = %q, want empty", got)
	}
}
