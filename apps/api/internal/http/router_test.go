package http_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/nusaid/apps/api/internal/http"
	"github.com/amirfaisalz/nusaid/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/internal/usage"
	"github.com/amirfaisalz/nusaid/tests/fixtures/synthetic"
)

type dummyPinger struct{}

func (d *dummyPinger) PingContext(ctx context.Context) error {
	return nil
}

type dummyKeyStore struct {
	keys map[string]*store.APIKey
}

func newDummyKeyStore() *dummyKeyStore {
	return &dummyKeyStore{
		keys: make(map[string]*store.APIKey),
	}
}

func (d *dummyKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	key.ID = "dummy-id-1"
	d.keys[key.KeyHash] = key
	return nil
}

func (d *dummyKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	k, ok := d.keys[keyHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return k, nil
}

func (d *dummyKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	return nil, nil
}

func (d *dummyKeyStore) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	return nil
}

func (d *dummyKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	return nil
}

type dummyOCRStore struct {
	records map[string]*store.OCRRequest
}

func newDummyOCRStore() *dummyOCRStore {
	return &dummyOCRStore{
		records: make(map[string]*store.OCRRequest),
	}
}

func (d *dummyOCRStore) CreateOCRRequest(ctx context.Context, req *store.OCRRequest) error {
	req.CreatedAt = time.Now()
	d.records[req.ID] = req
	return nil
}

func (d *dummyOCRStore) GetOCRRequestByID(ctx context.Context, orgID string, id string) (*store.OCRRequest, error) {
	rec, ok := d.records[id]
	if !ok || rec.OrgID != orgID {
		return nil, store.ErrNotFound
	}
	return rec, nil
}

func TestNewRouter(t *testing.T) {
	kStore := newDummyKeyStore()
	gen, _ := apikey.Generate(apikey.EnvLive)
	_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:          "dummy-id-1",
		OrgID:       "org-1",
		KeyHash:     gen.KeyHash,
		Scopes:      []string{"ocr:write", "ocr:read"},
		Environment: "live",
	})

	dummyStore := newDummyOCRStore()
	_ = dummyStore.CreateOCRRequest(context.Background(), &store.OCRRequest{
		ID:         "test-ocr-id",
		OrgID:      "org-1",
		Status:     "completed",
		Confidence: 0.98,
		LatencyMS:  100,
		DocType:    "ktp",
	})

	router := internalhttp.NewRouter(&dummyPinger{}, kStore, nil, dummyStore)

	tests := []struct {
		name           string
		method         string
		path           string
		authHeader     string
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
			name:           "openapi endpoint",
			method:         http.MethodGet,
			path:           "/openapi",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "openapi.yaml endpoint",
			method:         http.MethodGet,
			path:           "/openapi.yaml",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "docs endpoint",
			method:         http.MethodGet,
			path:           "/docs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "auth verify without key returns 401",
			method:         http.MethodGet,
			path:           "/api/v1/auth/verify",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "auth verify with valid key returns 200",
			method:         http.MethodGet,
			path:           "/api/v1/auth/verify",
			authHeader:     "Bearer " + gen.Plaintext,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ocr post without key returns 401",
			method:         http.MethodPost,
			path:           "/api/v1/ocr/ktp",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "ocr get without key returns 401",
			method:         http.MethodGet,
			path:           "/api/v1/ocr/test-ocr-id",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "ocr get with valid key returns 200",
			method:         http.MethodGet,
			path:           "/api/v1/ocr/test-ocr-id",
			authHeader:     "Bearer " + gen.Plaintext,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unknown endpoint returns 404",
			method:         http.MethodGet,
			path:           "/unknown",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d for %s %s, got %d", tc.expectedStatus, tc.method, tc.path, rec.Code)
			}

			// Verify X-Request-ID header is always emitted on responses
			if reqID := rec.Header().Get("X-Request-ID"); reqID == "" {
				t.Error("expected X-Request-ID header to be present on response")
			}
		})
	}
}

func TestNewRouter_NilKeyStore(t *testing.T) {
	router := internalhttp.NewRouter(&dummyPinger{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

type dummyUsageStore struct {
	mu      sync.Mutex
	records []*store.UsageRecord
}

func (d *dummyUsageStore) CreateUsageRecord(ctx context.Context, rec *store.UsageRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.records = append(d.records, rec)
	return nil
}

func (d *dummyUsageStore) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*store.UsageSummary, error) {
	return &store.UsageSummary{
		TotalRequests:     10,
		SuccessCount:      10,
		ErrorCount:        0,
		QuotaLimit:        planQuota,
		QuotaRemaining:    planQuota - 10,
		BillingCycleReset: cycleReset,
	}, nil
}

func (d *dummyUsageStore) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]store.DailyUsage, error) {
	return []store.DailyUsage{
		{Date: "2026-09-01", TotalRequests: 10, SuccessCount: 10, ErrorCount: 0},
	}, nil
}

