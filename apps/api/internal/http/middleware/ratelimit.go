package middleware

import (
	"context"
	"net"
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
	limiter      ratelimit.RateLimiter
	accountStore store.AccountStore
	defaultOrgID string
	replicas     int
	mu           sync.RWMutex
	cache        map[string]planCacheEntry
}

// NewRateLimitMiddleware initializes rate limiting middleware with cached plan limits.
func NewRateLimitMiddleware(limiter ratelimit.RateLimiter, accountStore store.AccountStore, defaultOrgID string) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:      limiter,
		accountStore: accountStore,
		defaultOrgID: defaultOrgID,
		replicas:     1,
		cache:        make(map[string]planCacheEntry),
	}
}

// SetReplicaCount tells the limiter how many API replicas share the load, so the
// per-replica budget can be divided down to approximate the advertised plan limit.
//
// The token buckets live in process memory, so with N replicas behind a load
// balancer a tenant could otherwise burst N x its plan limit. Dividing is an
// approximation: it assumes even load balancing and under-counts when replicas
// scale in. It is a deliberate ceiling — the exact fix is a shared counter
// (Redis / Postgres), which is the upgrade path once billing accuracy demands it.
func (m *RateLimitMiddleware) SetReplicaCount(n int) {
	if n < 1 {
		n = 1
	}
	m.mu.Lock()
	m.replicas = n
	m.cache = make(map[string]planCacheEntry) // cached limits were computed for the old divisor
	m.mu.Unlock()
}

// perReplicaLimit divides a plan limit across replicas, never below 1.
func (m *RateLimitMiddleware) perReplicaLimit(limit int) int {
	m.mu.RLock()
	replicas := m.replicas
	m.mu.RUnlock()

	if replicas <= 1 {
		return limit
	}
	if divided := limit / replicas; divided >= 1 {
		return divided
	}
	return 1
}

// Handler returns an http.Handler middleware enforcing rate limits and emitting standard RFC headers.
func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Probes, docs and cheap dashboard reads bypass the limiter. Account
		// *writes* deliberately do not: POST /api/v1/account/organizations is a
		// mutation and was previously unthrottled by the blanket prefix match.
		accountRead := strings.HasPrefix(r.URL.Path, "/api/v1/account") && r.Method == http.MethodGet
		if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" ||
			strings.HasPrefix(r.URL.Path, "/docs") || strings.HasPrefix(r.URL.Path, "/openapi") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/auth/me") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/auth/logout") ||
			accountRead {
			next.ServeHTTP(w, r)
			return
		}

		// Rate-limit key: prefer API-key org, then OIDC subject, then configured default org, then client IP.
		// Never bypass: every caller gets a bucket.
		rateKey := ""
		if key := GetAPIKey(r.Context()); key != nil && key.OrgID != "" {
			rateKey = key.OrgID
		} else if user := GetOIDCUser(r.Context()); user != nil && (user.Subject != "" || user.Email != "") {
			sub := user.Subject
			if sub == "" {
				sub = user.Email
			}
			rateKey = "oidc:" + sub
		} else if m.defaultOrgID != "" {
			rateKey = m.defaultOrgID
		} else if ip := clientIPFromRequest(r); ip != "" {
			rateKey = "ip:" + ip
		}
		if rateKey == "" {
			rateKey = "anonymous"
		}

		limit, planCode := m.getOrgRateLimit(r.Context(), rateKey)
		res := m.limiter.Allow(rateKey, m.perReplicaLimit(limit))

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetTime, 10))

		if !res.Allowed {
			telemetry.RecordRateLimitExceeded(r.Context(), rateKey, planCode)
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
	if orgID == "" || orgID == "00000000-0000-0000-0000-000000000001" {
		return 10, "free"
	}
	if strings.HasPrefix(orgID, "ip:") || orgID == "anonymous" {
		return 10, "free"
	}

	now := time.Now()

	m.mu.RLock()
	entry, exists := m.cache[orgID]
	m.mu.RUnlock()

	if exists && entry.expiresAt.After(now) {
		return entry.limit, entry.planCode
	}

	if strings.HasPrefix(orgID, "oidc:") {
		userID := strings.TrimPrefix(orgID, "oidc:")
		if m.accountStore != nil {
			var org *store.Organization
			var err error
			if strings.Contains(userID, "@") {
				if user, uErr := m.accountStore.GetUserByEmail(ctx, userID); uErr == nil && user != nil {
					org, err = m.accountStore.GetUserOrganization(ctx, user.ID)
				}
			} else {
				org, err = m.accountStore.GetUserOrganization(ctx, userID)
			}
			if err == nil && org != nil {
				limit, planCode := m.getOrgRateLimit(ctx, org.ID)
				m.mu.Lock()
				m.cache[orgID] = planCacheEntry{
					limit:     limit,
					planCode:  planCode,
					expiresAt: now.Add(1 * time.Minute),
				}
				m.mu.Unlock()
				return limit, planCode
			}
		}
		return 10, "free"
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

// clientIPFromRequest derives the rate-limit identity for unauthenticated callers.
//
// X-Forwarded-For is client-controlled: a caller can prepend any address they
// like. Only the RIGHTMOST entry is appended by our own ingress, so that is the
// one hop an attacker cannot forge; reading the leftmost entry (as before) let
// anyone mint a fresh bucket per request and skip the limit entirely.
func clientIPFromRequest(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if idx := strings.LastIndex(xff, ","); idx >= 0 {
			if last := strings.TrimSpace(xff[idx+1:]); last != "" {
				return last
			}
		} else {
			return xff
		}
	}
	if addr := strings.TrimSpace(r.RemoteAddr); addr != "" {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			return host
		}
		return addr
	}
	return ""
}
