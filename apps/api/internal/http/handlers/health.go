package handlers

import (
	"net/http"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
)

// HealthResponse represents the response payload for the liveness probe.
type HealthResponse struct {
	Status string `json:"status"`
}

// HealthHandler returns an HTTP handler for process liveness checks.
// It verifies the process is alive without checking external dependencies.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, HealthResponse{
			Status: "ok",
		})
	}
}
