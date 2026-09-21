package ratelimit_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("skipping live database tests: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Without the counter table the limiter silently falls back to per-replica
	// limiting, so these tests would pass without ever reaching PostgreSQL.
	// Create it from the migration file itself rather than running golang-migrate:
	// test packages execute in parallel, and two concurrent migration runs leave
	// the schema_migrations row dirty. The statements are CREATE .. IF NOT EXISTS,
	// so applying them here is idempotent and lock-free.
	applyRateLimitSchema(t, db)

	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('public.rate_limit_counters') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("rate_limit_counters table is missing; the limiter would silently degrade (err=%v)", err)
	}

	return db
}

// applyRateLimitSchema executes the counter table's migration directly from the
// embedded SQL, keeping the test in step with the real schema without taking the
// migration lock other packages may hold.
func applyRateLimitSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	sqlBytes, err := migrations.FS.ReadFile("000016_rate_limit_counters.up.sql")
	if err != nil {
		t.Fatalf("reading the rate limit migration: %v", err)
	}
	if _, err := db.Exec(string(sqlBytes)); err != nil {
		t.Fatalf("applying the rate limit schema: %v", err)
	}
}

func uniqueKey(t *testing.T, db *sql.DB) string {
	t.Helper()
	key := fmt.Sprintf("test-%s-%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM rate_limit_counters WHERE bucket_key = $1`, key)
	})
	return key
}

func TestPostgresLimiter_EnforcesTheLimit(t *testing.T) {
	db := openTestDB(t)
	limiter := ratelimit.NewPostgresLimiter(db, nil)
	key := uniqueKey(t, db)

	const limit = 5
	allowed := 0
	for i := 0; i < limit*2; i++ {
		if limiter.Allow(key, limit).Allowed {
			allowed++
		}
	}

	if allowed != limit {
		t.Errorf("expected exactly %d requests allowed, got %d", limit, allowed)
	}
}

func TestPostgresLimiter_ReportsHeaders(t *testing.T) {
	db := openTestDB(t)
	limiter := ratelimit.NewPostgresLimiter(db, nil)
	key := uniqueKey(t, db)

	first := limiter.Allow(key, 3)
	if !first.Allowed || first.Limit != 3 || first.Remaining != 2 {
		t.Errorf("first call: allowed=%v limit=%d remaining=%d, want true/3/2",
			first.Allowed, first.Limit, first.Remaining)
	}
	if first.ResetTime <= time.Now().Unix()-1 {
		t.Errorf("reset time %d should be in the future", first.ResetTime)
	}

	limiter.Allow(key, 3)
	limiter.Allow(key, 3)
	denied := limiter.Allow(key, 3)
	if denied.Allowed {
		t.Fatal("the fourth call against a limit of 3 must be denied")
	}
	if denied.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", denied.Remaining)
	}
	if denied.RetryAfter < 1 {
		t.Errorf("RetryAfter = %d, want at least 1 second", denied.RetryAfter)
	}
}

// Two limiter instances stand in for two API replicas: they must share one budget.
func TestPostgresLimiter_IsSharedAcrossReplicas(t *testing.T) {
	db := openTestDB(t)
	replicaA := ratelimit.NewPostgresLimiter(db, nil)
	replicaB := ratelimit.NewPostgresLimiter(db, nil)
	key := uniqueKey(t, db)

	const limit = 4
	allowed := 0
	for i := 0; i < limit*2; i++ {
		l := replicaA
		if i%2 == 1 {
			l = replicaB
		}
		if l.Allow(key, limit).Allowed {
			allowed++
		}
	}

	if allowed != limit {
		t.Errorf("two replicas sharing one counter allowed %d requests, want %d", allowed, limit)
	}
}

// The upsert must not lose increments under concurrency.
func TestPostgresLimiter_IsAtomicUnderConcurrency(t *testing.T) {
	db := openTestDB(t)
	limiter := ratelimit.NewPostgresLimiter(db, nil)
	key := uniqueKey(t, db)

	const limit = 10
	const callers = 40

	var wg sync.WaitGroup
	results := make([]bool, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = limiter.Allow(key, limit).Allowed
		}(i)
	}
	wg.Wait()

	allowed := 0
	for _, ok := range results {
		if ok {
			allowed++
		}
	}
	if allowed != limit {
		t.Errorf("concurrent callers were allowed %d times, want exactly %d", allowed, limit)
	}
}

func TestPostgresLimiter_CleanupRemovesElapsedWindows(t *testing.T) {
	db := openTestDB(t)
	limiter := ratelimit.NewPostgresLimiter(db, nil)

	key := fmt.Sprintf("test-cleanup-%d", time.Now().UnixNano())
	old := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	if _, err := db.Exec(
		`INSERT INTO rate_limit_counters (bucket_key, window_start, hits) VALUES ($1, $2, 7)`,
		key, old); err != nil {
		t.Fatalf("seeding an elapsed window: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM rate_limit_counters WHERE bucket_key = $1`, key) })

	if _, err := limiter.CleanupStale(context.Background()); err != nil {
		t.Fatalf("CleanupStale: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rate_limit_counters WHERE bucket_key = $1`, key).Scan(&count); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 0 {
		t.Errorf("elapsed window survived the sweep (%d rows)", count)
	}
}

// A database failure must degrade to per-replica limiting, never to no limiting
// and never to rejecting every request.
func TestPostgresLimiter_FallsBackWhenDatabaseFails(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/nonexistent?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	limiter := ratelimit.NewPostgresLimiter(db, nil)

	const limit = 3
	allowed := 0
	for i := 0; i < limit*2; i++ {
		if limiter.Allow("fallback-key", limit).Allowed {
			allowed++
		}
	}

	if allowed == 0 {
		t.Error("a database outage must not reject every request")
	}
	if allowed > limit {
		t.Errorf("the in-memory fallback must still enforce the limit, allowed %d of %d", allowed, limit*2)
	}
}

func TestPostgresLimiter_NilReceiverAndNilDB(t *testing.T) {
	var nilLimiter *ratelimit.PostgresLimiter
	if res := nilLimiter.Allow("k", 5); !res.Allowed {
		t.Error("a nil limiter must not block traffic")
	}
	if rows, err := nilLimiter.CleanupStale(context.Background()); err != nil || rows != 0 {
		t.Errorf("CleanupStale on a nil limiter: rows=%d err=%v", rows, err)
	}
}
