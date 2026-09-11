package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

type mockAccountStoreForRateLimit struct {
	plan *store.Plan
}

func (m *mockAccountStoreForRateLimit) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAccountStoreForRateLimit) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	return m.plan, nil
}

func (m *mockAccountStoreForRateLimit) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	return nil
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	accountStore := &mockAccountStoreForRateLimit{
		plan: &store.Plan{RateLimitPerMinute: 2},
	}

	mw := middleware.NewRateLimitMiddleware(limiter, accountStore, "org-test-1")
	nextCalled := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Handler(next)

	// 1st request should pass
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}
	if rec1.Header().Get("X-RateLimit-Limit") != "2" {
		t.Errorf("expected limit header 2, got %s", rec1.Header().Get("X-RateLimit-Limit"))
	}
	if rec1.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Errorf("expected remaining header 1, got %s", rec1.Header().Get("X-RateLimit-Remaining"))
	}

	// 2nd request should pass
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if rec2.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected remaining header 0, got %s", rec2.Header().Get("X-RateLimit-Remaining"))
	}

	// 3rd request should fail with 429
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") == "" {
		t.Error("expected non-empty Retry-After header on 429")
	}
	if nextCalled != 2 {
		t.Errorf("expected next handler called exactly 2 times, got %d", nextCalled)
	}
}

func TestRateLimitMiddleware_WithAPIKeyContext(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	mw := middleware.NewRateLimitMiddleware(limiter, nil, "")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Handler(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	key := &store.APIKey{OrgID: "custom-org-id"}
	req = req.WithContext(middleware.WithAPIKey(req.Context(), key))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
