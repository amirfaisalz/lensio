package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
)

func TestDeprecation(t *testing.T) {
	tests := []struct {
		name           string
		deprecated     bool
		sunset         string
		wantDeprecate  string
		wantSunset     string
	}{
		{
			name:          "not deprecated",
			deprecated:    false,
			sunset:        "",
			wantDeprecate: "",
			wantSunset:    "",
		},
		{
			name:          "deprecated without sunset date",
			deprecated:    true,
			sunset:        "",
			wantDeprecate: "true",
			wantSunset:    "",
		},
		{
			name:          "deprecated with sunset date",
			deprecated:    true,
			sunset:        "2027-01-01",
			wantDeprecate: "true",
			wantSunset:    "2027-01-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.Deprecation(tc.deprecated, tc.sunset)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/old", nil)

			handler.ServeHTTP(rec, req)

			if rec.Header().Get("Deprecation") != tc.wantDeprecate {
				t.Errorf("Deprecation header = %q, want %q", rec.Header().Get("Deprecation"), tc.wantDeprecate)
			}
			if rec.Header().Get("Sunset") != tc.wantSunset {
				t.Errorf("Sunset header = %q, want %q", rec.Header().Get("Sunset"), tc.wantSunset)
			}
		})
	}
}
