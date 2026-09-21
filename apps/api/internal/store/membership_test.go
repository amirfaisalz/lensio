package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func membershipTestDB(t *testing.T) *store.DB {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}
	return db
}

// The central capability this table adds: one user, several organizations.
// users.org_id could never express it, because AssignUserToOrg overwrote it.
func TestOrganizationMembership_UserBelongsToSeveralOrgs(t *testing.T) {
	db := membershipTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	orgA := createTestOrg(t, db)
	orgB := createTestOrg(t, db)

	email := fmt.Sprintf("member_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Multi Org User", email, "$2a$10$hash", "tok")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	if err := db.AssignUserToOrg(ctx, user.ID, orgA.ID, "owner"); err != nil {
		t.Fatalf("assigning to org A: %v", err)
	}
	if err := db.AssignUserToOrg(ctx, user.ID, orgB.ID, "member"); err != nil {
		t.Fatalf("assigning to org B: %v", err)
	}

	memberships, err := db.ListUserOrganizations(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserOrganizations: %v", err)
	}
	if len(memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(memberships))
	}

	roles := map[string]string{}
	for _, m := range memberships {
		roles[m.ID] = m.Role
	}
	if roles[orgA.ID] != "owner" {
		t.Errorf("role in org A = %q, want owner", roles[orgA.ID])
	}
	if roles[orgB.ID] != "member" {
		t.Errorf("role in org B = %q, want member", roles[orgB.ID])
	}

	// Membership in the second org must survive: joining B used to erase A.
	for _, org := range []*store.Organization{orgA, orgB} {
		member, role, err := db.IsOrgMember(ctx, user.ID, org.ID)
		if err != nil {
			t.Fatalf("IsOrgMember(%s): %v", org.ID, err)
		}
		if !member {
			t.Errorf("user should still be a member of %s", org.ID)
		}
		if role == "" {
			t.Errorf("expected a role in %s", org.ID)
		}
	}
}

func TestOrganizationMembership_NonMembersAreRefused(t *testing.T) {
	db := membershipTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	org := createTestOrg(t, db)
	outsider := fmt.Sprintf("outsider_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Outsider", outsider, "$2a$10$hash", "tok")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	member, role, err := db.IsOrgMember(ctx, user.ID, org.ID)
	if err != nil {
		t.Fatalf("IsOrgMember: %v", err)
	}
	if member || role != "" {
		t.Errorf("a non-member must not be reported as a member (member=%v role=%q)", member, role)
	}

	if memberships, err := db.ListUserOrganizations(ctx, user.ID); err != nil || len(memberships) != 0 {
		t.Errorf("expected no memberships, got %d (err=%v)", len(memberships), err)
	}
}

func TestOrganizationMembership_RoleIsUpdatedNotDuplicated(t *testing.T) {
	db := membershipTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	org := createTestOrg(t, db)
	email := fmt.Sprintf("promote_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Promoted User", email, "$2a$10$hash", "tok")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	if err := db.AssignUserToOrg(ctx, user.ID, org.ID, "member"); err != nil {
		t.Fatalf("first assignment: %v", err)
	}
	if err := db.AssignUserToOrg(ctx, user.ID, org.ID, "admin"); err != nil {
		t.Fatalf("promotion: %v", err)
	}

	memberships, err := db.ListUserOrganizations(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserOrganizations: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("re-assignment must update the row, not add one: got %d memberships", len(memberships))
	}
	if memberships[0].Role != "admin" {
		t.Errorf("role = %q, want admin after promotion", memberships[0].Role)
	}
}

func TestOrganizationMembership_RejectsBlankArguments(t *testing.T) {
	db := membershipTestDB(t)
	ctx := context.Background()

	if _, err := db.ListUserOrganizations(ctx, "  "); err == nil {
		t.Error("expected an error for a blank user id")
	}
	if member, _, err := db.IsOrgMember(ctx, "", "org"); err != nil || member {
		t.Errorf("blank user id must be a non-member without error, got member=%v err=%v", member, err)
	}
	if member, _, err := db.IsOrgMember(ctx, "user", ""); err != nil || member {
		t.Errorf("blank org id must be a non-member without error, got member=%v err=%v", member, err)
	}
}
