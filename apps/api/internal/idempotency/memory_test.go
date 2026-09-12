package idempotency_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
)

func TestMemoryStore_Validation(t *testing.T) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()

	// LockOrGet empty orgID
	_, _, err := store.LockOrGet(ctx, "", "key-1", "hash-1", time.Hour)
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	// LockOrGet empty key
	_, _, err = store.LockOrGet(ctx, "org-1", "", "hash-1", time.Hour)
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}

	// Complete empty orgID
	err = store.Complete(ctx, "", "key-1", 200, nil, nil)
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	// Complete empty key
	err = store.Complete(ctx, "org-1", "", 200, nil, nil)
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}

	// Release empty orgID
	err = store.Release(ctx, "", "key-1")
	if !errors.Is(err, idempotency.ErrOrgIDRequired) {
		t.Fatalf("expected ErrOrgIDRequired, got %v", err)
	}

	// Release empty key
	err = store.Release(ctx, "org-1", "")
	if !errors.Is(err, idempotency.ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}
}

func TestMemoryStore_Lifecycle(t *testing.T) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()

	// 1. Initial lock acquisition -> isNew is true
	rec, isNew, err := store.LockOrGet(ctx, "org-1", "key-1", "hash-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Fatal("expected isNew = true on first acquisition")
	}
	if rec.Status != idempotency.StatusInProgress {
		t.Fatalf("expected status %s, got %s", idempotency.StatusInProgress, rec.Status)
	}
	if rec.RequestHash != "hash-1" {
		t.Fatalf("expected hash 'hash-1', got %s", rec.RequestHash)
	}

	// 2. Subsequent call with same key -> isNew is false, returns existing in_progress record
	rec2, isNew2, err := store.LockOrGet(ctx, "org-1", "key-1", "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNew2 {
		t.Fatal("expected isNew = false on second acquisition")
	}
	if rec2.Status != idempotency.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", rec2.Status)
	}

	// 3. Complete the record
	headers := map[string]string{"Content-Type": "application/json"}
	body := []byte(`{"status":"ok"}`)
	err = store.Complete(ctx, "org-1", "key-1", 200, headers, body)
	if err != nil {
		t.Fatalf("unexpected error on Complete: %v", err)
	}

	// Complete on non-existent key returns cleanly
	if err := store.Complete(ctx, "org-1", "key-unknown", 200, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4. Subsequent call returns completed record with cached body and headers
	rec3, isNew3, err := store.LockOrGet(ctx, "org-1", "key-1", "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNew3 {
		t.Fatal("expected isNew = false for completed key")
	}
	if rec3.Status != idempotency.StatusCompleted {
		t.Fatalf("expected status %s, got %s", idempotency.StatusCompleted, rec3.Status)
	}
	if rec3.StatusCode != 200 {
		t.Fatalf("expected status code 200, got %d", rec3.StatusCode)
	}
	if rec3.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type header, got %v", rec3.Headers)
	}
	if string(rec3.ResponseBody) != string(body) {
		t.Fatalf("expected body %s, got %s", body, rec3.ResponseBody)
	}

	// 5. Release on a completed record does not delete it
	err = store.Release(ctx, "org-1", "key-1")
	if err != nil {
		t.Fatalf("unexpected error on Release: %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("expected 1 record after release on completed, got %d", store.Count())
	}
}

func TestMemoryStore_ReleaseInProgress(t *testing.T) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()

	// Lock in-progress
	_, isNew, err := store.LockOrGet(ctx, "org-1", "key-transient", "hash-1", time.Hour)
	if err != nil || !isNew {
		t.Fatalf("failed acquiring initial lock: %v", err)
	}

	// Release in-progress record
	err = store.Release(ctx, "org-1", "key-transient")
	if err != nil {
		t.Fatalf("unexpected error on Release: %v", err)
	}
	if store.Count() != 0 {
		t.Fatalf("expected 0 records after releasing in_progress, got %d", store.Count())
	}

	// Can be acquired anew
	_, isNew, err = store.LockOrGet(ctx, "org-1", "key-transient", "hash-1", time.Hour)
	if err != nil || !isNew {
		t.Fatalf("failed acquiring lock after release: %v", err)
	}
}

func TestMemoryStore_TTLAndCleanup(t *testing.T) {
	store := idempotency.NewMemoryStore(10 * time.Millisecond)
	ctx := context.Background()

	_, isNew, err := store.LockOrGet(ctx, "org-1", "key-exp", "hash-1", 10*time.Millisecond)
	if err != nil || !isNew {
		t.Fatalf("failed initial lock: %v", err)
	}

	time.Sleep(25 * time.Millisecond)

	// Key should be expired, so LockOrGet treats it as new
	_, isNew2, err := store.LockOrGet(ctx, "org-1", "key-exp", "hash-new", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error after TTL: %v", err)
	}
	if !isNew2 {
		t.Fatal("expected isNew = true for expired key")
	}

	// Add a key with very short TTL and run CleanupStale
	_, _, _ = store.LockOrGet(ctx, "org-1", "key-clean", "hash-clean", 5*time.Millisecond)
	time.Sleep(15 * time.Millisecond)

	cleaned := store.CleanupStale()
	if cleaned < 1 {
		t.Fatalf("expected at least 1 record cleaned, got %d", cleaned)
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()
	concurrency := 20
	key := "concurrent-key"
	hash := "shared-hash"

	var wg sync.WaitGroup
	wg.Add(concurrency)

	newCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, isNew, err := store.LockOrGet(ctx, "org-1", key, hash, time.Hour)
			if err != nil {
				t.Errorf("unexpected error in goroutine: %v", err)
				return
			}
			if isNew {
				mu.Lock()
				newCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Exactly one goroutine must win the lock acquisition
	if newCount != 1 {
		t.Fatalf("expected exactly 1 isNew=true among %d concurrent workers, got %d", concurrency, newCount)
	}
}
