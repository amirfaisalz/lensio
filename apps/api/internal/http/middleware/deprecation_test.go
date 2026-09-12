package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
)

func TestDeprecation(t *testing.T) {
	tests := []struct {
		name          string
		deprecated    bool
		sunset        string
		wantDeprecate string
		wantSunset    string
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

func TestDeprecationWithConfig(t *testing.T) {
	tests := []struct {
		name          string
		cfg           middleware.DeprecationConfig
		wantDeprecate string
		wantSunset    string
		wantLink      string
	}{
		{
			name: "not deprecated with config",
			cfg: middleware.DeprecationConfig{
				Deprecated: false,
				Sunset:     "2027-12-31",
				Link:       `<https://docs.lensio.dev/migration>; rel="sunset"`,
			},
			wantDeprecate: "",
			wantSunset:    "",
			wantLink:      "",
		},
		{
			name: "deprecated with sunset and link",
			cfg: middleware.DeprecationConfig{
				Deprecated: true,
				Sunset:     "2027-09-12",
				Link:       `<https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"`,
			},
			wantDeprecate: "true",
			wantSunset:    "2027-09-12",
			wantLink:      `<https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"`,
		},
		{
			name: "deprecated only with link",
			cfg: middleware.DeprecationConfig{
				Deprecated: true,
				Link:       `<https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"`,
			},
			wantDeprecate: "true",
			wantSunset:    "",
			wantLink:      `<https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"`,
		},
		{
			name: "deprecated minimal",
			cfg: middleware.DeprecationConfig{
				Deprecated: true,
			},
			wantDeprecate: "true",
			wantSunset:    "",
			wantLink:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.DeprecationWithConfig(tc.cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/ktp", nil)

			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Deprecation"); got != tc.wantDeprecate {
				t.Errorf("Deprecation header = %q, want %q", got, tc.wantDeprecate)
			}
			if got := rec.Header().Get("Sunset"); got != tc.wantSunset {
				t.Errorf("Sunset header = %q, want %q", got, tc.wantSunset)
			}
			if got := rec.Header().Get("Link"); got != tc.wantLink {
				t.Errorf("Link header = %q, want %q", got, tc.wantLink)
			}
		})
	}
}
