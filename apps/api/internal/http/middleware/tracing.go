package middleware

import (
	"fmt"
	"net/http"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	// HeaderXTraceID is the HTTP header returned to the client carrying the OpenTelemetry trace ID.
	HeaderXTraceID = "X-Trace-ID"
	// HeaderXSpanID is the HTTP header returned to the client carrying the root span ID.
	HeaderXSpanID = "X-Span-ID"
)

// Tracing returns an HTTP middleware that extracts incoming W3C TraceContext headers,
// starts a distributed root span, records HTTP semantic attributes, and injects X-Trace-ID
// into the response headers.
func Tracing(tracer trace.Tracer) func(http.Handler) http.Handler {
	if tracer == nil {
		tracer = telemetry.Tracer()
	}

	propagator := otel.GetTextMapPropagator()
	if len(propagator.Fields()) == 0 {
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract existing trace context from W3C headers (traceparent, tracestate)
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// 2. Start root server span
			spanName := fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// 3. Inject trace identifiers into response headers for client/operator correlation
			spanCtx := span.SpanContext()
			if spanCtx.IsValid() {
				w.Header().Set(HeaderXTraceID, spanCtx.TraceID().String())
				w.Header().Set(HeaderXSpanID, spanCtx.SpanID().String())
			}

			// 4. Record standard HTTP semantic attributes
			reqID := response.GetRequestID(ctx)
			if reqID == "" {
				reqID = r.Header.Get(HeaderXRequestID)
			}

			clientIP := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				clientIP = xff
			}

			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", r.URL.Path),
				attribute.String("http.target", r.URL.RequestURI()),
				attribute.String("http.client_ip", clientIP),
				attribute.String("http.user_agent", r.UserAgent()),
			}
			if reqID != "" {
				attrs = append(attrs, attribute.String("request_id", reqID))
			}
			span.SetAttributes(attrs...)

			// 5. Wrap response writer to capture HTTP status code
			sw := &tracingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(sw, r.WithContext(ctx))

			// 6. Record status code and mark span error on 5xx
			span.SetAttributes(attribute.Int("http.status_code", sw.statusCode))
			if sw.statusCode >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d error", sw.statusCode))
			} else {
				span.SetStatus(codes.Ok, "OK")
			}
		})
	}
}

type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *tracingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
