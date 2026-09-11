package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	internalhttp "github.com/amirfaisalz/nusaid/apps/api/internal/http"
)

type dummyPinger struct{}

func (d *dummyPinger) PingContext(ctx context.Context) error {
	return nil
}

func TestNewRouter(t *testing.T) {
	router := internalhttp.NewRouter(&dummyPinger{})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ready endpoint",
			method:         http.MethodGet,
			path:           "/ready",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unknown endpoint",
			method:         http.MethodGet,
			path:           "/unknown",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "health method not allowed",
			method:         http.MethodPost,
			path:           "/health",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d for %s %s, got %d", tc.expectedStatus, tc.method, tc.path, rec.Code)
			}
		})
	}
}
