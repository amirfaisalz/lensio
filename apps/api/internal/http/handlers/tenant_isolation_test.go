package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// These tests cover the authorization boundary rather than handler behaviour:
// every org-scoped endpoint accepts an org_id from the caller, and each one must
// refuse an organization the caller does not belong to. The absence of exactly
// this suite is why a full green run previously sat on top of a cross-tenant
// read/write hole.

const (
	attackerOrg = "aaaaaaaa-0000-0000-0000-000000000001"
	victimOrg   = "bbbbbbbb-0000-0000-0000-000000000002"
)

// tenantSpy records which organization actually reached the store. A handler that
// leaks must be caught by the recorded org, not only by the HTTP status.
type tenantSpy struct{ queriedOrg string }

func (s *tenantSpy) note(orgID string) { s.queriedOrg = orgID }

func (s *tenantSpy) CreateUsageRecord(context.Context, *store.UsageRecord) error { return nil }
func (s *tenantSpy) GetUsageSummary(_ context.Context, orgID string, _ time.Time, _ int, _ time.Time) (*store.UsageSummary, error) {
	s.note(orgID)
	return &store.UsageSummary{TotalRequests: 4242}, nil
}
func (s *tenantSpy) GetDailyUsage(_ context.Context, orgID string, _ time.Time) ([]store.DailyUsage, error) {
	s.note(orgID)
	return nil, nil
}
func (s *tenantSpy) GetEndpointUsage(_ context.Context, orgID string, _ time.Time) ([]store.EndpointUsage, error) {
	s.note(orgID)
	return nil, nil
}
func (s *tenantSpy) GetMonthlyOCRCount(context.Context, string, time.Time) (int, error) {
	return 0, nil
}
func (s *tenantSpy) GetUsageRecords(_ context.Context, orgID string, _ store.UsageRecordFilter) ([]store.UsageRecord, int, error) {
	s.note(orgID)
	return []store.UsageRecord{{OrgID: orgID, RequestID: "victim-only-request-id"}}, 1, nil
}

func (s *tenantSpy) GetOrganization(_ context.Context, orgID string) (*store.Organization, error) {
	s.note(orgID)
	return &store.Organization{ID: orgID, Name: "Victim Corp"}, nil
}
func (s *tenantSpy) GetOrganizationPlan(_ context.Context, orgID string) (*store.Plan, error) {
	s.note(orgID)
	return &store.Plan{Code: "scale", MonthlyQuota: 100000, RateLimitPerMinute: 600}, nil
}
func (s *tenantSpy) UpdateOrganizationPlan(_ context.Context, orgID, _ string) error {
	s.note(orgID)
	return nil
}
func (s *tenantSpy) GetOrganizationMembers(_ context.Context, orgID string) ([]store.User, error) {
	s.note(orgID)
	return []store.User{{Email: "victim.employee@example.com", FullName: "Victim Employee"}}, nil
}
func (s *tenantSpy) CreateUser(context.Context, string, string, string, string) (*store.User, error) {
	return nil, nil
}
func (s *tenantSpy) GetUserByEmail(context.Context, string) (*store.UserWithAuth, error) {
	return nil, store.ErrNotFound
}
func (s *tenantSpy) GetUserByID(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (s *tenantSpy) VerifyUserEmail(context.Context, string, string) error { return nil }
func (s *tenantSpy) CreateOrganization(context.Context, string, string, string) (*store.Organization, error) {
	return nil, nil
}
func (s *tenantSpy) AssignUserToOrg(context.Context, string, string, string) error { return nil }
func (s *tenantSpy) GetUserOrganization(context.Context, string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}

// asAttacker authenticates the request with an ordinary API key scoped to
// attackerOrg, holding every scope the platform issues by default.
func asAttacker(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithAPIKey(r.Context(), &store.APIKey{
		ID:     "key-attacker",
		OrgID:  attackerOrg,
		Scopes: []string{"ocr:read", "ocr:write", "usage:read", "developer", "admin"},
	}))
}

