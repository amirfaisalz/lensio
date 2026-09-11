package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestPIISanitizer_SensitiveKeysRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	ctx := context.Background()

	logger.InfoContext(ctx, "test event",
		slog.String("nik", "3171012345670001"),
		slog.String("nama", "Budi Santoso"),
		slog.String("full_name", "Jane Doe"),
		slog.String("alamat", "Jl. Sudirman No. 12"),
		slog.String("tanggal_lahir", "1990-05-15"),
		slog.String("token", "lensio_live_secret123"),
		slog.String("password", "supersecret"),
		slog.String("key_hash", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
		slog.String("safe_field", "safe_value"),
	)

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v, raw: %s", err, buf.String())
	}

	sensitiveCheckKeys := []string{"nik", "nama", "full_name", "alamat", "tanggal_lahir", "token", "password", "key_hash"}
	for _, key := range sensitiveCheckKeys {
		val, exists := logMap[key]
		if !exists {
			t.Errorf("expected key %q to exist in log output", key)
			continue
		}
		if val != RedactedValue {
			t.Errorf("expected key %q to be redacted as %q, got %q", key, RedactedValue, val)
		}
	}

	if logMap["safe_field"] != "safe_value" {
		t.Errorf("expected safe_field to remain untouched, got %v", logMap["safe_field"])
	}
}

func TestPIISanitizer_MessageNIKMasking(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	ctx := context.Background()
	logger.InfoContext(ctx, "processing citizen record with NIK 3171012345670001 successfully")

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	msg, ok := logMap["msg"].(string)
	if !ok {
		t.Fatalf("msg is not a string: %v", logMap["msg"])
	}

	if strings.Contains(msg, "3171012345670001") {
		t.Errorf("raw NIK leaked in message: %s", msg)
	}
	if !strings.Contains(msg, "3171************") {
		t.Errorf("expected masked NIK in message, got: %s", msg)
	}
}

func TestPIISanitizer_ValueNIKMasking(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	ctx := context.Background()
	logger.InfoContext(ctx, "event",
		slog.String("query_summary", "matched citizen id: 3171098765432109 in registry"),
	)

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	summary := logMap["query_summary"].(string)
	if strings.Contains(summary, "3171098765432109") {
		t.Errorf("raw NIK leaked in attribute value: %s", summary)
	}
	if !strings.Contains(summary, "3171************") {
		t.Errorf("expected masked NIK in attribute value, got: %s", summary)
	}
}

func TestPIISanitizer_ImageDataRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	ctx := context.Background()
	logger.InfoContext(ctx, "upload event",
		slog.String("data_uri", "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEASABIAAD..."),
		slog.String("raw_bytes", "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA="),
	)

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	if logMap["data_uri"] != RedactedImageData {
		t.Errorf("expected data_uri to be %q, got %v", RedactedImageData, logMap["data_uri"])
	}
	if logMap["raw_bytes"] != RedactedImageData {
		t.Errorf("expected raw_bytes to be %q, got %v", RedactedImageData, logMap["raw_bytes"])
	}
}

func TestPIISanitizer_NestedGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	ctx := context.Background()
	logger.InfoContext(ctx, "grouped event",
		slog.Group("identity",
			slog.String("nama", "Alice"),
			slog.String("nik", "3201012345670002"),
			slog.String("public_id", "pub_999"),
		),
	)

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	ident, ok := logMap["identity"].(map[string]any)
	if !ok {
		t.Fatalf("expected identity group map, got: %v", logMap["identity"])
	}

	if ident["nama"] != RedactedValue {
		t.Errorf("expected ident.nama to be redacted, got: %v", ident["nama"])
	}
	if ident["nik"] != RedactedValue {
		t.Errorf("expected ident.nik to be redacted, got: %v", ident["nik"])
	}
	if ident["public_id"] != "pub_999" {
		t.Errorf("expected ident.public_id to remain pub_999, got: %v", ident["public_id"])
	}
}

func TestPIISanitizer_ContextCorrelation(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	// 1. Create a real OpenTelemetry tracer and start a span
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	// 2. Inject Request ID into context
	reqID := "req_01JABCDEF1234567890"
	ctx = response.WithRequestID(ctx, reqID)

	// 3. Log with context
	logger.InfoContext(ctx, "operation finished", slog.Int("count", 42))

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	// Verify trace_id and span_id match OpenTelemetry span context
	spanCtx := span.SpanContext()
	if logMap["trace_id"] != spanCtx.TraceID().String() {
		t.Errorf("expected trace_id %s, got %v", spanCtx.TraceID().String(), logMap["trace_id"])
	}
	if logMap["span_id"] != spanCtx.SpanID().String() {
		t.Errorf("expected span_id %s, got %v", spanCtx.SpanID().String(), logMap["span_id"])
	}

	// Verify request_id
	if logMap["request_id"] != reqID {
		t.Errorf("expected request_id %s, got %v", reqID, logMap["request_id"])
	}
}

func TestPIISanitizer_WithAttrsAndWithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	baseLogger := InitLogger(slog.LevelInfo, buf)

	// Use WithAttrs with sensitive key
	subLogger := baseLogger.With(slog.String("nama", "Secret Person"), slog.String("tenant", "org_1"))

	// Use WithGroup
	groupLogger := subLogger.WithGroup("audit")
	groupLogger.Info("action performed", slog.String("action", "login"))

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	if logMap["nama"] != RedactedValue {
		t.Errorf("expected WithAttrs nama to be redacted, got: %v", logMap["nama"])
	}
	if logMap["tenant"] != "org_1" {
		t.Errorf("expected tenant to be org_1, got: %v", logMap["tenant"])
	}

	audit, ok := logMap["audit"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit group, got: %v", logMap["audit"])
	}
	if audit["action"] != "login" {
		t.Errorf("expected audit.action to be login, got: %v", audit["action"])
	}
}

func TestPIISanitizer_EdgeCases(t *testing.T) {
	// 1. Default output (nil out defaults to os.Stdout)
	defaultLogger := InitLogger(slog.LevelInfo, nil)
	if defaultLogger == nil {
		t.Fatal("expected non-nil defaultLogger")
	}

	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)

	// 2. Existing request_id in attributes should not be duplicated
	ctx := response.WithRequestID(context.Background(), "req_ctx_123")
	logger.InfoContext(ctx, "event with explicit req id",
		slog.String("request_id", "req_explicit_456"),
		slog.Int("numeric_attr", 12345),
		slog.Bool("boolean_attr", true),
	)

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed parsing log JSON: %v", err)
	}

	if logMap["request_id"] != "req_explicit_456" {
		t.Errorf("expected explicit request_id to be preserved, got %v", logMap["request_id"])
	}

	// 3. Test Enabled method
	if logger.Handler().Enabled(ctx, slog.LevelDebug) {
		t.Error("expected LevelDebug to be disabled for LevelInfo logger")
	}
	if !logger.Handler().Enabled(ctx, slog.LevelInfo) {
		t.Error("expected LevelInfo to be enabled")
	}
}

func BenchmarkPIISanitizer(b *testing.B) {
	buf := &bytes.Buffer{}
	logger := InitLogger(slog.LevelInfo, buf)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		logger.InfoContext(ctx, "processing citizen record NIK 3171012345670001",
			slog.String("nik", "3171012345670001"),
			slog.String("nama", "Budi Santoso"),
			slog.String("status", "ok"),
		)
	}
}
