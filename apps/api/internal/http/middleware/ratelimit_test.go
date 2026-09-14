package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
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

func (m *mockAccountStoreForRateLimit) GetOrganizationMembers(ctx context.Context, orgID string) ([]store.User, error) {
	return nil, nil
}

func (m *mockAccountStoreForRateLimit) CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*store.User, error) {
	return nil, nil
}

func (m *mockAccountStoreForRateLimit) GetUserByEmail(ctx context.Context, email string) (*store.UserWithAuth, error) {
	return nil, store.ErrNotFound
}

func (m *mockAccountStoreForRateLimit) GetUserByID(ctx context.Context, userID string) (*store.User, error) {
	return nil, store.ErrNotFound
}

func (m *mockAccountStoreForRateLimit) VerifyUserEmail(ctx context.Context, email, token string) error {
	return nil
}

func (m *mockAccountStoreForRateLimit) CreateOrganization(ctx context.Context, name, slug, planCode string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAccountStoreForRateLimit) AssignUserToOrg(ctx context.Context, userID, orgID, role string) error {
	return nil
}

func (m *mockAccountStoreForRateLimit) GetUserOrganization(ctx context.Context, userID string) (*store.Organization, error) {
	return nil, store.ErrNotFound
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

func TestRateLimitMiddleware_OIDCUserRateLimited(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	mw := middleware.NewRateLimitMiddleware(limiter, nil, "org-test-1")

	nextCalled := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Handler(next)

	// Exhaust limiter for this OIDC identity (default free-tier bucket is small in tests via direct Allow).
	for i := 0; i < 15; i++ {
		limiter.Allow("oidc:sub-operator-1", 1)
	}

	// Request with OIDCUser must be rate-limited under its own identity key, never bypass.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	user := &middleware.OIDCUser{Subject: "sub-operator-1", Email: "operator@lensio.dev"}
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exhausted OIDC identity, got %d", rec.Code)
	}
	if nextCalled != 0 {
		t.Errorf("expected next not called on 429, got %d", nextCalled)
	}
}

func TestRateLimitMiddleware_AuthMeAndLogoutBypass(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	mw := middleware.NewRateLimitMiddleware(limiter, nil, "org-test-1")

	nextCalled := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Handler(next)

	for _, path := range []string{"/api/v1/auth/me", "/api/v1/auth/logout"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", path, rec.Code)
		}
	}
	if nextCalled != 2 {
		t.Errorf("expected next called 2 times, got %d", nextCalled)
	}
}

func TestRateLimitMiddleware_AccountEndpointsBypass(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	exhaustOrg := "ad39edb4-a2d7-476a-b6e3-a11aca1930ee"
	// Exhaust limiter for this org key
	for i := 0; i < 20; i++ {
		limiter.Allow(exhaustOrg, 1)
	}

	mw := middleware.NewRateLimitMiddleware(limiter, nil, exhaustOrg)

	nextCalled := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Handler(next)

	for _, path := range []string{
		"/api/v1/account/members?org_id=" + exhaustOrg,
		"/api/v1/account?org_id=" + exhaustOrg,
		"/api/v1/account/plan?org_id=" + exhaustOrg,
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", path, rec.Code)
		}
	}
	if nextCalled != 3 {
		t.Errorf("expected next called 3 times, got %d", nextCalled)
	}
}

func TestRateLimitMiddleware_LoginEnforcedAndOIDCIgnoresOrgQuery(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	mw := middleware.NewRateLimitMiddleware(limiter, nil, "")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.Handler(next)

	for i := 0; i < 15; i++ {
		limiter.Allow("ip:192.0.2.1", 1)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exhausted login bucket, got %d", loginRec.Code)
	}

	for i := 0; i < 15; i++ {
		limiter.Allow("oidc:sub-query-1", 1)
	}
	oidcReq := httptest.NewRequest(http.MethodGet, "/test?org_id=some-other-org", nil)
	oidcReq = oidcReq.WithContext(middleware.WithOIDCUser(oidcReq.Context(), &middleware.OIDCUser{Subject: "sub-query-1"}))
	oidcRec := httptest.NewRecorder()
	handler.ServeHTTP(oidcRec, oidcReq)
	if oidcRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 under identity key despite org_id query, got %d", oidcRec.Code)
	}
}
