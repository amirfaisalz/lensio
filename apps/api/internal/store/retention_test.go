package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestPurgeExpiredRecords_LiveDB(t *testing.T) {
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
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	org := createTestOrg(t, db)
	now := time.Now()

	expiredID := fmt.Sprintf("ret-expired-%d", now.UnixNano())
	freshID := fmt.Sprintf("ret-fresh-%d", now.UnixNano())

	// One row comfortably past the 90-day window, one well inside it.
	for _, row := range []struct {
		id      string
		created time.Time
	}{
		{expiredID, now.Add(-store.OCRRequestRetention - 48*time.Hour)},
		{freshID, now.Add(-24 * time.Hour)},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO ocr_requests (id, org_id, status, confidence, latency_ms, doc_type, created_at)
			 VALUES ($1, $2, 'completed', 0.99, 120, 'ktp', $3);`,
			row.id, org.ID, row.created); err != nil {
			t.Fatalf("seeding %s: %v", row.id, err)
		}
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, `DELETE FROM ocr_requests WHERE id IN ($1, $2)`, expiredID, freshID)
	})

	res, err := db.PurgeExpiredRecords(ctx, now, 1000)
	if err != nil {
		t.Fatalf("PurgeExpiredRecords: %v", err)
	}
	if res.OCRRequests < 1 {
		t.Errorf("expected at least the seeded expired row to be purged, got %d", res.OCRRequests)
	}

	exists := func(id string) bool {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ocr_requests WHERE id = $1`, id).Scan(&count); err != nil {
			t.Fatalf("counting %s: %v", id, err)
		}
		return count > 0
	}

	if exists(expiredID) {
		t.Error("a record past the 90-day retention window survived the sweep")
	}
	if !exists(freshID) {
		t.Error("a record inside the retention window was deleted")
	}
}

func TestPurgeExpiredRecords_BatchSizeIsBounded(t *testing.T) {
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
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	org := createTestOrg(t, db)
	now := time.Now()
	prefix := fmt.Sprintf("ret-batch-%d", now.UnixNano())

	for i := 0; i < 5; i++ {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO ocr_requests (id, org_id, status, confidence, latency_ms, doc_type, created_at)
			 VALUES ($1, $2, 'completed', 0.5, 10, 'ktp', $3);`,
			fmt.Sprintf("%s-%d", prefix, i), org.ID, now.Add(-store.OCRRequestRetention-72*time.Hour)); err != nil {
			t.Fatalf("seeding row %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, `DELETE FROM ocr_requests WHERE id LIKE $1`, prefix+"%")
	})

	// A batch of 2 must stop at 2, so a neglected table cannot lock the row set
	// for an unbounded time.
	res, err := db.PurgeExpiredRecords(ctx, now, 2)
	if err != nil {
		t.Fatalf("PurgeExpiredRecords: %v", err)
	}
	if res.OCRRequests != 2 {
		t.Errorf("expected the sweep to honour the batch size of 2, deleted %d", res.OCRRequests)
	}
}
