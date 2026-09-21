package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

// Recover converts a panicking handler into a standard 500 error envelope.
//
// net/http already recovers panics per connection, but it does so by closing the
// connection without writing a response: the caller sees a reset, the request
// never reaches the metering middleware, and no request_id is ever correlated.
// Recovering here keeps a panic observable and gives the client an envelope it
// can act on, without letting the stack trace escape the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			// ErrAbortHandler is net/http's documented way to abandon a response
			// on purpose; re-panic so the server handles it as intended.
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			slog.ErrorContext(r.Context(), "recovered from panic in http handler",
				slog.Any("panic", recovered),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("stack", string(debug.Stack())),
			)

			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"An unexpected internal error occurred",
			)
		}()

		next.ServeHTTP(w, r)
	})
}
