package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// throttleAccountStore counts how many password checks actually reach the store,
// which is what a brute-force run is trying to maximise.
type throttleAccountStore struct{ lookups int }

func (a *throttleAccountStore) GetOrganization(context.Context, string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}
func (a *throttleAccountStore) GetOrganizationPlan(context.Context, string) (*store.Plan, error) {
	return &store.Plan{Code: "free", RateLimitPerMinute: 10, MonthlyQuota: 100}, nil
}
func (a *throttleAccountStore) UpdateOrganizationPlan(context.Context, string, string) error {
	return nil
}
func (a *throttleAccountStore) GetOrganizationMembers(context.Context, string) ([]store.User, error) {
	return nil, nil
}
func (a *throttleAccountStore) CreateUser(context.Context, string, string, string, string) (*store.User, error) {
	return nil, nil
}
func (a *throttleAccountStore) GetUserByEmail(context.Context, string) (*store.UserWithAuth, error) {
	a.lookups++
	return nil, store.ErrNotFound
}
func (a *throttleAccountStore) GetUserByID(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (a *throttleAccountStore) VerifyUserEmail(context.Context, string, string) error { return nil }
func (a *throttleAccountStore) CreateOrganization(context.Context, string, string, string) (*store.Organization, error) {
	return nil, nil
}
func (a *throttleAccountStore) AssignUserToOrg(context.Context, string, string, string) error {
	return nil
}
func (a *throttleAccountStore) GetUserOrganization(context.Context, string) (*store.Organization, error) {
	return nil, store.ErrNotFound
}

type throttleKeyStore struct{}

func (throttleKeyStore) CreateAPIKey(context.Context, *store.APIKey) error { return nil }
func (throttleKeyStore) GetAPIKeyByHash(context.Context, string) (*store.APIKey, error) {
	return nil, store.ErrNotFound
}
func (throttleKeyStore) ListAPIKeysByOrg(context.Context, string) ([]*store.APIKey, error) {
	return nil, nil
}
func (throttleKeyStore) RevokeAPIKey(context.Context, string, string) error { return nil }
func (throttleKeyStore) TouchAPIKeyLastUsed(context.Context, string, time.Time) error {
	return nil
}

// TestPublicAuthRoutesAreThrottled guards the wiring, not the limiter: the
// limiter was already correct, but it was never placed in the public auth chain,
// so login accepted unlimited password guesses and unlimited bcrypt work.
func TestPublicAuthRoutesAreThrottled(t *testing.T) {
	for _, route := range []struct{ path, body string }{
		{"/api/v1/auth/login", `{"email":"victim@example.com","password":"guess"}`},
		{"/api/v1/auth/register", `{"full_name":"A","email":"a@example.com","password":"password123"}`},
		{"/api/v1/auth/verify-email", `{"email":"a@example.com","token":"deadbeef"}`},
	} {
		t.Run(route.path, func(t *testing.T) {
			acct := &throttleAccountStore{}
			router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
				KeyStore:     throttleKeyStore{},
				AccountStore: acct,
				RateLimiter:  ratelimit.NewLimiter(),
			})

			const attempts = 40
			throttled := 0
			for i := 0; i < attempts; i++ {
				req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
				req.RemoteAddr = "203.0.113.9:1234"
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code == http.StatusTooManyRequests {
					throttled++
				}
			}

			if throttled == 0 {
				t.Fatalf("%s accepted all %d attempts from one IP with no rate limiting", route.path, attempts)
			}
		})
	}
}

// A spoofed X-Forwarded-For must not mint a fresh bucket per request.
func TestSpoofedForwardedForCannotEscapeThrottle(t *testing.T) {
	acct := &throttleAccountStore{}
	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:     throttleKeyStore{},
		AccountStore: acct,
		RateLimiter:  ratelimit.NewLimiter(),
	})

	throttled := 0
	for i := 0; i < 40; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			strings.NewReader(`{"email":"victim@example.com","password":"guess"}`))
		req.RemoteAddr = "203.0.113.9:1234"
		// Attacker-prepended hop; only the rightmost entry is appended by our ingress.
		req.Header.Set("X-Forwarded-For", "10.0.0."+string(rune('0'+i%10))+", 203.0.113.9")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			throttled++
		}
	}

	if throttled == 0 {
		t.Fatal("rotating the leftmost X-Forwarded-For entry bypassed the rate limit")
	}
}
