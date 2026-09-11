package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	metricsMu sync.RWMutex

	// Availability & HTTP Metrics
	httpRequestsTotal   metric.Int64Counter
	httpRequestDuration metric.Float64Histogram
	httpErrorsTotal     metric.Int64Counter

	// Rate Limiting Metrics
	rateLimitExceededTotal metric.Int64Counter

	// OCR Pipeline & Performance Metrics
	ocrRequestsTotal   metric.Int64Counter
	ocrDurationSeconds metric.Float64Histogram
	ocrErrorsTotal     metric.Int64Counter
	ocrConfidenceScore metric.Float64Histogram

	// Database Connection Pool Registration
	dbStatsRegistered bool
)

// initInstruments creates all Prometheus / OpenTelemetry metric instruments.
func initInstruments(meter metric.Meter) error {
	var err error

	// 1. Availability: HTTP requests total
	httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests processed"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("creating http_requests_total: %w", err)
	}

	// 2. Performance: HTTP request latency histogram
	httpRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request execution latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0),
	)
	if err != nil {
		return fmt.Errorf("creating http_request_duration_seconds: %w", err)
	}

	// 3. Errors: HTTP error responses counter
	httpErrorsTotal, err = meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP error responses returned"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("creating http_errors_total: %w", err)
	}

	// 4. Rate Limiting: Rate limit exceeded counter
	rateLimitExceededTotal, err = meter.Int64Counter(
		"rate_limit_exceeded_total",
		metric.WithDescription("Total number of HTTP 429 rate limit exceeded events"),
		metric.WithUnit("{rejection}"),
	)
	if err != nil {
		return fmt.Errorf("creating rate_limit_exceeded_total: %w", err)
	}

	// 5. Business Metrics: OCR requests total
	ocrRequestsTotal, err = meter.Int64Counter(
		"ocr_requests_total",
		metric.WithDescription("Total number of OCR extraction attempts"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("creating ocr_requests_total: %w", err)
	}

	// 6. OCR Pipeline Latency histogram
	ocrDurationSeconds, err = meter.Float64Histogram(
		"ocr_duration_seconds",
		metric.WithDescription("OCR processing pipeline latency in seconds by stage"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.05, 0.1, 0.25, 0.5, 1.0, 1.5, 2.0, 3.0, 5.0, 10.0),
	)
	if err != nil {
		return fmt.Errorf("creating ocr_duration_seconds: %w", err)
	}

	// 7. OCR Errors counter
	ocrErrorsTotal, err = meter.Int64Counter(
		"ocr_errors_total",
		metric.WithDescription("Total number of OCR errors categorized by stage and error code"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("creating ocr_errors_total: %w", err)
	}

	// 8. OCR Confidence Score histogram
	ocrConfidenceScore, err = meter.Float64Histogram(
		"ocr_confidence_score",
		metric.WithDescription("Extraction confidence score distribution (0.0 to 1.0)"),
		metric.WithUnit("1"),
		metric.WithExplicitBucketBoundaries(0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.85, 0.9, 0.95, 1.0),
	)
	if err != nil {
		return fmt.Errorf("creating ocr_confidence_score: %w", err)
	}

	return nil
}

// RecordHTTPRequest records an HTTP request completion including latency and errors.
func RecordHTTPRequest(ctx context.Context, route, method string, statusCode int, durationSec float64) {
	if route == "" {
		route = "unknown"
	}
	if method == "" {
		method = "GET"
	}

	statusStr := strconv.Itoa(statusCode)
	attrs := []attribute.KeyValue{
		attribute.String("route", route),
		attribute.String("method", method),
		attribute.String("status_code", statusStr),
	}

	if httpRequestsTotal != nil {
		httpRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	if httpRequestDuration != nil {
		httpRequestDuration.Record(ctx, durationSec, metric.WithAttributes(attrs...))
	}
}

// RecordHTTPError records an HTTP client or server error response.
func RecordHTTPError(ctx context.Context, route string, statusCode int, errorCode string) {
	if httpErrorsTotal == nil {
		return
	}
	if route == "" {
		route = "unknown"
	}
	if errorCode == "" {
		errorCode = "unknown"
	}

	attrs := []attribute.KeyValue{
		attribute.String("route", route),
		attribute.String("status_code", strconv.Itoa(statusCode)),
		attribute.String("error_code", errorCode),
	}
	httpErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRateLimitExceeded records an HTTP 429 rate limit violation.
func RecordRateLimitExceeded(ctx context.Context, orgID, plan string) {
	if rateLimitExceededTotal == nil {
		return
	}
	if orgID == "" {
		orgID = "unknown"
	}
	if plan == "" {
		plan = "free"
	}

	attrs := []attribute.KeyValue{
		attribute.String("org_id", orgID),
		attribute.String("plan", plan),
	}
	rateLimitExceededTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordOCRRequest records a completed or failed OCR request and its confidence score.
func RecordOCRRequest(ctx context.Context, docType, status string, confidence float64, durationSec float64) {
	if docType == "" {
		docType = "ktp"
	}
	if status == "" {
		status = "completed"
	}

	attrs := []attribute.KeyValue{
		attribute.String("doc_type", docType),
		attribute.String("status", status),
	}

	if ocrRequestsTotal != nil {
		ocrRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	if confidence > 0 && ocrConfidenceScore != nil {
		ocrConfidenceScore.Record(ctx, confidence, metric.WithAttributes(attribute.String("doc_type", docType)))
	}

	if durationSec > 0 && ocrDurationSeconds != nil {
		ocrDurationSeconds.Record(ctx, durationSec, metric.WithAttributes(
			attribute.String("stage", "overall"),
			attribute.String("status", status),
		))
	}
}

// RecordOCRStageDuration records execution latency for an individual OCR pipeline stage.
func RecordOCRStageDuration(ctx context.Context, stage, status string, durationSec float64) {
	if ocrDurationSeconds == nil {
		return
	}
	if stage == "" {
		stage = "unknown"
	}
	if status == "" {
		status = "success"
	}

	attrs := []attribute.KeyValue{
		attribute.String("stage", stage),
		attribute.String("status", status),
	}
	ocrDurationSeconds.Record(ctx, durationSec, metric.WithAttributes(attrs...))
}

// RecordOCRError records an error encountered within an OCR pipeline stage.
func RecordOCRError(ctx context.Context, stage, errorCode string) {
	if ocrErrorsTotal == nil {
		return
	}
	if stage == "" {
		stage = "unknown"
	}
	if errorCode == "" {
		errorCode = "unknown"
	}

	attrs := []attribute.KeyValue{
		attribute.String("stage", stage),
		attribute.String("error_code", errorCode),
	}
	ocrErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RegisterDBStats registers observable gauge metrics for PostgreSQL connection pool status.
func RegisterDBStats(db *sql.DB) error {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	if db == nil || dbStatsRegistered {
		return nil
	}

	meter := Meter()

	// 1. DB open connections
	_, err := meter.Int64ObservableGauge(
		"db_pool_connections_open",
		metric.WithDescription("The number of established connections both in use and idle"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := db.Stats()
			o.Observe(int64(stats.OpenConnections))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("registering db_pool_connections_open: %w", err)
	}

	// 2. DB in-use connections
	_, err = meter.Int64ObservableGauge(
		"db_pool_connections_in_use",
		metric.WithDescription("The number of connections currently in use"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := db.Stats()
			o.Observe(int64(stats.InUse))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("registering db_pool_connections_in_use: %w", err)
	}

	// 3. DB idle connections
	_, err = meter.Int64ObservableGauge(
		"db_pool_connections_idle",
		metric.WithDescription("The number of idle connections"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := db.Stats()
			o.Observe(int64(stats.Idle))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("registering db_pool_connections_idle: %w", err)
	}

	// 4. DB wait count
	_, err = meter.Int64ObservableGauge(
		"db_pool_connections_wait_count",
		metric.WithDescription("The total number of connections waited for"),
		metric.WithUnit("{wait}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := db.Stats()
			o.Observe(stats.WaitCount)
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("registering db_pool_connections_wait_count: %w", err)
	}

	dbStatsRegistered = true
	return nil
}
