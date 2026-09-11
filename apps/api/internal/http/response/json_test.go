package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
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

	response.Error(rec, http.StatusBadRequest, "invalid_request", "Missing required field", "req_12345")

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

	if envelope.Error.Code != "invalid_request" {
		t.Fatalf("expected code 'invalid_request', got '%s'", envelope.Error.Code)
	}
	if envelope.Error.Message != "Missing required field" {
		t.Fatalf("expected message 'Missing required field', got '%s'", envelope.Error.Message)
	}
	if envelope.Error.RequestID != "req_12345" {
		t.Fatalf("expected request_id 'req_12345', got '%s'", envelope.Error.RequestID)
	}
}
