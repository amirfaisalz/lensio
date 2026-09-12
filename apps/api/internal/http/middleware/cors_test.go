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
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	cors.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Errorf("expected next handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rec.Code)
	}

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
		t.Errorf("expected allowed origin 'http://localhost:5173', got %q", origin)
	}
	if creds := rec.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected credentials 'true', got %q", creds)
	}
}

func TestCORS_UnauthorizedOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := NewCORSMiddleware([]string{"https://app.lensio.dev"})(next)

	// Unauthorized preflight
	preReq := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/me", nil)
	preReq.Header.Set("Origin", "https://evil-site.com")
	preRec := httptest.NewRecorder()
	cors.ServeHTTP(preRec, preReq)

	if preRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unauthorized origin preflight, got %d", preRec.Code)
	}
	if origin := preRec.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected empty allow-origin for evil site, got %q", origin)
	}

	// Unauthorized GET
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	getReq.Header.Set("Origin", "https://evil-site.com")
	getRec := httptest.NewRecorder()
	cors.ServeHTTP(getRec, getReq)

	if origin := getRec.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected empty allow-origin for evil site GET, got %q", origin)
	}
}

func TestCORS_WildcardMode(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := NewCORSMiddleware([]string{"*"})(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://anywhere.com")
	rec := httptest.NewRecorder()
	cors.ServeHTTP(rec, req)

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected wildcard origin '*', got %q", origin)
	}
	// Per W3C spec, wildcard MUST NOT have credentials true
	if creds := rec.Header().Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected no credentials with wildcard, got %q", creds)
	}
}
