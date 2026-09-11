package http

import (
	"net/http"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

// NewRouter constructs the root HTTP handler with standard probes registered.
func NewRouter(pinger store.Pinger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handlers.HealthHandler())
	mux.HandleFunc("GET /ready", handlers.ReadyHandler(pinger))

	return mux
}
