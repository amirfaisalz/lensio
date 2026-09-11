package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

func TestAccountStore_ValidationErrors(t *testing.T) {
	var db store.DB
	ctx := context.Background()

	if _, err := db.GetOrganization(ctx, ""); err == nil {
		t.Fatal("expected error for empty orgID in GetOrganization")
	}

	if _, err := db.GetOrganizationPlan(ctx, ""); err == nil {
		t.Fatal("expected error for empty orgID in GetOrganizationPlan")
	}

	if err := db.UpdateOrganizationPlan(ctx, "", "pro"); err == nil {
		t.Fatal("expected error for empty orgID in UpdateOrganizationPlan")
	}

	if err := db.UpdateOrganizationPlan(ctx, "00000000-0000-0000-0000-000000000001", ""); err == nil {
		t.Fatal("expected error for empty planCode in UpdateOrganizationPlan")
	}
}

func TestAccountStore_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://nusaid:nusaid_dev_password@localhost:5432/nusaid?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	defaultOrgID := "00000000-0000-0000-0000-000000000001"

	// 1. Get default organization
	org, err := db.GetOrganization(ctx, defaultOrgID)
	if err != nil {
		t.Fatalf("failed getting default organization: %v", err)
	}
	if org.ID != defaultOrgID {
		t.Errorf("expected org ID %s, got %s", defaultOrgID, org.ID)
	}
	if org.PlanCode == "" {
		t.Errorf("expected non-empty plan code")
	}

	// 2. Get organization plan
	plan, err := db.GetOrganizationPlan(ctx, defaultOrgID)
	if err != nil {
		t.Fatalf("failed getting organization plan: %v", err)
	}
	if plan.Code == "" || plan.MonthlyQuota <= 0 {
		t.Errorf("unexpected plan details: %+v", plan)
	}

	// 3. Update organization plan to 'pro'
	if err := db.UpdateOrganizationPlan(ctx, defaultOrgID, "pro"); err != nil {
		t.Fatalf("failed updating plan to pro: %v", err)
	}

	updatedOrg, err := db.GetOrganization(ctx, defaultOrgID)
	if err != nil {
		t.Fatalf("failed getting updated organization: %v", err)
	}
	if updatedOrg.PlanCode != "pro" || updatedOrg.MonthlyQuota != 10000 {
		t.Errorf("expected plan pro with quota 10000, got %s / %d", updatedOrg.PlanCode, updatedOrg.MonthlyQuota)
	}

	// 4. Update with unknown plan code
	err = db.UpdateOrganizationPlan(ctx, defaultOrgID, "non_existent_tier")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown plan, got %v", err)
	}

	// 5. Update non-existent organization
	err = db.UpdateOrganizationPlan(ctx, "00000000-0000-0000-0000-999999999999", "free")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing org, got %v", err)
	}

	// Reset back to 'free' for clean state
	_ = db.UpdateOrganizationPlan(ctx, defaultOrgID, "free")
}
