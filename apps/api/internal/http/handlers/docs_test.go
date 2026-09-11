package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
)

func TestOpenAPIHandler(t *testing.T) {
	handler := handlers.OpenAPIHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Errorf("expected application/yaml content type, got %s", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3.0.3") {
		t.Error("expected OpenAPI specification content in response body")
	}
}

func TestDocsHandler(t *testing.T) {
	handler := handlers.DocsHandler("/openapi.yaml")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Error("expected Scalar API reference in html")
	}
	if !strings.Contains(body, `data-url="/openapi.yaml"`) {
		t.Error("expected data-url to point to /openapi.yaml")
	}
}
