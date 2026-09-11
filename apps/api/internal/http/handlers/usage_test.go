package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

type mockUsageStore struct {
	summary   *store.UsageSummary
	daily     []store.DailyUsage
	endpoints []store.EndpointUsage
	ocrCount  int
	err       error
}

func (m *mockUsageStore) CreateUsageRecord(ctx context.Context, rec *store.UsageRecord) error {
	return nil
}

func (m *mockUsageStore) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*store.UsageSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summary, nil
}

func (m *mockUsageStore) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]store.DailyUsage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.daily, nil
}

func (m *mockUsageStore) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]store.EndpointUsage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.endpoints, nil
}

func (m *mockUsageStore) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.ocrCount, nil
}

type mockAccountStoreForUsage struct {
	plan *store.Plan
	err  error
}

func (m *mockAccountStoreForUsage) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAccountStoreForUsage) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.plan, nil
}

func (m *mockAccountStoreForUsage) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	return nil
}

func TestUsageSummaryHandler(t *testing.T) {
	t.Run("nil usage store returns empty summary", func(t *testing.T) {
		handler := handlers.UsageSummaryHandler(nil, nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("successful summary retrieval", func(t *testing.T) {
		uStore := &mockUsageStore{
			summary: &store.UsageSummary{
				TotalRequests:  50,
				SuccessCount:   48,
				ErrorCount:     2,
				QuotaLimit:     1000,
				QuotaRemaining: 950,
			},
		}
		aStore := &mockAccountStoreForUsage{
			plan: &store.Plan{MonthlyQuota: 1000},
		}

		handler := handlers.UsageSummaryHandler(uStore, aStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		uStore := &mockUsageStore{err: errors.New("db error")}
		handler := handlers.UsageSummaryHandler(uStore, nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestDailyUsageHandler(t *testing.T) {
	t.Run("nil store returns empty array", func(t *testing.T) {
		handler := handlers.DailyUsageHandler(nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/daily", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("successful daily retrieval", func(t *testing.T) {
		uStore := &mockUsageStore{
			daily: []store.DailyUsage{
				{Date: "2026-09-01", TotalRequests: 10, SuccessCount: 10, ErrorCount: 0},
			},
		}
		handler := handlers.DailyUsageHandler(uStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/daily", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		uStore := &mockUsageStore{err: errors.New("query failed")}
		handler := handlers.DailyUsageHandler(uStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/daily", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestEndpointUsageHandler(t *testing.T) {
	t.Run("nil store returns empty array", func(t *testing.T) {
		handler := handlers.EndpointUsageHandler(nil, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/endpoints", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("successful endpoint retrieval", func(t *testing.T) {
		uStore := &mockUsageStore{
			endpoints: []store.EndpointUsage{
				{Endpoint: "/api/v1/ocr/ktp", TotalRequests: 25, AvgLatencyMS: 120.5},
			},
		}
		handler := handlers.EndpointUsageHandler(uStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/endpoints", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		uStore := &mockUsageStore{err: errors.New("query failed")}
		handler := handlers.EndpointUsageHandler(uStore, "org-1")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/endpoints", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}
