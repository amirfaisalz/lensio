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

func TestCreateOCRRequest_Nil(t *testing.T) {
	var db store.DB
	err := db.CreateOCRRequest(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when inserting nil ocr request, got nil")
	}
}

func TestOCRRequestStore_LiveDB(t *testing.T) {
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
		t.Fatalf("failed to run migrations: %v", err)
	}

	testOrg := createTestOrg(t, db)
	testOrgID := testOrg.ID
	reqID := fmt.Sprintf("req_test_%d", time.Now().UnixNano())

	req := &store.OCRRequest{
		ID:         reqID,
		OrgID:      testOrgID,
		Status:     "completed",
		Confidence: 0.98,
		LatencyMS:  125,
		DocType:    "ktp",
	}

	if err := db.CreateOCRRequest(ctx, req); err != nil {
		t.Fatalf("failed creating ocr request: %v", err)
	}
	if req.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at after insert")
	}

	// Retrieve by ID
	fetched, err := db.GetOCRRequestByID(ctx, testOrgID, reqID)
	if err != nil {
		t.Fatalf("failed fetching ocr request by id: %v", err)
	}
	if fetched.ID != reqID || fetched.OrgID != testOrgID || fetched.Status != "completed" {
		t.Errorf("fetched metadata mismatch: %+v", fetched)
	}
	if fetched.Confidence < 0.97 || fetched.Confidence > 0.99 {
		t.Errorf("expected confidence ~0.98, got %f", fetched.Confidence)
	}

	// Tenant isolation check: another org ID cannot access this record
	wrongOrgID := "00000000-0000-0000-0000-000000000002"
	_, err = db.GetOCRRequestByID(ctx, wrongOrgID, reqID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong org_id, got %v", err)
	}

	// Non-existent ID check
	_, err = db.GetOCRRequestByID(ctx, testOrgID, "non_existent_req_id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing id, got %v", err)
	}
}
