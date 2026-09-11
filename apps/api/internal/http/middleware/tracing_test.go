package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestTracingMiddleware_NewRootSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(t.Context()) }()
	tracer := tp.Tracer("test-tracer")

	called := false
	handler := Tracing(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		span := trace.SpanFromContext(r.Context())
		if !span.SpanContext().IsValid() {
			t.Error("expected valid span in request context")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler was not called")
	}

	traceID := rec.Header().Get(HeaderXTraceID)
	if traceID == "" {
		t.Error("expected X-Trace-ID header to be present")
	}

	spanID := rec.Header().Get(HeaderXSpanID)
	if spanID == "" {
		t.Error("expected X-Span-ID header to be present")
	}
}

func TestTracingMiddleware_PropagateIncomingW3CTraceContext(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(t.Context()) }()
	tracer := tp.Tracer("test-tracer")

	// Standard W3C TraceContext traceparent: version-trace_id-parent_id-flags
	incomingTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	incomingParentID := "00f067aa0ba902b7"
	traceparent := "00-" + incomingTraceID + "-" + incomingParentID + "-01"

	handler := Tracing(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if span.SpanContext().TraceID().String() != incomingTraceID {
			t.Errorf("expected trace ID %s, got %s", incomingTraceID, span.SpanContext().TraceID().String())
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", nil)
	req.Header.Set("traceparent", traceparent)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get(HeaderXTraceID) != incomingTraceID {
		t.Errorf("expected X-Trace-ID to match incoming %s, got %s", incomingTraceID, rec.Header().Get(HeaderXTraceID))
	}
}

func TestTracingMiddleware_ErrorStatus(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(t.Context()) }()
	tracer := tp.Tracer("test-tracer")

	handler := Tracing(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal_error"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/error", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
	if rec.Header().Get(HeaderXTraceID) == "" {
		t.Error("expected X-Trace-ID header on error response")
	}
}

func TestTracingMiddleware_DefaultTracer(t *testing.T) {
	// Call Tracing(nil) to test fallback to telemetry.Tracer()
	handler := Tracing(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func BenchmarkTracingMiddleware(b *testing.B) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(b.Context()) }()
	tracer := tp.Tracer("bench-tracer")

	handler := Tracing(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/bench", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
