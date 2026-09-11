package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultServiceName is the default OpenTelemetry service name for NusaID API.
	DefaultServiceName = "nusaid-api"
	// DefaultServiceVersion is the default application version.
	DefaultServiceVersion = "1.0.0"
)

// Config configures OpenTelemetry tracing and metrics.
type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// Telemetry encapsulates OpenTelemetry tracing, metrics, and Prometheus exporter instances.
type Telemetry struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	promHandler    http.Handler
}

var (
	globalMu        sync.RWMutex
	globalTelemetry *Telemetry
	globalTracer    trace.Tracer
	globalMeter     metric.Meter
)

func init() {
	tp := sdktrace.NewTracerProvider()
	globalTracer = tp.Tracer(DefaultServiceName)
	globalMeter = otel.GetMeterProvider().Meter(DefaultServiceName)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

// Init sets up the OpenTelemetry TracerProvider, MeterProvider, and Prometheus exporter.
func Init(ctx context.Context, cfg Config) (*Telemetry, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = DefaultServiceName
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = DefaultServiceVersion
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	// 1. Build OpenTelemetry Resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telemetry resource: %w", err)
	}

	// 2. Set up TracerProvider with W3C TraceContext Propagator
	sampler := sdktrace.AlwaysSample()
	if cfg.TraceSampleRate > 0 && cfg.TraceSampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.TraceSampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 3. Set up Prometheus Exporter and MeterProvider
	promExporter, err := otelprom.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(mp)

	t := &Telemetry{
		TracerProvider: tp,
		MeterProvider:  mp,
		promHandler:    promhttp.Handler(),
	}

	// 4. Initialize core metrics
	if err := initInstruments(mp.Meter(cfg.ServiceName)); err != nil {
		return nil, fmt.Errorf("initializing metric instruments: %w", err)
	}

	SetGlobalTelemetry(t)
	return t, nil
}

// SetGlobalTelemetry updates the global telemetry instance and package-level accessors.
func SetGlobalTelemetry(t *Telemetry) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalTelemetry = t
	if t != nil && t.TracerProvider != nil {
		globalTracer = t.TracerProvider.Tracer(DefaultServiceName)
	} else {
		globalTracer = sdktrace.NewTracerProvider().Tracer(DefaultServiceName)
	}

	if t != nil && t.MeterProvider != nil {
		globalMeter = t.MeterProvider.Meter(DefaultServiceName)
	} else {
		globalMeter = otel.GetMeterProvider().Meter(DefaultServiceName)
	}
}

// Tracer returns the global OpenTelemetry Tracer.
func Tracer() trace.Tracer {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalTracer
}

// Meter returns the global OpenTelemetry Meter.
func Meter() metric.Meter {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalMeter
}

// PrometheusHandler returns an http.Handler that serves Prometheus metrics at /metrics.
func PrometheusHandler() http.Handler {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalTelemetry != nil && globalTelemetry.promHandler != nil {
		return globalTelemetry.promHandler
	}
	return promhttp.Handler()
}

// Shutdown flushes and terminates the global TracerProvider and MeterProvider.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error

	if t.TracerProvider != nil {
		if err := t.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutting down tracer provider: %w", err))
		}
	}

	if t.MeterProvider != nil {
		if err := t.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutting down meter provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("telemetry shutdown errors: %v", errs)
	}
	return nil
}