func TestTenantIsolation_OrgScopedEndpointsRefuseForeignOrgID(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		handler func(spy *tenantSpy) http.Handler
	}{
		{
			name: "usage summary", method: http.MethodGet, target: "/api/v1/usage",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.UsageSummaryHandler(s, s, handlers.DefaultOrgID)
			},
		},
		{
			name: "usage daily", method: http.MethodGet, target: "/api/v1/usage/daily",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.DailyUsageHandler(s, s, handlers.DefaultOrgID)
			},
		},
		{
			name: "usage endpoints", method: http.MethodGet, target: "/api/v1/usage/endpoints",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.EndpointUsageHandler(s, s, handlers.DefaultOrgID)
			},
		},
		{
			name: "usage records", method: http.MethodGet, target: "/api/v1/usage/records",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.UsageRecordsHandler(s, s, handlers.DefaultOrgID)
			},
		},
		{
			name: "account details", method: http.MethodGet, target: "/api/v1/account",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.AccountDetailsHandler(s, handlers.DefaultOrgID)
			},
		},
		{
			name: "account plan read", method: http.MethodGet, target: "/api/v1/account/plan",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.AccountPlanHandler(s, handlers.DefaultOrgID)
			},
		},
		{
			name: "account plan write", method: http.MethodPut, target: "/api/v1/account/plan",
			body: `{"plan_code":"free"}`,
			handler: func(s *tenantSpy) http.Handler {
				return handlers.UpdatePlanHandler(s, nil, handlers.DefaultOrgID)
			},
		},
		{
			name: "account members", method: http.MethodGet, target: "/api/v1/account/members",
			handler: func(s *tenantSpy) http.Handler {
				return handlers.AccountMembersHandler(s, handlers.DefaultOrgID)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &tenantSpy{}
			req := httptest.NewRequest(tc.method, tc.target+"?org_id="+victimOrg, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			tc.handler(spy).ServeHTTP(rec, asAttacker(req))

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for foreign org_id, got %d: %s", rec.Code, rec.Body.String())
			}
			if spy.queriedOrg == victimOrg {
				t.Errorf("handler reached the store with the victim org %q", victimOrg)
			}
			if strings.Contains(rec.Body.String(), victimOrg) {
				t.Errorf("victim org id echoed in response body: %s", rec.Body.String())
			}
		})
	}
}

func TestTenantIsolation_OwnOrgIDIsStillAccepted(t *testing.T) {
	spy := &tenantSpy{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/records?org_id="+attackerOrg, nil)
	rec := httptest.NewRecorder()
	handlers.UsageRecordsHandler(spy, spy, handlers.DefaultOrgID).ServeHTTP(rec, asAttacker(req))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for the caller's own org, got %d: %s", rec.Code, rec.Body.String())
	}
	if spy.queriedOrg != attackerOrg {
		t.Errorf("expected store queried with %q, got %q", attackerOrg, spy.queriedOrg)
	}
}

func TestTenantIsolation_OmittedOrgIDFallsBackToCallerOrg(t *testing.T) {
	spy := &tenantSpy{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/records", nil)
	rec := httptest.NewRecorder()
	handlers.UsageRecordsHandler(spy, spy, handlers.DefaultOrgID).ServeHTTP(rec, asAttacker(req))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if spy.queriedOrg != attackerOrg {
		t.Errorf("expected the caller's own org %q, got %q", attackerOrg, spy.queriedOrg)
	}
}

// TestEmailVerification_RequiresToken guards the unauthenticated account
// activation bypass: an empty token must never verify an address.
func TestEmailVerification_RequiresToken(t *testing.T) {
	spy := &tenantSpy{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email",
		strings.NewReader(`{"email":"victim@example.com","token":""}`))
	rec := httptest.NewRecorder()
	handlers.VerifyEmailHandler(spy).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an empty verification token, got %d: %s", rec.Code, rec.Body.String())
	}
}
