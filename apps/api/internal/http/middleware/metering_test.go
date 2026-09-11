package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/internal/usage"
)

type mockUsageStoreForMiddleware struct {
	mu      sync.Mutex
	records []*store.UsageRecord
}

func (m *mockUsageStoreForMiddleware) CreateUsageRecord(ctx context.Context, rec *store.UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	return nil
}

func (m *mockUsageStoreForMiddleware) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*store.UsageSummary, error) {
	return nil, nil
}

func (m *mockUsageStoreForMiddleware) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]store.DailyUsage, error) {
	return nil, nil
}

func (m *mockUsageStoreForMiddleware) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]store.EndpointUsage, error) {
	return nil, nil
}

func (m *mockUsageStoreForMiddleware) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	return 0, nil
}

func TestUsageMeteringMiddleware(t *testing.T) {
	mockStore := &mockUsageStoreForMiddleware{}
	recorder := usage.NewRecorder(mockStore, 10)

	mw := middleware.UsageMetering(recorder, "default-org")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	key := &store.APIKey{
		ID:    "key-123",
		OrgID: "custom-org",
	}
	req = req.WithContext(middleware.WithAPIKey(req.Context(), key))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	// Close recorder to flush worker
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("closing recorder failed: %v", err)
	}

	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	if len(mockStore.records) != 1 {
		t.Fatalf("expected 1 record persisted, got %d", len(mockStore.records))
	}

	r := mockStore.records[0]
	if r.OrgID != "custom-org" {
		t.Errorf("expected org custom-org, got %s", r.OrgID)
	}
	if r.APIKeyID == nil || *r.APIKeyID != "key-123" {
		t.Errorf("expected apiKeyID key-123, got %v", r.APIKeyID)
	}
	if r.Endpoint != "/api/v1/ocr/ktp" {
		t.Errorf("expected endpoint /api/v1/ocr/ktp, got %s", r.Endpoint)
	}
	if r.StatusCode != http.StatusCreated {
		t.Errorf("expected status code 201, got %d", r.StatusCode)
	}
}

func TestUsageMeteringMiddleware_NilRecorder(t *testing.T) {
	mw := middleware.UsageMetering(nil, "")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