func (d *dummyUsageStore) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]store.EndpointUsage, error) {
	return []store.EndpointUsage{
		{Endpoint: "/api/v1/ocr/ktp", TotalRequests: 10, AvgLatencyMS: 120},
	}, nil
}

func (d *dummyUsageStore) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	return 10, nil
}

type dummyAccountStore struct {
	org  *store.Organization
	plan *store.Plan
}

func (d *dummyAccountStore) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	if d.org != nil {
		return d.org, nil
	}
	return &store.Organization{
		ID:              orgID,
		Name:            "Acme Org",
		Slug:            "acme",
		PlanCode:        "starter",
		PlanName:        "Starter Tier",
		ActiveKeysCount: 1,
		CreatedAt:       time.Now(),
	}, nil
}

func (d *dummyAccountStore) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	if d.plan != nil {
		return d.plan, nil
	}
	return &store.Plan{
		Code:               "starter",
		Name:               "Starter Tier",
		MonthlyQuota:       1000,
		RateLimitPerMinute: 30,
	}, nil
}

func (d *dummyAccountStore) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	if d.plan != nil {
		d.plan.Code = planCode
	}
	return nil
}

type dummyAuditStore struct {
	mu   sync.Mutex
	logs []*store.AuditLog
}

func (d *dummyAuditStore) RecordAuditLog(ctx context.Context, log *store.AuditLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logs = append(d.logs, log)
	return nil
}

func (d *dummyAuditStore) ListAuditLogsByOrg(ctx context.Context, orgID string) ([]*store.AuditLog, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.logs, nil
}

func TestNewRouterWithDeps_Phase4(t *testing.T) {
	kStore := newDummyKeyStore()
	gen, _ := apikey.Generate(apikey.EnvLive)
	_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:          "dummy-id-1",
		OrgID:       "org-1",
		KeyHash:     gen.KeyHash,
		Scopes:      []string{"ocr:write", "ocr:read", "usage:read"},
		Environment: "live",
	})

	uStore := &dummyUsageStore{}
	uRecorder := usage.NewRecorder(uStore, 100)
	aStore := &dummyAccountStore{}
	auditStore := &dummyAuditStore{}
	limiter := ratelimit.NewLimiter()

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		Pinger:        &dummyPinger{},
		KeyStore:      kStore,
		OCRStore:      newDummyOCRStore(),
		UsageStore:    uStore,
		AccountStore:  aStore,
		AuditStore:    auditStore,
		RateLimiter:   limiter,
		UsageRecorder: uRecorder,
	})

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = uRecorder.Close(ctx)
	}()

	authHeader := "Bearer " + gen.Plaintext

	// 1. Test GET /api/v1/usage
	t.Run("GET /api/v1/usage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("expected X-RateLimit-Limit header")
		}
	})

	// 2. Test GET /api/v1/usage/daily
	t.Run("GET /api/v1/usage/daily", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/daily", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	// 3. Test GET /api/v1/usage/endpoints
	t.Run("GET /api/v1/usage/endpoints", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/endpoints", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	// 4. Test GET /api/v1/account
	t.Run("GET /api/v1/account", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	// 5. Test GET /api/v1/account/plan
	t.Run("GET /api/v1/account/plan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/plan", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	// 6. Test PUT /api/v1/account/plan
	t.Run("PUT /api/v1/account/plan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/account/plan", bytes.NewReader([]byte(`{"plan_code":"pro"}`)))
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if len(auditStore.logs) == 0 {
			t.Error("expected audit log recorded on plan update")
		}
	})

	// 7. Test POST /api/v1/ocr/ktp with valid synthetic image
	t.Run("POST /api/v1/ocr/ktp", func(t *testing.T) {
		validImg := synthetic.GenerateValidKTPImage()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("document", "ktp.png")
		_, _ = part.Write(validImg)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	// 8. Test Rate Limiter 429
	t.Run("Rate Limiting Exceeded triggers 429", func(t *testing.T) {
		exhaustOrg := "org-exhaust"
		genExhaust, _ := apikey.Generate(apikey.EnvLive)
		_ = kStore.CreateAPIKey(context.Background(), &store.APIKey{
			ID:          "dummy-exhaust",
			OrgID:       exhaustOrg,
			KeyHash:     genExhaust.KeyHash,
			Scopes:      []string{"ocr:read", "usage:read"},
			Environment: "live",
		})
		header := "Bearer " + genExhaust.Plaintext

		// Send 35 requests to exceed default limit of 30 req/min
		hit429 := false
		for i := 0; i < 35; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
			req.Header.Set("Authorization", header)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				hit429 = true
				if rec.Header().Get("Retry-After") == "" {
					t.Error("expected Retry-After header on 429")
				}
				break
			}
		}

		if !hit429 {
			t.Fatal("expected 429 rate limit exceeded after bursting requests")
		}
	})
}
