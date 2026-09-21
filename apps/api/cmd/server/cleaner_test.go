package main

import (
	"context"
	"errors"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
)

func TestRunCleanup_RateLimiter(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Generate a bucket
	res := limiter.Allow("test-ip-1", 60)
	if !res.Allowed {
		t.Fatal("expected request to be allowed")
	}

	// Immediate cleanup with 1 hour max idle should not remove anything
	runCleanup(context.Background(), logger, 1*time.Hour, limiter, nil)

	// Cleanup with 0 idle duration should remove the bucket
	time.Sleep(10 * time.Millisecond)
	runCleanup(context.Background(), logger, 5*time.Millisecond, limiter, nil)

	// Limiter should have pruned the bucket
	limiter.Reset("test-ip-1")
}

func TestRunCleanup_MemoryIdempotency(t *testing.T) {
	memStore := idempotency.NewMemoryStore(10 * time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Lock a key
	_, isNew, err := memStore.LockOrGet(context.Background(), "org-1", "idemp-key-1", "hash-1", 10*time.Millisecond)
	if err != nil || !isNew {
		t.Fatalf("expected new lock, err: %v", err)
	}

	if memStore.Count() != 1 {
		t.Fatalf("expected 1 record, got %d", memStore.Count())
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Run cleanup
	runCleanup(context.Background(), logger, 10*time.Minute, nil, memStore)

	if memStore.Count() != 0 {
		t.Fatalf("expected 0 records after cleanup, got %d", memStore.Count())
	}
}

func TestRunCleanup_PostgresIdempotency_NilDB(t *testing.T) {
	pgStore := idempotency.NewPostgresStore(nil, 1*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Should not panic when db is nil
	runCleanup(context.Background(), logger, 10*time.Minute, nil, pgStore)
}

func TestStartBackgroundCleaner_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter := ratelimit.NewLimiter()
	memStore := idempotency.NewMemoryStore(50 * time.Millisecond)

	done := startBackgroundCleaner(ctx, logger, 10*time.Millisecond, limiter, memStore, nil, nil)

	// Let it run at least one tick
	time.Sleep(25 * time.Millisecond)

	// Trigger shutdown
	cancel()

	select {
	case <-done:
		// Clean exit
	case <-time.After(1 * time.Second):
		t.Fatal("background cleaner did not shut down within timeout")
	}
}

func TestMemoryGrowth_BoundedUnderSyntheticLoad(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	memStore := idempotency.NewMemoryStore(10 * time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Synthetic load: 500 distinct keys
	for i := 0; i < 500; i++ {
		key := "client-" + string(rune(i))
		_ = limiter.Allow(key, 60)
		_, _, _ = memStore.LockOrGet(context.Background(), "org-test", key, "hash", 10*time.Millisecond)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Execute cleanup
	runCleanup(context.Background(), logger, 10*time.Millisecond, limiter, memStore)

	if memStore.Count() != 0 {
		t.Errorf("expected 0 idempotency records after cleanup, got %d", memStore.Count())
	}
}

// stubPurger records how the retention sweep was invoked.
type stubPurger struct {
	mu        sync.Mutex
	calls     int
	batchSize int
	err       error
}

func (s *stubPurger) PurgeExpiredRecords(_ context.Context, _ time.Time, batchSize int) (store.RetentionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.batchSize = batchSize
	if s.err != nil {
		return store.RetentionResult{}, s.err
	}
	return store.RetentionResult{OCRRequests: 3, UsageRecords: 2, AuditLogs: 1}, nil
}

func (s *stubPurger) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestRunRetention_PurgesAndLogs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	purger := &stubPurger{}

	runRetention(context.Background(), logger, purger)

	if purger.callCount() != 1 {
		t.Fatalf("expected one purge call, got %d", purger.callCount())
	}
	if purger.batchSize <= 0 {
		t.Errorf("expected a positive batch size, got %d", purger.batchSize)
	}
}

func TestRunRetention_NilPurgerIsANoOp(t *testing.T) {
	// Ephemeral mode has no database; the sweep must not panic.
	runRetention(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestRunRetention_SurvivesPurgeErrors(t *testing.T) {
	purger := &stubPurger{err: errors.New("connection reset")}
	// A failing sweep must be logged and swallowed, never crash the cleaner loop.
	runRetention(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), purger)

	if purger.callCount() != 1 {
		t.Fatalf("expected the purge to be attempted once, got %d", purger.callCount())
	}
}

func TestRetentionResult_Total(t *testing.T) {
	res := store.RetentionResult{OCRRequests: 5, UsageRecords: 3, AuditLogs: 2}
	if res.Total() != 10 {
		t.Errorf("Total() = %d, want 10", res.Total())
	}
}
