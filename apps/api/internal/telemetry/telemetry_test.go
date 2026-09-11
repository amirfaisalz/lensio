package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInit_DefaultConfig(t *testing.T) {
	ctx := context.Background()
	cfg := Config{}

	tel, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() {
		_ = tel.Shutdown(context.Background())
	}()

	if tel.TracerProvider == nil {
		t.Error("expected non-nil TracerProvider")
	}
	if tel.MeterProvider == nil {
		t.Error("expected non-nil MeterProvider")
	}
	if tel.promHandler == nil {
		t.Error("expected non-nil promHandler")
	}

	tracer := Tracer()
	if tracer == nil {
		t.Error("expected non-nil global tracer")
	}

	meter := Meter()
	if meter == nil {
		t.Error("expected non-nil global meter")
	}

	h := PrometheusHandler()
	if h == nil {
		t.Error("expected non-nil prometheus handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /metrics, got %d", rec.Code)
	}
}

func TestInit_CustomConfig(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		ServiceName:     "custom-lensio",
		ServiceVersion:  "2.1.0",
		Environment:     "staging",
		TraceSampleRate: 0.5,
	}

	tel, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("Init with custom config failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tel.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestSetGlobalTelemetry_Nil(t *testing.T) {
	SetGlobalTelemetry(nil)

	tracer := Tracer()
	if tracer == nil {
		t.Error("expected fallback tracer when globalTelemetry is nil")
	}

	meter := Meter()
	if meter == nil {
		t.Error("expected fallback meter when globalTelemetry is nil")
	}

	h := PrometheusHandler()
	if h == nil {
		t.Error("expected fallback promHandler when globalTelemetry is nil")
	}
}

func TestTelemetry_ShutdownNilProviders(t *testing.T) {
	tel := &Telemetry{}
	if err := tel.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown with nil providers returned error: %v", err)
	}
}
