package quota

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

// CurrentBillingCycle returns the start of the current month and the reset timestamp (1st of next month) in UTC.
func CurrentBillingCycle(now time.Time) (start time.Time, reset time.Time) {
	u := now.UTC()
	start = time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	// Add 1 month to get the first of next month
	year, month, _ := start.Date()
	if month == time.December {
		reset = time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)
	} else {
		reset = time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	}
	return start, reset
}

// Enforcer evaluates monthly quota limits prior to running expensive OCR workloads.
type Enforcer struct {
	accountStore store.AccountStore
	usageStore   store.UsageStore
	nowFunc      func() time.Time
}

// NewEnforcer creates a new quota enforcer.
func NewEnforcer(accountStore store.AccountStore, usageStore store.UsageStore) *Enforcer {
	return &Enforcer{
		accountStore: accountStore,
		usageStore:   usageStore,
		nowFunc:      time.Now,
	}
}

// CheckQuota verifies if the organization has remaining OCR requests in the current billing cycle.
// Returns (allowed, remaining, limit, error).
func (e *Enforcer) CheckQuota(ctx context.Context, orgID string) (bool, int, int, error) {
	if orgID == "" {
		return false, 0, 0, errors.New("orgID is required")
	}
	if e.accountStore == nil || e.usageStore == nil {
		// If stores are not available, allow by default in ephemeral mode
		return true, 100, 100, nil
	}

	plan, err := e.accountStore.GetOrganizationPlan(ctx, orgID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("retrieving org plan for quota: %w", err)
	}

	limit := plan.MonthlyQuota
	cycleStart, _ := CurrentBillingCycle(e.nowFunc())

	used, err := e.usageStore.GetMonthlyOCRCount(ctx, orgID, cycleStart)
	if err != nil {
		return false, 0, 0, fmt.Errorf("retrieving monthly usage count: %w", err)
	}

	remaining := limit - used
	if remaining <= 0 {
		return false, 0, limit, nil
	}

	return true, remaining, limit, nil
}
