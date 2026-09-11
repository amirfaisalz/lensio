package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// ReadyResponse represents the response payload for the readiness probe.
type ReadyResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// ReadyHandler returns an HTTP handler for readiness checks.
// It probes critical dependencies (PostgreSQL) and returns 503 if unreachable.
func ReadyHandler(pinger store.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pinger == nil {
			response.JSON(w, http.StatusServiceUnavailable, ReadyResponse{
				Status:   "unready",
				Database: "not_configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pinger.PingContext(ctx); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, ReadyResponse{
				Status:   "unready",
				Database: "disconnected",
			})
			return
		}

		response.JSON(w, http.StatusOK, ReadyResponse{
			Status:   "ready",
			Database: "connected",
		})
	}
}
