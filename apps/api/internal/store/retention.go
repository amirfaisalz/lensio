package store

import (
	"context"
	"fmt"
	"time"
)

// Retention windows published in PRIVACY.md. They lived only in that document
// until now: nothing in the codebase ever deleted anything, so "hapus/anonimkan
// maksimal 90 hari" was a promise with no implementation behind it.
const (
	// OCRRequestRetention bounds how long per-request OCR metadata is kept.
	// The rows hold no PII (id, org, confidence, latency, doc type, status),
	// but they are still a per-document activity trail.
	OCRRequestRetention = 90 * 24 * time.Hour

	// UsageRecordRetention bounds the per-request billing trail.
	UsageRecordRetention = 365 * 24 * time.Hour

	// AuditLogRetention bounds administrative audit entries.
	AuditLogRetention = 365 * 24 * time.Hour
)

// RetentionResult reports how many rows each sweep removed.
type RetentionResult struct {
	OCRRequests  int64
	UsageRecords int64
	AuditLogs    int64
}

// Total returns the number of rows deleted across all tables.
func (r RetentionResult) Total() int64 {
	return r.OCRRequests + r.UsageRecords + r.AuditLogs
}

// PurgeExpiredRecords deletes rows past their published retention window.
//
// Deletion is batched so a long-neglected table cannot hold a single lock long
// enough to stall live traffic; the sweep simply resumes on the next tick.
func (db *DB) PurgeExpiredRecords(ctx context.Context, now time.Time, batchSize int) (RetentionResult, error) {
	if batchSize <= 0 {
		batchSize = 5000
	}

	var res RetentionResult

	sweeps := []struct {
		name    string
		query   string
		cutoff  time.Time
		counter *int64
	}{
		{
			name: "ocr_requests",
			query: `DELETE FROM ocr_requests WHERE id IN (
				SELECT id FROM ocr_requests WHERE created_at < $1 LIMIT $2
			);`,
			cutoff:  now.Add(-OCRRequestRetention),
			counter: &res.OCRRequests,
		},
		{
			name: "usage_records",
			query: `DELETE FROM usage_records WHERE id IN (
				SELECT id FROM usage_records WHERE timestamp < $1 LIMIT $2
			);`,
			cutoff:  now.Add(-UsageRecordRetention),
			counter: &res.UsageRecords,
		},
		{
			name: "audit_logs",
			query: `DELETE FROM audit_logs WHERE id IN (
				SELECT id FROM audit_logs WHERE created_at < $1 LIMIT $2
			);`,
			cutoff:  now.Add(-AuditLogRetention),
			counter: &res.AuditLogs,
		},
	}

	for _, sweep := range sweeps {
		result, err := db.ExecContext(ctx, sweep.query, sweep.cutoff, batchSize)
		if err != nil {
			return res, fmt.Errorf("purging expired %s: %w", sweep.name, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return res, fmt.Errorf("counting purged %s: %w", sweep.name, err)
		}
		*sweep.counter = rows
	}

	return res, nil
}
