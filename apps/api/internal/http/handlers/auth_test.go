package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type mockAccountStore struct {
	orgs          map[string]*store.Organization
	users         map[string]*store.UserWithAuth
	plans         map[string]*store.Plan
	createUserErr error
	verifyErr     error
	getOrgErr     error
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{
		orgs:  make(map[string]*store.Organization),
		users: make(map[string]*store.UserWithAuth),
		plans: map[string]*store.Plan{
			"free": {
				ID:                 "plan-free",
				Code:               "free",
				Name:               "Free Tier",
				MonthlyQuota:       100,
				RateLimitPerMinute: 10,
			},
		},
	}
}

func (m *mockAccountStore) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	if m.getOrgErr != nil {
		return nil, m.getOrgErr
	}
	org, ok := m.orgs[orgID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return org, nil
}

func (m *mockAccountStore) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	return m.plans["free"], nil
}

func (m *mockAccountStore) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	return nil
}

func (m *mockAccountStore) GetOrganizationMembers(ctx context.Context, orgID string) ([]store.User, error) {
	var members []store.User
	for _, u := range m.users {
		if u.OrgID == orgID {
			members = append(members, u.User)
		}
	}
	return members, nil
}

func (m *mockAccountStore) CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*store.User, error) {
	if m.createUserErr != nil {
		return nil, m.createUserErr
	}
	if _, exists := m.users[email]; exists {
		return nil, store.ErrDuplicateEmail
	}
	u := &store.UserWithAuth{
		User: store.User{
			ID:            "user-" + email,
			Email:         email,
			FullName:      fullName,
			Role:          "member",
			EmailVerified: false,
			CreatedAt:     time.Now(),
		},
		PasswordHash:      passwordHash,
		VerificationToken: verificationToken,
	}
	m.users[email] = u
	return &u.User, nil
}

func (m *mockAccountStore) GetUserByEmail(ctx context.Context, email string) (*store.UserWithAuth, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (m *mockAccountStore) GetUserByID(ctx context.Context, userID string) (*store.User, error) {
	for _, u := range m.users {
		if u.ID == userID {
			return &u.User, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockAccountStore) VerifyUserEmail(ctx context.Context, email, token string) error {
	if m.verifyErr != nil {
		return m.verifyErr
	}
	u, ok := m.users[email]
	if !ok {
		return store.ErrNotFound
	}
	if token != "" && u.VerificationToken != token {
		return store.ErrNotFound
	}
	u.EmailVerified = true
	u.VerificationToken = ""
	return nil
}

func (m *mockAccountStore) CreateOrganization(ctx context.Context, name, slug, planCode string) (*store.Organization, error) {
	org := &store.Organization{
		ID:                 "org-" + slug,
		Name:               name,
		Slug:               slug,
		PlanCode:           planCode,
		PlanName:           "Free Tier",
		MonthlyQuota:       100,
		RateLimitPerMinute: 10,
		CreatedAt:          time.Now(),
	}
	m.orgs[org.ID] = org
	return org, nil
}

func (m *mockAccountStore) AssignUserToOrg(ctx context.Context, userID, orgID, role string) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.OrgID = orgID
			u.Role = role
			return nil
		}
	}
	return store.ErrNotFound
}

func (m *mockAccountStore) GetUserOrganization(ctx context.Context, userID string) (*store.Organization, error) {
	for _, u := range m.users {
		if u.ID == userID && u.OrgID != "" {
			return m.GetOrganization(ctx, u.OrgID)
		}
	}
	return nil, store.ErrNotFound
}

func TestRegisterHandler(t *testing.T) {
	// Nil store
	hNil := handlers.RegisterHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	hNil.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil store, got %d", rec.Code)
	}

	mockStore := newMockAccountStore()
	h := handlers.RegisterHandler(mockStore)

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`invalid-json`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid json, got %d", rec.Code)
	}

	// Empty name
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"","email":"test@example.com","password":"password123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d", rec.Code)
	}

	// Invalid email
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"Test","email":"notanemail","password":"password123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid email, got %d", rec.Code)
	}

	// Short password
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"Test","email":"test@example.com","password":"short"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", rec.Code)
	}

	// Valid registration
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"Test User","email":"test@example.com","password":"password123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 for valid registration, got %d", rec.Code)
	}

	// Duplicate email
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"Test User","email":"test@example.com","password":"password123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate email, got %d", rec.Code)
	}

	// Store error
	mockStore.createUserErr = errors.New("db down")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"full_name":"New","email":"new@example.com","password":"password123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for db error, got %d", rec.Code)
	}
}

