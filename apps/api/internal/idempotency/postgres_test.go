package idempotency_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestPostgresStore_NilDB(t *testing.T) {
	pgStore := idempotency.NewPostgresStore(nil, 0)
	ctx := context.Background()

	_, _, err := pgStore.LockOrGet(ctx, "org-1", "key-1", "hash-1", 0)
	if err == nil {
		t.Fatal("expected error with nil db handle, got nil")
	}

	err = pgStore.Complete(ctx, "org-1", "key-1", 200, nil, nil)
	if err == nil {
		t.Fatal("expected error with nil db handle on Complete, got nil")
	}

	err = pgStore.Release(ctx, "org-1", "key-1")
	if err == nil {
		t.Fatal("expected error with nil db handle on Release, got nil")
	}

	_, err = pgStore.CleanupStale(ctx)
	if err == nil {
		t.Fatal("expected error with nil db handle on CleanupStale, got nil")
	}
}

func TestPostgresStore_Validation(t *testing.T) {
	pgStore := idempotency.NewPostgresStore(nil, time.Hour)
	ctx := context.Background()

	_, _, err := pgStore.LockOrGet(ctx, "", "k", "h", time.Hour)
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	_, _, err = pgStore.LockOrGet(ctx, "org-1", "", "h", time.Hour)
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}

	err = pgStore.Complete(ctx, "", "k", 200, nil, nil)
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	err = pgStore.Complete(ctx, "org-1", "", 200, nil, nil)
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}

	err = pgStore.Release(ctx, "", "k")
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	err = pgStore.Release(ctx, "org-1", "")
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}
}

func TestPostgresStore_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed applying migrations: %v", err)
	}

	pgStore := idempotency.NewPostgresStore(db.DB, 24*time.Hour)
	orgID := "00000000-0000-0000-0000-000000000001"
	key := fmt.Sprintf("pg_test_key_%d", time.Now().UnixNano())
	hash := "pg_test_hash_abcdef123456"

	// 1. Initial lock acquisition -> isNew is true
	rec, isNew, err := pgStore.LockOrGet(ctx, orgID, key, hash, time.Hour)
	if err != nil {
		t.Fatalf("failed acquiring initial postgres lock: %v", err)
	}
	if !isNew {
		t.Fatal("expected isNew = true on first acquisition in postgres")
	}
	if rec.Status != idempotency.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", rec.Status)
	}

	// 2. Duplicate acquisition while in_progress -> isNew is false
	rec2, isNew2, err := pgStore.LockOrGet(ctx, orgID, key, hash, time.Hour)
	if err != nil {
		t.Fatalf("failed second LockOrGet: %v", err)
	}
	if isNew2 {
		t.Fatal("expected isNew = false on second acquisition")
	}
	if rec2.Status != idempotency.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", rec2.Status)
	}

	// 3. Complete record
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Custom-H":   "val",
	}
	body := []byte(`{"result":"extracted"}`)
	err = pgStore.Complete(ctx, orgID, key, 200, headers, body)
	if err != nil {
		t.Fatalf("failed completing postgres record: %v", err)
	}

	// 4. Retrieve completed record
	rec3, isNew3, err := pgStore.LockOrGet(ctx, orgID, key, hash, time.Hour)
	if err != nil {
		t.Fatalf("failed LockOrGet on completed record: %v", err)
	}
	if isNew3 {
		t.Fatal("expected isNew = false on completed record")
	}
	if rec3.Status != idempotency.StatusCompleted {
		t.Fatalf("expected completed status, got %s", rec3.Status)
	}
	if rec3.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", rec3.StatusCode)
	}
	if rec3.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected cached header, got %v", rec3.Headers)
	}
	if string(rec3.ResponseBody) != string(body) {
		t.Fatalf("expected cached body %s, got %s", body, rec3.ResponseBody)
	}

	// 5. Test Release on an in-progress record
	keyTransient := fmt.Sprintf("pg_transient_%d", time.Now().UnixNano())
	_, isNew, err = pgStore.LockOrGet(ctx, orgID, keyTransient, hash, time.Hour)
	if err != nil || !isNew {
		t.Fatalf("failed acquiring transient lock: %v", err)
	}
	err = pgStore.Release(ctx, orgID, keyTransient)
	if err != nil {
		t.Fatalf("failed releasing in_progress key: %v", err)
	}
	// Verify it can be locked anew after release
	_, isNew, err = pgStore.LockOrGet(ctx, orgID, keyTransient, hash, time.Hour)
	if err != nil || !isNew {
		t.Fatalf("failed acquiring lock anew after release: %v", err)
	}

	// 6. Test Multi-tenant isolation: different org with same key
	otherOrgID := "00000000-0000-0000-0000-000000000002"
	// Ensure other org exists for foreign key
	_, _ = db.DB.ExecContext(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, 'Tenant Two', 'tenant-two')
		ON CONFLICT (id) DO NOTHING;
	`, otherOrgID)

	recOther, isNewOther, err := pgStore.LockOrGet(ctx, otherOrgID, key, hash, time.Hour)
	if err != nil {
		t.Fatalf("failed acquiring lock for different tenant: %v", err)
	}
	if !isNewOther {
		t.Fatal("expected isNew = true for different tenant with same key")
	}
	if recOther.OrgID != otherOrgID {
		t.Fatalf("expected orgID %s, got %s", otherOrgID, recOther.OrgID)
	}

	// 7. Test CleanupStale
	cleaned, err := pgStore.CleanupStale(ctx)
	if err != nil {
		t.Fatalf("failed CleanupStale: %v", err)
	}
	t.Logf("cleaned %d stale rows", cleaned)
}
