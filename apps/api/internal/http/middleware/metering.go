package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
	"github.com/amirfaisalz/lensio/apps/api/internal/usage"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	ctx        context.Context
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) SetRequestContext(ctx context.Context) {
	w.ctx = ctx
}

// UsageMetering returns a middleware that measures request latency, captures status codes,
// and asynchronously dispatches usage records to the non-blocking usage recorder.
func UsageMetering(recorder *usage.Recorder, defaultOrgID string) func(http.Handler) http.Handler {
	_ = defaultOrgID

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // default if WriteHeader is not called explicitly
			}

			next.ServeHTTP(sw, r)

			latencyMS := int(time.Since(start).Milliseconds())
			durationSec := float64(latencyMS) / 1000.0

			// Record OpenTelemetry & Prometheus availability and latency metrics
			telemetry.RecordHTTPRequest(r.Context(), r.URL.Path, r.Method, sw.statusCode, durationSec)
			if sw.statusCode >= http.StatusBadRequest {
				telemetry.RecordHTTPError(r.Context(), r.URL.Path, sw.statusCode, http.StatusText(sw.statusCode))
			}

			if recorder == nil {
				return
			}

			// Exclude internal health probes, API docs, public auth, and dashboard telemetry/management queries from tenant usage records
			if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" ||
				r.URL.Path == "/docs" || r.URL.Path == "/openapi" || r.URL.Path == "/openapi.yaml" ||
				strings.HasPrefix(r.URL.Path, "/api/v1/auth") ||
				strings.HasPrefix(r.URL.Path, "/api/v1/usage") ||
				strings.HasPrefix(r.URL.Path, "/api/v1/account") {
				return
			}

			// Exclude idempotently replayed responses from creating duplicate usage records / consuming quota
			if sw.Header().Get("Idempotent-Replayed") == "true" {
				return
			}

			reqCtx := r.Context()
			if sw.ctx != nil {
				reqCtx = sw.ctx
			}

			key := GetAPIKey(reqCtx)
			if key == nil || key.OrgID == "" {
				return
			}

			orgID := key.OrgID
			var apiKeyID *string
			if key.ID != "" {
				apiKeyID = &key.ID
			}

			reqID := response.GetRequestID(reqCtx)
			if reqID == "" {
				reqID = r.Header.Get("X-Request-ID")
			}

			recorder.Record(&store.UsageRecord{
				OrgID:      orgID,
				APIKeyID:   apiKeyID,
				RequestID:  reqID,
				Endpoint:   r.URL.Path,
				StatusCode: sw.statusCode,
				LatencyMS:  latencyMS,
				Timestamp:  start,
			})
		})
	}
}
