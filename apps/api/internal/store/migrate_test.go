package store_test

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/migrations"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestMigrationsFS(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("failed to read embedded migrations: %v", err)
	}

	expectedFiles := []string{
		"000001_create_organizations.up.sql",
		"000001_create_organizations.down.sql",
		"000002_create_users.up.sql",
		"000002_create_users.down.sql",
		"000003_create_plans.up.sql",
		"000003_create_plans.down.sql",
		"000004_create_api_keys.up.sql",
		"000004_create_api_keys.down.sql",
		"000005_create_ocr_requests.up.sql",
		"000005_create_ocr_requests.down.sql",
		"000006_create_usage_records.up.sql",
		"000006_create_usage_records.down.sql",
		"000007_create_audit_logs.up.sql",
		"000007_create_audit_logs.down.sql",
		"000008_seed_default_plans.up.sql",
		"000008_seed_default_plans.down.sql",
	}

	foundMap := make(map[string]bool)
	for _, entry := range entries {
		foundMap[entry.Name()] = true

		// Ensure no migration file is empty
		data, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			t.Fatalf("failed reading migration file %s: %v", entry.Name(), err)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			t.Fatalf("migration file %s is empty", entry.Name())
		}
	}

	for _, expected := range expectedFiles {
		if !foundMap[expected] {
			t.Errorf("missing expected migration file: %s", expected)
		}
	}

	// Verify iofs driver can load the embedded filesystem
	driver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("iofs.New failed: %v", err)
	}
	defer func() {
		_ = driver.Close()
	}()

	first, err := driver.First()
	if err != nil {
		t.Fatalf("failed getting first migration version: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected first migration version to be 1, got %d", first)
	}
}

func TestRunMigrations_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://nusaid:nusaid_dev_password@localhost:5432/nusaid?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live migration test (db unavailable): %v", err)
	}
	defer db.Close()

	// 1. Run migrations Up
	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations up: %v", err)
	}

	// 2. Run migrations Up again (idempotent no change)
	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations up on no-change: %v", err)
	}

	// 3. Verify plans table has seeded rows
	var planCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM plans").Scan(&planCount)
	if err != nil {
		t.Fatalf("failed querying plans table: %v", err)
	}
	if planCount != 3 {
		t.Fatalf("expected 3 seeded plans, got %d", planCount)
	}

	// 4. Test NewMigrator constructor
	m, err := store.NewMigrator(db.DB)
	if err != nil {
		t.Fatalf("failed to create new migrator: %v", err)
	}
	version, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if dirty {
		t.Fatalf("database schema is in dirty state")
	}
	if version != 8 {
		t.Fatalf("expected migration version 8, got %d", version)
	}

	// 5. Test RunMigrationsDown (rollback)
	if err := store.RunMigrationsDown(db.DB); err != nil {
		t.Fatalf("failed running migrations down: %v", err)
	}

	// 6. Test RunMigrationsDown again (idempotent no change)
	if err := store.RunMigrationsDown(db.DB); err != nil {
		t.Fatalf("failed running migrations down on no-change: %v", err)
	}

	// 7. Migrate back Up so DB remains in ready state
	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed re-running migrations up: %v", err)
	}
}

func TestRunMigrations_ErrorBranches(t *testing.T) {
	closedDB, err := store.New(context.Background(), "postgres://invalid:user@127.0.0.1:54399/nusaid?sslmode=disable")
	if err == nil && closedDB != nil {
		_ = closedDB.Close()
	}

	// Test NewMigrator with nil / invalid db driver
	_, err = store.NewMigrator(nil)
	if err == nil {
		t.Fatal("expected error from NewMigrator with nil DB, got nil")
	}

	if err := store.RunMigrationsUp(nil); err == nil {
		t.Fatal("expected error from RunMigrationsUp with nil DB, got nil")
	}

	if err := store.RunMigrationsDown(nil); err == nil {
		t.Fatal("expected error from RunMigrationsDown with nil DB, got nil")
	}
}

