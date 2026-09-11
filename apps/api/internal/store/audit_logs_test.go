package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

func TestAuditStore_ValidationErrors(t *testing.T) {
	var db store.DB
	ctx := context.Background()

	if err := db.RecordAuditLog(ctx, nil); err == nil {
		t.Fatal("expected error when inserting nil audit log")
	}

	if err := db.RecordAuditLog(ctx, &store.AuditLog{Action: "test"}); err == nil {
		t.Fatal("expected error for empty orgID")
	}

	if err := db.RecordAuditLog(ctx, &store.AuditLog{OrgID: "org-1"}); err == nil {
		t.Fatal("expected error for empty action")
	}

	if _, err := db.ListAuditLogsByOrg(ctx, ""); err == nil {
		t.Fatal("expected error for empty orgID in ListAuditLogsByOrg")
	}
}

func TestAuditStore_LiveDB(t *testing.T) {
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

	log := &store.AuditLog{
		OrgID:          defaultOrgID,
		ActorID:        "test-actor",
		Action:         "api_key.create",
		TargetResource: "api_key:test-key-123",
		Metadata: map[string]any{
			"key_name": "Test Key",
			"scopes":   []string{"ocr:write"},
		},
	}

	if err := db.RecordAuditLog(ctx, log); err != nil {
		t.Fatalf("failed recording audit log: %v", err)
	}
	if log.ID == "" || log.CreatedAt.IsZero() {
		t.Fatal("expected non-empty ID and CreatedAt after insert")
	}

	// Retrieve logs
	logs, err := db.ListAuditLogsByOrg(ctx, defaultOrgID)
	if err != nil {
		t.Fatalf("failed listing audit logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("expected at least 1 audit log")
	}
	if logs[0].Action == "" {
		t.Errorf("expected non-empty action in retrieved audit log")
	}
}