func TestVerifyEmailHandler(t *testing.T) {
	// Nil store
	hNil := handlers.VerifyEmailHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	hNil.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil store, got %d", rec.Code)
	}

	mockStore := newMockAccountStore()
	h := handlers.VerifyEmailHandler(mockStore)

	// Seed user with token
	passHash, _ := auth.HashPassword("password123")
	_, _ = mockStore.CreateUser(context.Background(), "Alice", "alice@example.com", passHash, "valid-token")

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`invalid`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid json, got %d", rec.Code)
	}

	// Empty email
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{"email":""}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty email, got %d", rec.Code)
	}

	// Invalid token
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{"email":"alice@example.com","token":"wrong"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for wrong token, got %d", rec.Code)
	}

	// Valid token
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{"email":"alice@example.com","token":"valid-token"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", rec.Code)
	}

	// Generic db error
	mockStore.verifyErr = errors.New("db error")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{"email":"alice@example.com","token":"valid-token"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for db error, got %d", rec.Code)
	}
}

func TestLoginHandler(t *testing.T) {
	// Nil store
	hNil := handlers.LoginHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	hNil.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil store, got %d", rec.Code)
	}

	mockStore := newMockAccountStore()
	h := handlers.LoginHandler(mockStore)

	passHash, _ := auth.HashPassword("validpassword123")
	_, _ = mockStore.CreateUser(context.Background(), "Bob", "bob@example.com", passHash, "token")

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`bad`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json, got %d", rec.Code)
	}

	// Empty email/password
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"","password":""}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty fields, got %d", rec.Code)
	}

	// Nonexistent user
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"unknown@example.com","password":"validpassword123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown user, got %d", rec.Code)
	}

	// Wrong password
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"bob@example.com","password":"wrongpassword"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", rec.Code)
	}

	// Unverified user -> 403 Forbidden
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"bob@example.com","password":"validpassword123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unverified user, got %d", rec.Code)
	}

	// Verify Bob
	_ = mockStore.VerifyUserEmail(context.Background(), "bob@example.com", "token")

	// Successful login
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"bob@example.com","password":"validpassword123"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for successful login, got %d", rec.Code)
	}

	var loginResp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&loginResp)
	if loginResp["access_token"] == nil || loginResp["access_token"] == "" {
		t.Errorf("expected access_token in login response")
	}

	// Verify secure cookie
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "lensio_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected lensio_session cookie to be set")
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("expected cookie to be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite Lax")
	}
	if sessionCookie.Value == "" {
		t.Errorf("expected non-empty cookie value")
	}
}

func TestLogoutHandler(t *testing.T) {
	h := handlers.LogoutHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for logout, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "lensio_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected lensio_session cookie to be present in logout response")
	}
	if sessionCookie.MaxAge != -1 || sessionCookie.Value != "" {
		t.Errorf("expected cookie to be cleared with MaxAge -1 and empty value")
	}
}

func TestMeHandler(t *testing.T) {
	mockStore := newMockAccountStore()
	h := handlers.MeHandler(mockStore)

	// Unauthenticated
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when unauthenticated, got %d", rec.Code)
	}

	// Authenticated OIDC user
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	ctx := middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{
		Subject: "user-123",
		Email:   "user@example.com",
		Name:    "User Name",
		Roles:   []string{"developer"},
	})
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for authenticated user, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["user"] == nil {
		t.Errorf("expected user in response")
	}

	// Authenticated API key user
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	ctx = middleware.WithAPIKey(req.Context(), &store.APIKey{
		ID:          "key-1",
		OrgID:       "org-1",
		Name:        "Test Key",
		Prefix:      "lensio_live_",
		Environment: "live",
		Scopes:      []string{"ocr:write"},
	})
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for authenticated API key, got %d", rec.Code)
	}
}

func TestCreateOrganizationHandler(t *testing.T) {
	// Nil store
	hNil := handlers.CreateOrganizationHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/organizations", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	hNil.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil store, got %d", rec.Code)
	}

	mockStore := newMockAccountStore()
	h := handlers.CreateOrganizationHandler(mockStore)

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/v1/account/organizations", bytes.NewBufferString(`bad`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json, got %d", rec.Code)
	}

	// Empty name
	req = httptest.NewRequest(http.MethodPost, "/api/v1/account/organizations", bytes.NewBufferString(`{"name":""}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d", rec.Code)
	}

	// Success with authenticated OIDC user context
	req = httptest.NewRequest(http.MethodPost, "/api/v1/account/organizations", bytes.NewBufferString(`{"name":"Acme Corp","plan_code":"free"}`))
	ctx := middleware.WithOIDCUser(req.Context(), &middleware.OIDCUser{
		Subject: "user-bob@example.com",
		Email:   "bob@example.com",
	})
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 for org creation, got %d", rec.Code)
	}
}
