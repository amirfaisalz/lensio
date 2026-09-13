package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestUsageStore_ValidationErrors(t *testing.T) {
	var db store.DB
	ctx := context.Background()

	if err := db.CreateUsageRecord(ctx, nil); err == nil {
		t.Fatal("expected error when inserting nil usage record")
	}

	if _, err := db.GetUsageSummary(ctx, "", time.Now(), 100, time.Now()); err == nil {
		t.Fatal("expected error for empty orgID in GetUsageSummary")
	}

	if _, err := db.GetDailyUsage(ctx, "", time.Now()); err == nil {
		t.Fatal("expected error for empty orgID in GetDailyUsage")
	}

	if _, err := db.GetEndpointUsage(ctx, "", time.Now()); err == nil {
		t.Fatal("expected error for empty orgID in GetEndpointUsage")
	}

	if _, err := db.GetMonthlyOCRCount(ctx, "", time.Now()); err == nil {
		t.Fatal("expected error for empty orgID in GetMonthlyOCRCount")
	}

	if _, _, err := db.GetUsageRecords(ctx, "", store.UsageRecordFilter{}); err == nil {
		t.Fatal("expected error for empty orgID in GetUsageRecords")
	}
}

func TestUsageStore_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
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

	testOrg := createTestOrg(t, db)
	defaultOrgID := testOrg.ID

	since := time.Now().Add(-1 * time.Hour)
	cycleReset := time.Now().Add(24 * time.Hour)

	// 1. Insert 2 usage records: one success, one error
	rec1 := &store.UsageRecord{
		OrgID:      defaultOrgID,
		RequestID:  fmt.Sprintf("req_test_%d_1", time.Now().UnixNano()),
		Endpoint:   "/api/v1/ocr/ktp",
		StatusCode: 200,
		LatencyMS:  150,
		Timestamp:  time.Now(),
	}
	if err := db.CreateUsageRecord(ctx, rec1); err != nil {
		t.Fatalf("failed creating usage record 1: %v", err)
	}

	rec2 := &store.UsageRecord{
		OrgID:      defaultOrgID,
		RequestID:  fmt.Sprintf("req_test_%d_2", time.Now().UnixNano()),
		Endpoint:   "/api/v1/ocr/ktp",
		StatusCode: 429,
		LatencyMS:  5,
		Timestamp:  time.Now(),
	}
	if err := db.CreateUsageRecord(ctx, rec2); err != nil {
		t.Fatalf("failed creating usage record 2: %v", err)
	}

	rec3 := &store.UsageRecord{
		OrgID:      defaultOrgID,
		RequestID:  fmt.Sprintf("req_test_%d_3", time.Now().UnixNano()),
		Endpoint:   "/api/v1/account",
		StatusCode: 200,
		LatencyMS:  40,
		Timestamp:  time.Now(),
	}
	if err := db.CreateUsageRecord(ctx, rec3); err != nil {
		t.Fatalf("failed creating usage record 3: %v", err)
	}

	// 2. Query Usage Summary
	summary, err := db.GetUsageSummary(ctx, defaultOrgID, since, 100, cycleReset)
	if err != nil {
		t.Fatalf("failed querying usage summary: %v", err)
	}
	if summary.TotalRequests < 2 {
		t.Errorf("expected at least 2 total requests, got %d", summary.TotalRequests)
	}
	if summary.SuccessCount < 1 {
		t.Errorf("expected at least 1 success count, got %d", summary.SuccessCount)
	}
	if summary.ErrorCount < 1 {
		t.Errorf("expected at least 1 error count, got %d", summary.ErrorCount)
	}
	if summary.QuotaLimit != 100 {
		t.Errorf("expected quota limit 100, got %d", summary.QuotaLimit)
	}
	if summary.QuotaRemaining != 99 {
		t.Errorf("expected quota remaining 99, got %d", summary.QuotaRemaining)
	}
	if summary.P95LatencyMS != 40 {
		t.Errorf("expected platform P95 40ms (non-OCR only), got %d", summary.P95LatencyMS)
	}
	if summary.P95OCRLatencyMS != 143 {
		t.Errorf("expected OCR P95 143ms (percentile of {5,150}, PG ::int rounds), got %d", summary.P95OCRLatencyMS)
	}

	// 3. Query Daily Usage
	daily, err := db.GetDailyUsage(ctx, defaultOrgID, since)
	if err != nil {
		t.Fatalf("failed querying daily usage: %v", err)
	}
	if len(daily) == 0 {
		t.Errorf("expected non-empty daily usage results")
	}

	// 4. Query Endpoint Usage
	endpoints, err := db.GetEndpointUsage(ctx, defaultOrgID, since)
	if err != nil {
		t.Fatalf("failed querying endpoint usage: %v", err)
	}
	if len(endpoints) == 0 {
		t.Errorf("expected non-empty endpoint usage results")
	}

	// 5. Query Monthly OCR Count (only status_code < 400 counts towards quota)
	ocrCount, err := db.GetMonthlyOCRCount(ctx, defaultOrgID, since)
	if err != nil {
		t.Fatalf("failed querying monthly ocr count: %v", err)
	}
	if ocrCount != 1 {
		t.Errorf("expected exactly 1 successful ocr request (status_code < 400), got %d", ocrCount)
	}

	// 6. Query Usage Records (paginated)
	records, total, err := db.GetUsageRecords(ctx, defaultOrgID, store.UsageRecordFilter{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("failed querying usage records: %v", err)
	}
	if total < 2 {
		t.Errorf("expected at least 2 records total, got %d", total)
	}
	if len(records) == 0 {
		t.Errorf("expected non-empty records slice")
	}

	// 7. Query with filter
	filtered, totalFiltered, err := db.GetUsageRecords(ctx, defaultOrgID, store.UsageRecordFilter{StatusCode: 429, Endpoint: "/api/v1/ocr/ktp"})
	if err != nil {
		t.Fatalf("failed querying filtered usage records: %v", err)
	}
	if totalFiltered < 1 || len(filtered) < 1 {
		t.Errorf("expected at least 1 filtered record, got total %d, count %d", totalFiltered, len(filtered))
	}
}

