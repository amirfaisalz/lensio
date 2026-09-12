package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type mockAccountStoreForHandlers struct {
	org        *store.Organization
	plan       *store.Plan
	members    []store.User
	orgErr     error
	planErr    error
	updateErr  error
	membersErr error
}

func (m *mockAccountStoreForHandlers) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	if m.orgErr != nil {
		return nil, m.orgErr
	}
	return m.org, nil
}

func (m *mockAccountStoreForHandlers) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	if m.planErr != nil {
		return nil, m.planErr
	}
	return m.plan, nil
}

func (m *mockAccountStoreForHandlers) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *mockAccountStoreForHandlers) GetOrganizationMembers(ctx context.Context, orgID string) ([]store.User, error) {
	if m.membersErr != nil {
		return nil, m.membersErr
	}
	return m.members, nil
}

func (m *mockAccountStoreForHandlers) CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*store.User, error) {
	return nil, nil
}

func (m *mockAccountStoreForHandlers) GetUserByEmail(ctx context.Context, email string) (*store.UserWithAuth, error) {
	return nil, store.ErrNotFound
}

func (m *mockAccountStoreForHandlers) GetUserByID(ctx context.Context, userID string) (*store.User, error) {
	return nil, store.ErrNotFound
}

func (m *mockAccountStoreForHandlers) VerifyUserEmail(ctx context.Context, email, token string) error {
	return nil
}

func (m *mockAccountStoreForHandlers) CreateOrganization(ctx context.Context, name, slug, planCode string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAccountStoreForHandlers) AssignUserToOrg(ctx context.Context, userID, orgID, role string) error {
	return nil
}

func (m *mockAccountStoreForHandlers) GetUserOrganization(ctx context.Context, userID string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}

type mockAuditStoreForHandlers struct {
	recorded []*store.AuditLog
}

func (m *mockAuditStoreForHandlers) RecordAuditLog(ctx context.Context, log *store.AuditLog) error {
	m.recorded = append(m.recorded, log)
	return nil
}

func (m *mockAuditStoreForHandlers) ListAuditLogsByOrg(ctx context.Context, orgID string) ([]*store.AuditLog, error) {
	return m.recorded, nil
}

func TestAccountDetailsHandler(t *testing.T) {
	t.Run("nil store returns 500", func(t *testing.T) {
		handler := handlers.AccountDetailsHandler(nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("org not found returns 404", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{orgErr: store.ErrNotFound}
		handler := handlers.AccountDetailsHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("successful account details", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			org: &store.Organization{
				ID:              "org-1",
				Name:            "Acme Corp",
				Slug:            "acme",
				PlanCode:        "starter",
				PlanName:        "Starter Tier",
				ActiveKeysCount: 2,
				CreatedAt:       time.Now(),
			},
		}
		handler := handlers.AccountDetailsHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{orgErr: errors.New("db down")}
		handler := handlers.AccountDetailsHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestAccountPlanHandler(t *testing.T) {
	t.Run("nil store returns 500", func(t *testing.T) {
		handler := handlers.AccountPlanHandler(nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/plan", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("plan not found returns 404", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{planErr: store.ErrNotFound}
		handler := handlers.AccountPlanHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/plan", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("successful plan retrieval", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			plan: &store.Plan{
				Code:               "pro",
				Name:               "Pro Tier",
				MonthlyQuota:       10000,
				RateLimitPerMinute: 100,
			},
		}
		handler := handlers.AccountPlanHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/plan", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{planErr: errors.New("db error")}
		handler := handlers.AccountPlanHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/plan", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestUpdatePlanHandler(t *testing.T) {
	t.Run("invalid json payload returns 400", func(t *testing.T) {
		handler := handlers.UpdatePlanHandler(nil, nil, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`invalid-json`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing plan_code returns 400", func(t *testing.T) {
		handler := handlers.UpdatePlanHandler(nil, nil, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":""}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("nil store returns 500", func(t *testing.T) {
		handler := handlers.UpdatePlanHandler(nil, nil, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":"pro"}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("successful plan update with audit logging", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			plan: &store.Plan{Code: "free"},
		}
		auditStore := &mockAuditStoreForHandlers{}

		handler := handlers.UpdatePlanHandler(aStore, auditStore, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":"pro"}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if len(auditStore.recorded) != 1 {
			t.Fatalf("expected 1 audit log recorded, got %d", len(auditStore.recorded))
		}
		log := auditStore.recorded[0]
		if log.Action != "plan.change" {
			t.Errorf("expected action plan.change, got %s", log.Action)
		}
	})

	t.Run("unknown plan code returns 400", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			updateErr: store.ErrNotFound,
		}
		handler := handlers.UpdatePlanHandler(aStore, nil, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":"invalid"}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("store update error returns 500", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			updateErr: errors.New("db error"),
		}
		handler := handlers.UpdatePlanHandler(aStore, nil, "org-1")
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":"starter"}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestAccountMembersHandler(t *testing.T) {
	t.Run("nil store returns empty members", func(t *testing.T) {
		handler := handlers.AccountMembersHandler(nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/members", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("successful members retrieval", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			members: []store.User{
				{
					ID:        "user-1",
					OrgID:     "org-1",
					Email:     "dev@lensio.dev",
					FullName:  "Lensio Lead Developer",
					Role:      "owner",
					CreatedAt: time.Now(),
				},
			},
		}
		handler := handlers.AccountMembersHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/members", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		aStore := &mockAccountStoreForHandlers{
			membersErr: errors.New("db error"),
		}
		handler := handlers.AccountMembersHandler(aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/members", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

