package quota_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/quota"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type mockAccountStore struct {
	plan *store.Plan
	err  error
}

func (m *mockAccountStore) GetOrganization(ctx context.Context, orgID string) (*store.Organization, error) {
	return nil, nil
}

func (m *mockAccountStore) GetOrganizationPlan(ctx context.Context, orgID string) (*store.Plan, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.plan, nil
}

func (m *mockAccountStore) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	return nil
}

func (m *mockAccountStore) GetOrganizationMembers(ctx context.Context, orgID string) ([]store.User, error) {
	return nil, nil
}

type mockUsageStore struct {
	count int
	err   error
}

func (m *mockUsageStore) CreateUsageRecord(ctx context.Context, rec *store.UsageRecord) error {
	return nil
}

func (m *mockUsageStore) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*store.UsageSummary, error) {
	return nil, nil
}

func (m *mockUsageStore) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]store.DailyUsage, error) {
	return nil, nil
}

func (m *mockUsageStore) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]store.EndpointUsage, error) {
	return nil, nil
}

func (m *mockUsageStore) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

func (m *mockUsageStore) GetUsageRecords(ctx context.Context, orgID string, filter store.UsageRecordFilter) ([]store.UsageRecord, int, error) {
	return nil, 0, nil
}

func TestCurrentBillingCycle(t *testing.T) {
	// Mid-month
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	start, reset := quota.CurrentBillingCycle(now)

	expectedStart := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	expectedReset := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(expectedStart) {
		t.Errorf("expected start %v, got %v", expectedStart, start)
	}
	if !reset.Equal(expectedReset) {
		t.Errorf("expected reset %v, got %v", expectedReset, reset)
	}

	// December rollover
	dec := time.Date(2026, time.December, 31, 23, 59, 0, 0, time.UTC)
	startDec, resetDec := quota.CurrentBillingCycle(dec)

	expectedDecReset := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !startDec.Equal(time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("expected dec start 2026-12-01, got %v", startDec)
	}
	if !resetDec.Equal(expectedDecReset) {
		t.Errorf("expected dec reset 2027-01-01, got %v", resetDec)
	}
}

func TestEnforcer_CheckQuota(t *testing.T) {
	ctx := context.Background()

	t.Run("empty orgID returns error", func(t *testing.T) {
		e := quota.NewEnforcer(nil, nil)
		_, _, _, err := e.CheckQuota(ctx, "")
		if err == nil {
			t.Fatal("expected error for empty orgID")
		}
	})

	t.Run("nil stores fallback to allowed", func(t *testing.T) {
		e := quota.NewEnforcer(nil, nil)
		allowed, remaining, limit, err := e.CheckQuota(ctx, "org-1")
		if err != nil || !allowed || remaining != 100 || limit != 100 {
			t.Fatalf("unexpected fallback result: allowed=%v, remaining=%d, limit=%d, err=%v", allowed, remaining, limit, err)
		}
	})

	t.Run("account store error returns error", func(t *testing.T) {
		e := quota.NewEnforcer(
			&mockAccountStore{err: errors.New("db error")},
			&mockUsageStore{},
		)
		_, _, _, err := e.CheckQuota(ctx, "org-1")
		if err == nil {
			t.Fatal("expected error from account store")
		}
	})

	t.Run("usage store error returns error", func(t *testing.T) {
		e := quota.NewEnforcer(
			&mockAccountStore{plan: &store.Plan{MonthlyQuota: 100}},
			&mockUsageStore{err: errors.New("db error")},
		)
		_, _, _, err := e.CheckQuota(ctx, "org-1")
		if err == nil {
			t.Fatal("expected error from usage store")
		}
	})

	t.Run("quota remaining allows request", func(t *testing.T) {
		e := quota.NewEnforcer(
			&mockAccountStore{plan: &store.Plan{MonthlyQuota: 100}},
			&mockUsageStore{count: 42},
		)
		allowed, remaining, limit, err := e.CheckQuota(ctx, "org-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed || remaining != 58 || limit != 100 {
			t.Errorf("expected allowed with 58 remaining, got allowed=%v, remaining=%d, limit=%d", allowed, remaining, limit)
		}
	})

	t.Run("quota exhausted denies request", func(t *testing.T) {
		e := quota.NewEnforcer(
			&mockAccountStore{plan: &store.Plan{MonthlyQuota: 100}},
			&mockUsageStore{count: 100},
		)
		allowed, remaining, limit, err := e.CheckQuota(ctx, "org-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed || remaining != 0 || limit != 100 {
			t.Errorf("expected denied with 0 remaining, got allowed=%v, remaining=%d, limit=%d", allowed, remaining, limit)
		}
	})

	t.Run("usage strictly exceeding quota also denies request", func(t *testing.T) {
		e := quota.NewEnforcer(
			&mockAccountStore{plan: &store.Plan{MonthlyQuota: 100}},
			&mockUsageStore{count: 105},
		)
		allowed, remaining, limit, err := e.CheckQuota(ctx, "org-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed || remaining != 0 || limit != 100 {
			t.Errorf("expected denied with 0 remaining, got allowed=%v, remaining=%d, limit=%d", allowed, remaining, limit)
		}
	})
}
