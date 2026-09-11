package middleware

import (
	"net/http"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/internal/usage"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// UsageMetering returns a middleware that measures request latency, captures status codes,
// and asynchronously dispatches usage records to the non-blocking usage recorder.
func UsageMetering(recorder *usage.Recorder, defaultOrgID string) func(http.Handler) http.Handler {
	if defaultOrgID == "" {
		defaultOrgID = "00000000-0000-0000-0000-000000000001"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if recorder == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // default if WriteHeader is not called explicitly
			}

			next.ServeHTTP(sw, r)

			latencyMS := int(time.Since(start).Milliseconds())

			orgID := defaultOrgID
			var apiKeyID *string

			if key := GetAPIKey(r.Context()); key != nil {
				if key.OrgID != "" {
					orgID = key.OrgID
				}
				if key.ID != "" {
					apiKeyID = &key.ID
				}
			}

			reqID := response.GetRequestID(r.Context())
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
