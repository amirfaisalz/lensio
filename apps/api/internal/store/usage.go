package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// UsageRecord represents an individual API request execution metric record.
type UsageRecord struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	APIKeyID   *string   `json:"api_key_id,omitempty"`
	RequestID  string    `json:"request_id"`
	Endpoint   string    `json:"endpoint"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int       `json:"latency_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// UsageSummary aggregates total requests, success/error distribution, and quota consumption.
type UsageSummary struct {
	TotalRequests        int       `json:"total_requests"`
	SuccessCount         int       `json:"success_count"`
	ErrorCount           int       `json:"error_count"`
	QuotaLimit           int       `json:"quota_limit"`
	QuotaRemaining       int       `json:"quota_remaining"`
	P95LatencyMS         int       `json:"p95_latency_ms"`
	RateLimitViolations  int       `json:"rate_limit_violations"`
	BillingCycleReset    time.Time `json:"billing_cycle_reset"`
}

// DailyUsage represents aggregated request counts per calendar day.
type DailyUsage struct {
	Date          string `json:"date"`
	TotalRequests int    `json:"total_requests"`
	SuccessCount  int    `json:"success_count"`
	ErrorCount    int    `json:"error_count"`
}

// EndpointUsage represents usage breakdown and average latency per endpoint.
type EndpointUsage struct {
	Endpoint      string  `json:"endpoint"`
	TotalRequests int     `json:"total_requests"`
	AvgLatencyMS  float64 `json:"avg_latency_ms"`
}

// UsageRecordFilter specifies pagination and filtering criteria for request logs.
type UsageRecordFilter struct {
	Limit      int
	Offset     int
	StatusCode int
	Endpoint   string
}

// UsageStore specifies repository operations for persisting and querying API usage metrics.
type UsageStore interface {
	CreateUsageRecord(ctx context.Context, rec *UsageRecord) error
	GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*UsageSummary, error)
	GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]DailyUsage, error)
	GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]EndpointUsage, error)
	GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error)
	GetUsageRecords(ctx context.Context, orgID string, filter UsageRecordFilter) ([]UsageRecord, int, error)
}

// CreateUsageRecord inserts a new usage record into PostgreSQL.
func (db *DB) CreateUsageRecord(ctx context.Context, rec *UsageRecord) error {
	if rec == nil {
		return errors.New("usage record is nil")
	}

	query := `
		INSERT INTO usage_records (org_id, api_key_id, request_id, endpoint, status_code, latency_ms, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, '0001-01-01 00:00:00+00'::timestamptz), NOW()))
		RETURNING id, timestamp;
	`

	err := db.QueryRowContext(
		ctx,
		query,
		rec.OrgID,
		rec.APIKeyID,
		rec.RequestID,
		rec.Endpoint,
		rec.StatusCode,
		rec.LatencyMS,
		rec.Timestamp,
	).Scan(&rec.ID, &rec.Timestamp)
	if err != nil {
		return fmt.Errorf("inserting usage record: %w", err)
	}

	return nil
}

// GetUsageSummary aggregates total requests, success/error distribution, and quota remaining.
func (db *DB) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*UsageSummary, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT 
			COUNT(*),
			COALESCE(COUNT(*) FILTER (WHERE status_code < 400), 0),
			COALESCE(COUNT(*) FILTER (WHERE status_code >= 400), 0),
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0)::int,
			COALESCE(COUNT(*) FILTER (WHERE status_code = 429), 0)
		FROM usage_records
		WHERE org_id = $1 AND timestamp >= $2;
	`

	var (
		totalRequests       int
		successCount        int
		errorCount          int
		p95Latency          int
		rateLimitViolations int
	)

	err := db.QueryRowContext(ctx, query, orgID, since).Scan(
		&totalRequests,
		&successCount,
		&errorCount,
		&p95Latency,
		&rateLimitViolations,
	)
	if err != nil {
		return nil, fmt.Errorf("querying usage summary: %w", err)
	}

	quotaRemaining := planQuota - totalRequests
	if quotaRemaining < 0 {
		quotaRemaining = 0
	}

	return &UsageSummary{
		TotalRequests:       totalRequests,
		SuccessCount:        successCount,
		ErrorCount:          errorCount,
		QuotaLimit:          planQuota,
		QuotaRemaining:      quotaRemaining,
		P95LatencyMS:        p95Latency,
		RateLimitViolations: rateLimitViolations,
		BillingCycleReset:   cycleReset,
	}, nil
}

// GetDailyUsage queries timeseries metrics aggregated per calendar day.
func (db *DB) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]DailyUsage, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT 
			TO_CHAR(DATE(timestamp), 'YYYY-MM-DD') AS date_str,
			COUNT(*) AS total_requests,
			COALESCE(COUNT(*) FILTER (WHERE status_code < 400), 0) AS success_count,
			COALESCE(COUNT(*) FILTER (WHERE status_code >= 400), 0) AS error_count
		FROM usage_records
		WHERE org_id = $1 AND timestamp >= $2
		GROUP BY DATE(timestamp)
		ORDER BY DATE(timestamp) ASC;
	`

	rows, err := db.QueryContext(ctx, query, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("querying daily usage: %w", err)
	}
	defer rows.Close()

	var results []DailyUsage
	for rows.Next() {
		var d DailyUsage
		if err := rows.Scan(&d.Date, &d.TotalRequests, &d.SuccessCount, &d.ErrorCount); err != nil {
			return nil, fmt.Errorf("scanning daily usage row: %w", err)
		}
		results = append(results, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating daily usage rows: %w", err)
	}

	if results == nil {
		results = []DailyUsage{}
	}

	return results, nil
}

