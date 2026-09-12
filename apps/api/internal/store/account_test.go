package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
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

	if _, err := db.GetOrganizationMembers(ctx, ""); err == nil {
		t.Fatal("expected error for empty orgID in GetOrganizationMembers")
	}

	// Validation checks for new user & org operations
	if _, err := db.CreateUser(ctx, "", "test@example.com", "hash", "tok"); err == nil {
		t.Fatal("expected error for empty fullName in CreateUser")
	}
	if _, err := db.CreateUser(ctx, "Name", "", "hash", "tok"); err == nil {
		t.Fatal("expected error for empty email in CreateUser")
	}
	if _, err := db.CreateUser(ctx, "Name", "test@example.com", "", "tok"); err == nil {
		t.Fatal("expected error for empty passwordHash in CreateUser")
	}

	if _, err := db.GetUserByEmail(ctx, ""); err == nil {
		t.Fatal("expected error for empty email in GetUserByEmail")
	}

	if _, err := db.GetUserByID(ctx, ""); err == nil {
		t.Fatal("expected error for empty userID in GetUserByID")
	}

	if err := db.VerifyUserEmail(ctx, "", "tok"); err == nil {
		t.Fatal("expected error for empty email in VerifyUserEmail")
	}

	if _, err := db.CreateOrganization(ctx, "", "slug", "free"); err == nil {
		t.Fatal("expected error for empty name in CreateOrganization")
	}
	if _, err := db.CreateOrganization(ctx, "Org", "", "free"); err == nil {
		t.Fatal("expected error for empty slug in CreateOrganization")
	}

	if err := db.AssignUserToOrg(ctx, "", "org-1", "owner"); err == nil {
		t.Fatal("expected error for empty userID in AssignUserToOrg")
	}
	if err := db.AssignUserToOrg(ctx, "u-1", "", "owner"); err == nil {
		t.Fatal("expected error for empty orgID in AssignUserToOrg")
	}

	if _, err := db.GetUserOrganization(ctx, ""); err == nil {
		t.Fatal("expected error for empty userID in GetUserOrganization")
	}
}

