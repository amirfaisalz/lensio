package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_PreflightOptions(t *testing.T) {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	cors := CORS(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/usage", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	cors.ServeHTTP(rec, req)

	if handlerCalled {
		t.Errorf("expected next handler not to be called for OPTIONS preflight")
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204 No Content, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got %q", origin)
	}

	if creds := rec.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got %q", creds)
	}

	if methods := rec.Header().Get("Access-Control-Allow-Methods"); methods == "" {
		t.Errorf("expected Access-Control-Allow-Methods to be set")
	}

	if headers := rec.Header().Get("Access-Control-Allow-Headers"); headers == "" {
		t.Errorf("expected Access-Control-Allow-Headers to be set")
	}
}

func TestCORS_StandardRequest(t *testing.T) {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	cors := CORS(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
	rec := httptest.NewRecorder()

	cors.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Errorf("expected next handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected wildcard origin '*' when header absent, got %q", origin)
	}
}