// GetEndpointUsage queries request count and average latency grouped by endpoint.
func (db *DB) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]EndpointUsage, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT 
			endpoint,
			COUNT(*) AS total_requests,
			COALESCE(ROUND(AVG(latency_ms)::numeric, 2), 0) AS avg_latency_ms
		FROM usage_records
		WHERE org_id = $1 AND timestamp >= $2
		GROUP BY endpoint
		ORDER BY total_requests DESC;
	`

	rows, err := db.QueryContext(ctx, query, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("querying endpoint usage: %w", err)
	}
	defer rows.Close()

	var results []EndpointUsage
	for rows.Next() {
		var ep EndpointUsage
		if err := rows.Scan(&ep.Endpoint, &ep.TotalRequests, &ep.AvgLatencyMS); err != nil {
			return nil, fmt.Errorf("scanning endpoint usage row: %w", err)
		}
		results = append(results, ep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating endpoint usage rows: %w", err)
	}

	if results == nil {
		results = []EndpointUsage{}
	}

	return results, nil
}

// GetMonthlyOCRCount counts total successful or attempted OCR requests during the current billing cycle.
func (db *DB) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	if orgID == "" {
		return 0, errors.New("orgID is required")
	}

	query := `
		SELECT COUNT(*)
		FROM usage_records
		WHERE org_id = $1 
		  AND timestamp >= $2 
		  AND endpoint = '/api/v1/ocr/ktp';
	`

	var count int
	err := db.QueryRowContext(ctx, query, orgID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("querying monthly ocr count: %w", err)
	}

	return count, nil
}

// GetUsageRecords retrieves paginated request execution logs for an organization.
func (db *DB) GetUsageRecords(ctx context.Context, orgID string, filter UsageRecordFilter) ([]UsageRecord, int, error) {
	if orgID == "" {
		return nil, 0, errors.New("orgID is required")
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	baseWhere := "WHERE org_id = $1"
	args := []any{orgID}
	argIdx := 2

	if filter.StatusCode > 0 {
		baseWhere += fmt.Sprintf(" AND status_code = $%d", argIdx)
		args = append(args, filter.StatusCode)
		argIdx++
	}
	if filter.Endpoint != "" {
		baseWhere += fmt.Sprintf(" AND endpoint = $%d", argIdx)
		args = append(args, filter.Endpoint)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM usage_records " + baseWhere
	var total int
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting usage records: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, org_id, api_key_id, COALESCE(request_id, ''), endpoint, status_code, latency_ms, timestamp
		FROM usage_records
		%s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d;
	`, baseWhere, argIdx, argIdx+1)

	queryArgs := append(args, filter.Limit, filter.Offset)
	rows, err := db.QueryContext(ctx, selectQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying usage records: %w", err)
	}
	defer rows.Close()

	var records []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.ID, &r.OrgID, &r.APIKeyID, &r.RequestID, &r.Endpoint, &r.StatusCode, &r.LatencyMS, &r.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("scanning usage record row: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating usage records: %w", err)
	}
	if records == nil {
		records = []UsageRecord{}
	}

	return records, total, nil
}

