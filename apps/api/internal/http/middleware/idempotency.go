package middleware

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
)

type idempotencyResponseWriter struct {
	underlying  http.ResponseWriter
	statusCode  int
	wroteHeader bool
	body        *bytes.Buffer
}

func newIdempotencyResponseWriter(w http.ResponseWriter) *idempotencyResponseWriter {
	return &idempotencyResponseWriter{
		underlying: w,
		statusCode: http.StatusOK,
		body:       &bytes.Buffer{},
	}
}

func (w *idempotencyResponseWriter) Header() http.Header {
	return w.underlying.Header()
}

func (w *idempotencyResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.underlying.WriteHeader(code)
	}
}

func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.body.Write(b)
	return w.underlying.Write(b)
}

func (w *idempotencyResponseWriter) Flush() {
	if f, ok := w.underlying.(http.Flusher); ok {
		f.Flush()
	}
}

// Idempotency returns a middleware that guarantees safe client retries using the Idempotency-Key header.
// It detects in-progress requests (HTTP 409), payload mismatches (HTTP 422),
// and replays cached responses (HTTP 200 with Idempotent-Replayed: true).
//
// sealer encrypts the cached response body before it is persisted. It is required
// in any deployment that handles real documents: OCR responses contain extracted
// identity fields, so an unsealed cache turns idempotency into 24 hours of PII at
// rest. Passing nil disables encryption and is intended only for tests.
func Idempotency(store idempotency.Store, sealer *idempotency.Sealer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}

			keyHeader := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if keyHeader == "" {
				// No idempotency key provided; proceed normally without caching
				next.ServeHTTP(w, r)
				return
			}

			if len(keyHeader) > 256 {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusBadRequest,
					response.CodeInvalidRequest,
					"Idempotency-Key header exceeds maximum allowed length of 256 characters",
				)
				return
			}

			orgID := "00000000-0000-0000-0000-000000000001"
			if key := GetAPIKey(r.Context()); key != nil && key.OrgID != "" {
				orgID = key.OrgID
			}

			payloadHash, err := idempotency.ComputePayloadHash(r, idempotency.DefaultMaxBodySize)
			if err != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusBadRequest,
					response.CodeInvalidDocument,
					err.Error(),
				)
				return
			}

			rec, isNew, err := store.LockOrGet(r.Context(), orgID, keyHeader, payloadHash, idempotency.DefaultReplayTTL)
			if err != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusInternalServerError,
					response.CodeInternalError,
					"Failed processing idempotency key",
				)
				return
			}

			if !isNew {
				// Existing key found
				if rec.RequestHash != payloadHash {
					response.ErrorWithRequest(
						w,
						r,
						http.StatusUnprocessableEntity,
						response.CodeIdempotencyMismatch,
						"Idempotency-Key was previously used with a different request payload",
					)
					return
				}

				if rec.Status == idempotency.StatusInProgress {
					response.ErrorWithRequest(
						w,
						r,
						http.StatusConflict,
						response.CodeRequestInProgress,
						"A request with this Idempotency-Key is currently in progress",
					)
					return
				}

				if rec.Status == idempotency.StatusCompleted {
					body, err := sealer.Open(rec.ResponseBody)
					if err != nil {
						slog.ErrorContext(r.Context(), "failed decrypting cached idempotent response",
							"error", err.Error(),
							"request_id", response.GetRequestID(r.Context()),
						)
						response.ErrorWithRequest(
							w,
							r,
							http.StatusInternalServerError,
							response.CodeInternalError,
							"Failed replaying idempotent response",
						)
						return
					}
					for k, v := range rec.Headers {
						w.Header().Set(k, v)
					}
					w.Header().Set("Idempotent-Replayed", "true")
					w.WriteHeader(rec.StatusCode)
					_, _ = w.Write(body)
					return
				}
			}

			// New lock acquired: record response
			rw := newIdempotencyResponseWriter(w)
			defer func() {
				if recovered := recover(); recovered != nil {
					_ = store.Release(r.Context(), orgID, keyHeader)
					slog.Error("recovered from panic in handler under idempotency middleware",
						"panic", fmt.Sprint(recovered),
						"request_id", response.GetRequestID(r.Context()),
					)
					response.ErrorWithRequest(
						w,
						r,
						http.StatusInternalServerError,
						response.CodeInternalError,
						"An unexpected internal error occurred",
					)
				}
			}()

			next.ServeHTTP(rw, r)

			// Persist successful or client error responses, except 429 rate/quota
			// signals: those are retryable by nature and must never be replayed
			// after the caller's quota or rate window has reset.
			if rw.statusCode < http.StatusInternalServerError && rw.statusCode != http.StatusTooManyRequests {
				headersToCache := make(map[string]string)
				for k, vals := range rw.Header() {
					if len(vals) > 0 {
						headersToCache[k] = vals[0]
					}
				}
				sealed, err := sealer.Seal(rw.body.Bytes())
				if err != nil {
					// Never persist the plaintext as a fallback: release the lock
					// so the client can retry rather than leaving PII unsealed.
					slog.ErrorContext(r.Context(), "failed sealing idempotent response; releasing lock",
						"error", err.Error(),
						"request_id", response.GetRequestID(r.Context()),
					)
					_ = store.Release(r.Context(), orgID, keyHeader)
					return
				}
				_ = store.Complete(r.Context(), orgID, keyHeader, rw.statusCode, headersToCache, sealed)
			} else {
				// 5xx errors and 429 rate/quota signals: release the lock to
				// allow clients to retry with the same key.
				_ = store.Release(r.Context(), orgID, keyHeader)
			}
		})
	}
}
