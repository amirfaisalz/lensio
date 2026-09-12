package store_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestStore_ErrorBranchesAndEdgeCases(t *testing.T) {
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

	defaultOrgID := "00000000-0000-0000-0000-000000000001"
	missingOrgID := "00000000-0000-0000-0000-999999999999"

	canceledCtx, cancelCtx := context.WithCancel(context.Background())
	cancelCtx() // immediately canceled

	t.Run("account store edge cases and query errors", func(t *testing.T) {
		// Nonexistent org in GetOrganization
		_, err := db.GetOrganization(ctx, missingOrgID)
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("expected ErrNotFound for missing org, got %v", err)
		}

		// Canceled context for GetOrganization
		_, err = db.GetOrganization(canceledCtx, defaultOrgID)
		if err == nil {
			t.Error("expected error for canceled context in GetOrganization")
		}

		// Nonexistent org in GetOrganizationPlan falls back to 'free' plan
		plan, err := db.GetOrganizationPlan(ctx, missingOrgID)
		if err != nil {
			t.Errorf("expected fallback to free plan for missing org, got err: %v", err)
		}
		if plan == nil || plan.Code != "free" {
			t.Errorf("expected free plan, got %v", plan)
		}

		// Canceled context for GetOrganizationPlan
		_, err = db.GetOrganizationPlan(canceledCtx, defaultOrgID)
		if err == nil {
			t.Error("expected error for canceled context in GetOrganizationPlan")
		}

		// Canceled context for UpdateOrganizationPlan
		err = db.UpdateOrganizationPlan(canceledCtx, defaultOrgID, "pro")
		if err == nil {
			t.Error("expected error for canceled context in UpdateOrganizationPlan")
		}

		// Invalid UUID in UpdateOrganizationPlan triggers ExecContext error
		err = db.UpdateOrganizationPlan(ctx, "not-a-valid-uuid", "free")
		if err == nil {
			t.Error("expected error for invalid UUID in UpdateOrganizationPlan")
		}

		// Missing org in GetOrganizationMembers returns empty slice
		members, err := db.GetOrganizationMembers(ctx, missingOrgID)
		if err != nil {
			t.Errorf("unexpected error for missing org members: %v", err)
		}
		if len(members) != 0 {
			t.Errorf("expected 0 members, got %d", len(members))
		}

		// Canceled context for GetOrganizationMembers
		_, err = db.GetOrganizationMembers(canceledCtx, defaultOrgID)
		if err == nil {
			t.Error("expected error for canceled context in GetOrganizationMembers")
		}
	})

	t.Run("api keys edge cases and query errors", func(t *testing.T) {
		// Create with canceled context
		k := &store.APIKey{
			OrgID:       defaultOrgID,
			Name:        "Canceled Key",
			KeyHash:     "somehash",
			Prefix:      "ls_live_",
			Scopes:      []string{"ocr:read"},
			Environment: "live",
		}
		err := db.CreateAPIKey(canceledCtx, k)
		if err == nil {
			t.Error("expected error for canceled context in CreateAPIKey")
		}

		// Query by hash with canceled context
		_, err = db.GetAPIKeyByHash(canceledCtx, "somehash")
		if err == nil {
			t.Error("expected error for canceled context in GetAPIKeyByHash")
		}

		// Test revokedAt branch in GetAPIKeyByHash and ListAPIKeysByOrg
		revokedKey := &store.APIKey{
			OrgID:       defaultOrgID,
			Name:        "Revoked Key For Test",
			KeyHash:     "revoked_test_hash_unique_12345",
			Prefix:      "ls_live_",
			Scopes:      []string{"ocr:read"},
			Environment: "live",
		}
		if err := db.CreateAPIKey(ctx, revokedKey); err == nil {
			_ = db.RevokeAPIKey(ctx, defaultOrgID, revokedKey.ID)
			revKey, err := db.GetAPIKeyByHash(ctx, "revoked_test_hash_unique_12345")
			if err != nil || revKey.RevokedAt == nil {
				t.Errorf("expected revoked key with non-nil RevokedAt, got %v", revKey)
			}
			listedKeys, err := db.ListAPIKeysByOrg(ctx, defaultOrgID)
			if err != nil {
				t.Errorf("unexpected error listing keys: %v", err)
			}
			var foundRevoked bool
			for _, lk := range listedKeys {
				if lk.ID == revokedKey.ID && lk.RevokedAt != nil {
					foundRevoked = true
					break
				}
			}
			if !foundRevoked {
				t.Error("expected to find revoked key in listed keys")
			}
		}

		// List for empty org returns empty slice
		keys, err := db.ListAPIKeysByOrg(ctx, missingOrgID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("expected 0 keys, got %d", len(keys))
		}

		// List with canceled context
		_, err = db.ListAPIKeysByOrg(canceledCtx, defaultOrgID)
		if err == nil {
			t.Error("expected error for canceled context in ListAPIKeysByOrg")
		}

		// Revoke with canceled context
		err = db.RevokeAPIKey(canceledCtx, defaultOrgID, "some-id")
		if err == nil {
			t.Error("expected error for canceled context in RevokeAPIKey")
		}

		// Invalid UUID in RevokeAPIKey triggers ExecContext error
		err = db.RevokeAPIKey(ctx, defaultOrgID, "invalid-uuid")
		if err == nil {
			t.Error("expected error for invalid UUID in RevokeAPIKey")
		}

		// Touch with canceled context
		err = db.TouchAPIKeyLastUsed(canceledCtx, "some-id", time.Now())
		if err == nil {
			t.Error("expected error for canceled context in TouchAPIKeyLastUsed")
		}

		// Invalid UUID in TouchAPIKeyLastUsed triggers ExecContext error
		err = db.TouchAPIKeyLastUsed(ctx, "invalid-uuid", time.Now())
		if err == nil {
			t.Error("expected error for invalid UUID in TouchAPIKeyLastUsed")
		}

		// Malformed JSON scopes branch in GetAPIKeyByHash and ListAPIKeysByOrg
		_, _ = db.ExecContext(ctx, "INSERT INTO api_keys (org_id, name, key_hash, prefix, scopes, environment) VALUES ($1, $2, $3, $4, $5, $6)",
			defaultOrgID, "Bad Scopes Key", "bad_scopes_hash_for_unmarshal_test", "ls_live_", []byte(`"not-an-array"`), "live")
		badKey, err := db.GetAPIKeyByHash(ctx, "bad_scopes_hash_for_unmarshal_test")
		if err == nil && len(badKey.Scopes) != 0 {
			t.Errorf("expected empty scopes on unmarshal failure, got %v", badKey.Scopes)
		}
		// Also triggers unmarshal fallback in ListAPIKeysByOrg
		_, _ = db.ListAPIKeysByOrg(ctx, defaultOrgID)
	})

	t.Run("audit logs edge cases and query errors", func(t *testing.T) {
		// Record with bad metadata that fails json.Marshal
		badLog := &store.AuditLog{
			OrgID:    defaultOrgID,
			Action:   "test.action",
			Metadata: map[string]any{"bad": make(chan int)},
		}
		err := db.RecordAuditLog(ctx, badLog)
		if err == nil {
			t.Error("expected error for unmarshalable metadata in RecordAuditLog")
		}

		// Malformed JSON metadata fallback in ListAuditLogsByOrg
		_, _ = db.ExecContext(ctx, "INSERT INTO audit_logs (org_id, action, target_resource, metadata) VALUES ($1, $2, $3, $4)",
			defaultOrgID, "test.bad_meta", "test", []byte(`"not-an-object"`))
		_, _ = db.ListAuditLogsByOrg(ctx, defaultOrgID)

		// Record with canceled context
		validLog := &store.AuditLog{
			OrgID:  defaultOrgID,
			Action: "test.action",
		}
		err = db.RecordAuditLog(canceledCtx, validLog)
		if err == nil {
			t.Error("expected error for canceled context in RecordAuditLog")
		}

		// List for missing org returns empty slice
		logs, err := db.ListAuditLogsByOrg(ctx, missingOrgID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("expected 0 logs, got %d", len(logs))
		}

		// List with canceled context
		_, err = db.ListAuditLogsByOrg(canceledCtx, defaultOrgID)
		if err == nil {
			t.Error("expected error for canceled context in ListAuditLogsByOrg")
		}
	})

	t.Run("ocr requests edge cases and query errors", func(t *testing.T) {
		// Empty DocType defaults to 'ktp' and apiKeyID handling
		keys, _ := db.ListAPIKeysByOrg(ctx, defaultOrgID)
		var keyID *string
		if len(keys) > 0 {
			keyID = &keys[0].ID
		}

		req := &store.OCRRequest{
			ID:         "test-ocr-req-default-doctype",
			OrgID:      defaultOrgID,
			APIKeyID:   keyID,
			Status:     "completed",
			Confidence: 0.95,
			LatencyMS:  100,
			DocType:    "", // should default to ktp
		}
		err := db.CreateOCRRequest(ctx, req)
		if err != nil {
			t.Errorf("unexpected error creating ocr request with default doctype: %v", err)
		}
		if req.DocType != "ktp" {
			t.Errorf("expected DocType 'ktp', got %s", req.DocType)
		}

		// Get by ID verifies apiKeyID retrieval
		fetched, err := db.GetOCRRequestByID(ctx, defaultOrgID, req.ID)
		if err != nil {
			t.Errorf("unexpected error fetching request: %v", err)
		} else if keyID != nil && (fetched.APIKeyID == nil || *fetched.APIKeyID != *keyID) {
			t.Errorf("expected APIKeyID %s, got %v", *keyID, fetched.APIKeyID)
		}

		// Create with canceled context
		err = db.CreateOCRRequest(canceledCtx, req)
		if err == nil {
			t.Error("expected error for canceled context in CreateOCRRequest")
		}

		// Get with canceled context
		_, err = db.GetOCRRequestByID(canceledCtx, defaultOrgID, req.ID)
		if err == nil {
			t.Error("expected error for canceled context in GetOCRRequestByID")
		}
	})

	t.Run("usage store edge cases and query errors", func(t *testing.T) {
		// Create with canceled context
		rec := &store.UsageRecord{
			OrgID:      defaultOrgID,
			RequestID:  "req-cancel",
			Endpoint:   "/api/v1/ocr/ktp",
			StatusCode: 200,
		}
		err := db.CreateUsageRecord(canceledCtx, rec)
		if err == nil {
			t.Error("expected error for canceled context in CreateUsageRecord")
		}

		// GetUsageSummary with negative quota remaining
		summary, err := db.GetUsageSummary(ctx, defaultOrgID, time.Now().Add(-1*time.Hour), 0, time.Now())
		if err != nil {
			t.Errorf("unexpected error in GetUsageSummary: %v", err)
		}
		if summary.QuotaRemaining != 0 {
			t.Errorf("expected 0 quota remaining, got %d", summary.QuotaRemaining)
		}

		// GetUsageSummary with canceled context
		_, err = db.GetUsageSummary(canceledCtx, defaultOrgID, time.Now(), 100, time.Now())
		if err == nil {
			t.Error("expected error for canceled context in GetUsageSummary")
		}

		// GetDailyUsage for missing org
		daily, err := db.GetDailyUsage(ctx, missingOrgID, time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(daily) != 0 {
			t.Errorf("expected 0 daily usage items, got %d", len(daily))
		}

		// GetDailyUsage with canceled context
		_, err = db.GetDailyUsage(canceledCtx, defaultOrgID, time.Now())
		if err == nil {
			t.Error("expected error for canceled context in GetDailyUsage")
		}

		// GetEndpointUsage for missing org
		endpoints, err := db.GetEndpointUsage(ctx, missingOrgID, time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(endpoints) != 0 {
			t.Errorf("expected 0 endpoint items, got %d", len(endpoints))
		}

		// GetEndpointUsage with canceled context
		_, err = db.GetEndpointUsage(canceledCtx, defaultOrgID, time.Now())
		if err == nil {
			t.Error("expected error for canceled context in GetEndpointUsage")
		}

		// GetMonthlyOCRCount with canceled context
		_, err = db.GetMonthlyOCRCount(canceledCtx, defaultOrgID, time.Now())
		if err == nil {
			t.Error("expected error for canceled context in GetMonthlyOCRCount")
		}

		// GetUsageRecords with clamped limit and offset
		filter := store.UsageRecordFilter{
			Limit:  200, // clamped to 100
			Offset: -10, // clamped to 0
		}
		_, _, err = db.GetUsageRecords(ctx, defaultOrgID, filter)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// GetUsageRecords for missing org returns empty slice
		recs, total, err := db.GetUsageRecords(ctx, missingOrgID, store.UsageRecordFilter{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if total != 0 || len(recs) != 0 {
			t.Errorf("expected 0 records for missing org, got total %d, len %d", total, len(recs))
		}

		// GetUsageRecords with canceled context
		_, _, err = db.GetUsageRecords(canceledCtx, defaultOrgID, store.UsageRecordFilter{})
		if err == nil {
			t.Error("expected error for canceled context in GetUsageRecords")
		}

		// Select query failure in GetUsageRecords
		delayedUsageCtx := newDelayedCancelCtx(2)
		_, _, err = db.GetUsageRecords(delayedUsageCtx, defaultOrgID, store.UsageRecordFilter{})
		if err == nil {
			t.Error("expected error for select query failure in GetUsageRecords")
		}
	})

	t.Run("dirty schema migration error branches", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "UPDATE schema_migrations SET dirty = true")
		if err == nil {
			defer func() {
				_, _ = db.ExecContext(context.Background(), "UPDATE schema_migrations SET dirty = false")
			}()
			errUp := store.RunMigrationsUp(db.DB)
			if errUp == nil {
				t.Error("expected error from RunMigrationsUp on dirty schema")
			}
			errDown := store.RunMigrationsDown(db.DB)
			if errDown == nil {
				t.Error("expected error from RunMigrationsDown on dirty schema")
			}
		}
	})
}

type delayedCancelCtx struct {
	done  chan struct{}
	err   error
	mu    sync.Mutex
	calls int
	limit int
}

func newDelayedCancelCtx(limit int) *delayedCancelCtx {
	return &delayedCancelCtx{
		done:  make(chan struct{}),
		limit: limit,
	}
}

func (d *delayedCancelCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (d *delayedCancelCtx) Done() <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	if d.calls >= d.limit {
		select {
		case <-d.done:
		default:
			close(d.done)
			d.err = context.Canceled
		}
	}
	return d.done
}
func (d *delayedCancelCtx) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}
func (d *delayedCancelCtx) Value(key any) any { return nil }
