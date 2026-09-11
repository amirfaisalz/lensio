package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
)

type planCacheEntry struct {
	limit     int
	planCode  string
	expiresAt time.Time
}

// RateLimitMiddleware manages per-tenant rate limits based on subscription tier.
type RateLimitMiddleware struct {
	limiter      *ratelimit.Limiter
	accountStore store.AccountStore
	defaultOrgID string
	mu           sync.RWMutex
	cache        map[string]planCacheEntry
}

// NewRateLimitMiddleware initializes rate limiting middleware with cached plan limits.
func NewRateLimitMiddleware(limiter *ratelimit.Limiter, accountStore store.AccountStore, defaultOrgID string) *RateLimitMiddleware {
	if defaultOrgID == "" {
		defaultOrgID = "00000000-0000-0000-0000-000000000001"
	}
	return &RateLimitMiddleware{
		limiter:      limiter,
		accountStore: accountStore,
		defaultOrgID: defaultOrgID,
		cache:        make(map[string]planCacheEntry),
	}
}

// Handler returns an http.Handler middleware enforcing rate limits and emitting standard RFC headers.
func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/ready" || strings.HasPrefix(r.URL.Path, "/docs") || strings.HasPrefix(r.URL.Path, "/openapi") || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		orgID := m.defaultOrgID
		if key := GetAPIKey(r.Context()); key != nil && key.OrgID != "" {
			orgID = key.OrgID
		}

		limit, planCode := m.getOrgRateLimit(r.Context(), orgID)
		res := m.limiter.Allow(orgID, limit)

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetTime, 10))

		if !res.Allowed {
			telemetry.RecordRateLimitExceeded(r.Context(), orgID, planCode)
			w.Header().Set("Retry-After", strconv.Itoa(res.RetryAfter))
			response.ErrorWithRequest(
				w,
				r,
				http.StatusTooManyRequests,
				response.CodeRateLimitExceeded,
				"API rate limit exceeded",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getOrgRateLimit looks up organization rate limit with a 1-minute in-memory cache to guarantee O(1) hot paths.
func (m *RateLimitMiddleware) getOrgRateLimit(ctx context.Context, orgID string) (int, string) {
	now := time.Now()

	m.mu.RLock()
	entry, exists := m.cache[orgID]
	m.mu.RUnlock()

	if exists && entry.expiresAt.After(now) {
		return entry.limit, entry.planCode
	}

	limit := 10 // default Free tier limit (10 req/min)
	planCode := "free"
	if m.accountStore != nil {
		plan, err := m.accountStore.GetOrganizationPlan(ctx, orgID)
		if err == nil && plan != nil {
			if plan.RateLimitPerMinute > 0 {
				limit = plan.RateLimitPerMinute
			}
			if plan.Code != "" {
				planCode = plan.Code
			}
		}
	}

	m.mu.Lock()
	m.cache[orgID] = planCacheEntry{
		limit:     limit,
		planCode:  planCode,
		expiresAt: now.Add(1 * time.Minute),
	}
	m.mu.Unlock()

	return limit, planCode
}
