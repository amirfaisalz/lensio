package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type stubSessionAccountStore struct {
	orgByUserID map[string]*store.Organization
	userByEmail map[string]*store.UserWithAuth
}

func (s *stubSessionAccountStore) GetOrganization(_ context.Context, _ string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) GetOrganizationPlan(_ context.Context, _ string) (*store.Plan, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) UpdateOrganizationPlan(_ context.Context, _, _ string) error {
	return store.ErrNotFound
}

func (s *stubSessionAccountStore) GetOrganizationMembers(_ context.Context, _ string) ([]store.User, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) CreateUser(_ context.Context, _, _, _, _ string) (*store.User, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) GetUserByEmail(_ context.Context, email string) (*store.UserWithAuth, error) {
	if u, ok := s.userByEmail[email]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) GetUserByID(_ context.Context, _ string) (*store.User, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) VerifyUserEmail(_ context.Context, _, _ string) error {
	return store.ErrNotFound
}

func (s *stubSessionAccountStore) CreateOrganization(_ context.Context, _, _, _ string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}

func (s *stubSessionAccountStore) AssignUserToOrg(_ context.Context, _, _, _ string) error {
	return store.ErrNotFound
}

func (s *stubSessionAccountStore) GetUserOrganization(_ context.Context, userID string) (*store.Organization, error) {
	if org, ok := s.orgByUserID[userID]; ok {
		return org, nil
	}
	return nil, store.ErrNotFound
}

func sessionOrgProbe(seen **store.APIKey, called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*seen = middleware.GetAPIKey(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func TestSessionOrg_PassesThroughRealAPIKey(t *testing.T) {
	accountStore := &stubSessionAccountStore{}
	realKey := &store.APIKey{ID: "key-1", OrgID: "org-real", Scopes: []string{"ocr:write"}}

	var seen *store.APIKey
	var called bool
	handler := middleware.SessionOrg(accountStore)(sessionOrgProbe(&seen, &called))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithAPIKey(req.Context(), realKey))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected downstream handler to be called")
	}
	if seen != realKey {
		t.Errorf("expected original API key to pass through untouched")
	}
}

func TestSessionOrg_PassesThroughWithoutUser(t *testing.T) {
	accountStore := &stubSessionAccountStore{}

	var seen *store.APIKey
	seen = &store.APIKey{ID: "sentinel"}
	var called bool
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		seen = middleware.GetAPIKey(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.SessionOrg(accountStore)(probe)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected downstream handler to be called")
	}
	if seen != nil {
		t.Errorf("expected no identity injected without session user, got %+v", seen)
	}
}

func TestSessionOrg_InjectsOrgFromSessionSubject(t *testing.T) {
	accountStore := &stubSessionAccountStore{
		orgByUserID: map[string]*store.Organization{
			"user-123": {ID: "org-123", Name: "Acme"},
		},
	}

	var seen *store.APIKey
	var called bool
	handler := middleware.SessionOrg(accountStore)(sessionOrgProbe(&seen, &called))

	user := &middleware.OIDCUser{Subject: "user-123", Email: "budi@acme.test", Roles: []string{"developer", "ocr:write"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected downstream to run, got status %d", rec.Code)
	}
	if seen == nil {
		t.Fatal("expected synthetic identity to be injected")
	}
	if seen.OrgID != "org-123" {
		t.Errorf("expected org org-123, got %q", seen.OrgID)
	}
	if seen.ID != "" {
		t.Errorf("expected empty synthetic key ID so metering records NULL, got %q", seen.ID)
	}
	if len(seen.Scopes) != 2 || seen.Scopes[1] != "ocr:write" {
		t.Errorf("expected user roles as scopes, got %v", seen.Scopes)
	}
}

func TestSessionOrg_StripsWildcardScopes(t *testing.T) {
	accountStore := &stubSessionAccountStore{
		orgByUserID: map[string]*store.Organization{
			"user-123": {ID: "org-123", Name: "Acme"},
		},
	}

	var seen *store.APIKey
	var called bool
	handler := middleware.SessionOrg(accountStore)(sessionOrgProbe(&seen, &called))

	user := &middleware.OIDCUser{Subject: "user-123", Email: "budi@acme.test", Roles: []string{"admin", "*", "ocr:write"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected downstream to run, got status %d", rec.Code)
	}
	if seen == nil {
		t.Fatal("expected synthetic identity to be injected")
	}
	if len(seen.Scopes) != 1 || seen.Scopes[0] != "ocr:write" {
		t.Errorf("expected wildcard roles stripped, got %v", seen.Scopes)
	}
}

func TestSessionOrg_FallsBackToEmailLookup(t *testing.T) {
	accountStore := &stubSessionAccountStore{
		userByEmail: map[string]*store.UserWithAuth{
			"budi@acme.test": {User: store.User{ID: "user-999", OrgID: "org-999", Email: "budi@acme.test"}},
		},
	}

	var seen *store.APIKey
	var called bool
	handler := middleware.SessionOrg(accountStore)(sessionOrgProbe(&seen, &called))

	user := &middleware.OIDCUser{Subject: "keycloak-sub-xyz", Email: "budi@acme.test", Roles: []string{"ocr:write"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || seen == nil {
		t.Fatal("expected synthetic identity via email fallback")
	}
	if seen.OrgID != "org-999" {
		t.Errorf("expected org org-999, got %q", seen.OrgID)
	}
}

func TestSessionOrg_RejectsOrgLessSessionUser(t *testing.T) {
	accountStore := &stubSessionAccountStore{}

	var called bool
	var seen *store.APIKey
	handler := middleware.SessionOrg(accountStore)(sessionOrgProbe(&seen, &called))

	user := &middleware.OIDCUser{Subject: "ghost", Email: "ghost@nowhere.test", Roles: []string{"ocr:write"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("expected downstream handler NOT to be called without organization")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSessionOrg_NilStorePassesThrough(t *testing.T) {
	var called bool
	var seen *store.APIKey
	handler := middleware.SessionOrg(nil)(sessionOrgProbe(&seen, &called))

	user := &middleware.OIDCUser{Subject: "user-1", Email: "a@b.test"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || seen != nil {
		t.Errorf("expected passthrough with nil store, called=%v seen=%+v", called, seen)
	}
}