func TestAccountStore_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	testOrg := createTestOrg(t, db)
	testOrgID := testOrg.ID

	// 1. Get organization
	org, err := db.GetOrganization(ctx, testOrgID)
	if err != nil {
		t.Fatalf("failed getting organization: %v", err)
	}
	if org.ID != testOrgID {
		t.Errorf("expected org ID %s, got %s", testOrgID, org.ID)
	}
	if org.PlanCode == "" {
		t.Errorf("expected non-empty plan code")
	}

	// 2. Get organization plan
	plan, err := db.GetOrganizationPlan(ctx, testOrgID)
	if err != nil {
		t.Fatalf("failed getting organization plan: %v", err)
	}
	if plan.Code == "" || plan.MonthlyQuota <= 0 {
		t.Errorf("unexpected plan details: %+v", plan)
	}

	// 3. Update organization plan to 'pro'
	if err := db.UpdateOrganizationPlan(ctx, testOrgID, "pro"); err != nil {
		t.Fatalf("failed updating plan to pro: %v", err)
	}

	updatedOrg, err := db.GetOrganization(ctx, testOrgID)
	if err != nil {
		t.Fatalf("failed getting updated organization: %v", err)
	}
	if updatedOrg.PlanCode != "pro" || updatedOrg.MonthlyQuota != 10000 {
		t.Errorf("expected plan pro with quota 10000, got %s / %d", updatedOrg.PlanCode, updatedOrg.MonthlyQuota)
	}

	// 4. Update with unknown plan code
	err = db.UpdateOrganizationPlan(ctx, testOrgID, "non_existent_tier")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown plan, got %v", err)
	}

	// 5. Update non-existent organization
	err = db.UpdateOrganizationPlan(ctx, "00000000-0000-0000-0000-999999999999", "free")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing org, got %v", err)
	}

	// 6. Get organization members
	members, err := db.GetOrganizationMembers(ctx, testOrgID)
	if err != nil {
		t.Fatalf("failed getting organization members: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected 0 members for newly created org, got %d", len(members))
	}

	// Reset back to 'free' for clean state
	_ = db.UpdateOrganizationPlan(ctx, testOrgID, "free")

	// 7. Test User Registration, Verification, Login & Organization Creation in PostgreSQL
	timestamp := time.Now().UnixNano()
	testEmail := fmt.Sprintf("testuser_%d@lensio.dev", timestamp)
	testPasswordHash := "salt$hash12345"
	testToken := fmt.Sprintf("tok_%d", timestamp)

	// Create user
	createdUser, err := db.CreateUser(ctx, "Test User", testEmail, testPasswordHash, testToken)
	if err != nil {
		t.Fatalf("failed creating user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = db.DB.ExecContext(cleanupCtx, "DELETE FROM users WHERE id = $1", createdUser.ID)
	})
	if createdUser.Email != testEmail {
		t.Errorf("expected email %s, got %s", testEmail, createdUser.Email)
	}
	if createdUser.EmailVerified {
		t.Errorf("expected new user to not be verified initially")
	}
	if createdUser.OrgID != "" {
		t.Errorf("expected org_id to be empty for fresh user, got %s", createdUser.OrgID)
	}

	// Duplicate email check
	_, err = db.CreateUser(ctx, "Duplicate", testEmail, testPasswordHash, "tok2")
	if !errors.Is(err, store.ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}

	// Get user by email
	userAuth, err := db.GetUserByEmail(ctx, testEmail)
	if err != nil {
		t.Fatalf("failed getting user by email: %v", err)
	}
	if userAuth.PasswordHash != testPasswordHash {
		t.Errorf("expected password hash %s, got %s", testPasswordHash, userAuth.PasswordHash)
	}
	if userAuth.EmailVerified {
		t.Errorf("expected email to be unverified")
	}

	// Get user by ID
	userByID, err := db.GetUserByID(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("failed getting user by ID: %v", err)
	}
	if userByID.ID != createdUser.ID {
		t.Errorf("expected ID %s, got %s", createdUser.ID, userByID.ID)
	}

	// Get non-existent user
	_, err = db.GetUserByEmail(ctx, "nonexistent@lensio.dev")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for nonexistent user, got %v", err)
	}

	// Verify with wrong token
	err = db.VerifyUserEmail(ctx, testEmail, "wrong_token")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong verification token, got %v", err)
	}

	// Verify with correct token
	err = db.VerifyUserEmail(ctx, testEmail, testToken)
	if err != nil {
		t.Fatalf("failed verifying email: %v", err)
	}

	verifiedUser, err := db.GetUserByEmail(ctx, testEmail)
	if err != nil {
		t.Fatalf("failed getting verified user: %v", err)
	}
	if !verifiedUser.EmailVerified {
		t.Errorf("expected user to be verified now")
	}

	// Fresh user has no organization yet
	_, err = db.GetUserOrganization(ctx, createdUser.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for user without organization, got %v", err)
	}

	// Create Organization for user
	orgSlug := fmt.Sprintf("test-org-%d", timestamp)
	newOrg, err := db.CreateOrganization(ctx, "Test Company", orgSlug, "starter")
	if err != nil {
		t.Fatalf("failed creating organization: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = db.DB.ExecContext(cleanupCtx, "DELETE FROM organizations WHERE id = $1", newOrg.ID)
	})
	if newOrg.PlanCode != "starter" {
		t.Errorf("expected starter plan, got %s", newOrg.PlanCode)
	}

	// Assign user to organization
	err = db.AssignUserToOrg(ctx, createdUser.ID, newOrg.ID, "owner")
	if err != nil {
		t.Fatalf("failed assigning user to org: %v", err)
	}

	// Now GetUserOrganization succeeds
	userOrg, err := db.GetUserOrganization(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("failed getting user organization: %v", err)
	}
	if userOrg.ID != newOrg.ID {
		t.Errorf("expected org ID %s, got %s", newOrg.ID, userOrg.ID)
	}
}
