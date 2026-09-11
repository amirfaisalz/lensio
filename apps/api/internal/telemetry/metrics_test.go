package telemetry

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMetrics_RecordAndScrape(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry to ensure metric instruments are registered
	tel, err := Init(ctx, Config{
		ServiceName: "lensio-metrics-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() { _ = tel.Shutdown(ctx) }()

	// 1. Record HTTP requests
	RecordHTTPRequest(ctx, "/api/v1/ocr/ktp", "POST", http.StatusOK, 0.35)
	RecordHTTPRequest(ctx, "/api/v1/ocr/ktp", "POST", http.StatusBadRequest, 0.05)
	RecordHTTPRequest(ctx, "", "", http.StatusOK, 0.01) // test defaults

	// 2. Record HTTP errors
	RecordHTTPError(ctx, "/api/v1/ocr/ktp", http.StatusBadRequest, "invalid_document")
	RecordHTTPError(ctx, "", 0, "") // test defaults

	// 3. Record Rate Limit violations
	RecordRateLimitExceeded(ctx, "org_test_123", "starter")
	RecordRateLimitExceeded(ctx, "", "") // test defaults

	// 4. Record OCR requests and pipeline stages
	RecordOCRRequest(ctx, "ktp", "completed", 0.95, 0.35)
	RecordOCRRequest(ctx, "ktp", "low_confidence", 0.65, 0.40)
	RecordOCRRequest(ctx, "", "", 0.0, 0.0) // test defaults

	RecordOCRStageDuration(ctx, "validation", "success", 0.015)
	RecordOCRStageDuration(ctx, "ocr_engine", "success", 0.28)
	RecordOCRStageDuration(ctx, "field_extraction", "success", 0.05)
	RecordOCRStageDuration(ctx, "", "", 0.0) // test defaults

	RecordOCRError(ctx, "validation", "invalid_document")
	RecordOCRError(ctx, "", "") // test defaults

	// 5. Test RegisterDBStats with nil and empty DB
	if err := RegisterDBStats(nil); err != nil {
		t.Errorf("RegisterDBStats(nil) returned error: %v", err)
	}

	// 6. Scrape /metrics endpoint and verify expected Prometheus metrics are present
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	PrometheusHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /metrics, got %d", rec.Code)
	}

	body := rec.Body.String()

	expectedMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_errors_total",
		"rate_limit_exceeded_total",
		"ocr_requests_total",
		"ocr_duration_seconds",
		"ocr_errors_total",
		"ocr_confidence_score",
	}

	for _, metricName := range expectedMetrics {
		if !strings.Contains(body, metricName) {
			t.Errorf("expected /metrics output to contain %q, body:\n%s", metricName, body)
		}
	}
}

func TestRegisterDBStats_WithDB(t *testing.T) {
	ctx := context.Background()
	_, _ = Init(ctx, Config{ServiceName: "test-db-stats"})

	db, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	// Reset dbStatsRegistered for unit test
	metricsMu.Lock()
	dbStatsRegistered = false
	metricsMu.Unlock()

	if err := RegisterDBStats(db); err != nil {
		t.Fatalf("RegisterDBStats failed: %v", err)
	}

	// Scrape metrics to execute gauge callbacks
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	PrometheusHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /metrics, got %d", rec.Code)
	}

	// Registering again should be an idempotent no-op
	if err := RegisterDBStats(db); err != nil {
		t.Errorf("duplicate RegisterDBStats returned error: %v", err)
	}
}

func TestMetrics_NilInstruments(t *testing.T) {
	metricsMu.Lock()
	savedHTTP := httpErrorsTotal
	savedRL := rateLimitExceededTotal
	savedOCRErr := ocrErrorsTotal
	savedOCRDuration := ocrDurationSeconds
	httpErrorsTotal = nil
	rateLimitExceededTotal = nil
	ocrErrorsTotal = nil
	ocrDurationSeconds = nil
	metricsMu.Unlock()

	defer func() {
		metricsMu.Lock()
		httpErrorsTotal = savedHTTP
		rateLimitExceededTotal = savedRL
		ocrErrorsTotal = savedOCRErr
		ocrDurationSeconds = savedOCRDuration
		metricsMu.Unlock()
	}()

	ctx := context.Background()
	RecordHTTPError(ctx, "/route", 500, "err")
	RecordRateLimitExceeded(ctx, "org", "free")
	RecordOCRStageDuration(ctx, "stage", "err", 0.1)
	RecordOCRError(ctx, "stage", "err")
}

func BenchmarkMetricsRecording(b *testing.B) {
	ctx := context.Background()
	_, _ = Init(ctx, Config{ServiceName: "bench-test"})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RecordHTTPRequest(ctx, "/api/v1/ocr/ktp", "POST", 200, 0.12)
		RecordOCRStageDuration(ctx, "ocr_engine", "success", 0.08)
	}
}
