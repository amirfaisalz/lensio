package http

import (
	"net/http"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/services/ocr"
	"github.com/amirfaisalz/nusaid/services/ocr/providers"
)

// NewRouter constructs the root HTTP handler with standard probes, documentation,
// and API v1 endpoints registered.
func NewRouter(pinger store.Pinger, keyStore store.APIKeyStore, ocrEngine ocr.OCREngine, ocrStore store.OCRRequestStore) http.Handler {
	mux := http.NewServeMux()

	// Probes (PRD Section 17)
	mux.HandleFunc("GET /health", handlers.HealthHandler())
	mux.HandleFunc("GET /ready", handlers.ReadyHandler(pinger))

	// Documentation & Contract (PRD Section 29)
	mux.HandleFunc("GET /openapi", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /docs", handlers.DocsHandler("/openapi.yaml"))

	// API v1 Routes (PRD Section 6)
	if keyStore != nil {
		// API Key Lifecycle (PRD Section 7 & 8)
		mux.HandleFunc("POST /api/v1/auth/api-keys", handlers.CreateAPIKeyHandler(keyStore, handlers.DefaultOrgID))
		mux.HandleFunc("GET /api/v1/auth/api-keys", handlers.ListAPIKeysHandler(keyStore, handlers.DefaultOrgID))
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(keyStore, handlers.DefaultOrgID))

		authMiddleware := middleware.Authenticate(keyStore)
		scopeWriteMiddleware := middleware.RequireScope("ocr:write")
		scopeReadMiddleware := middleware.RequireScope("ocr:read")

		// Protected verification probe for testing Auth & Scopes
		verifyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := middleware.GetAPIKey(r.Context())
			response.JSON(w, http.StatusOK, map[string]any{
				"status": "authenticated",
				"key_id": key.ID,
				"org_id": key.OrgID,
				"scopes": key.Scopes,
			})
		})
		mux.Handle("GET /api/v1/auth/verify", authMiddleware(scopeWriteMiddleware(verifyHandler)))

		// OCR Pipeline Endpoints (PRD Section 12, 13 & 14)
		if ocrEngine == nil {
			ocrEngine = providers.NewMockEngine()
		}

		ktpHandler := handlers.KTPOCRHandler(ocrEngine, ocrStore)
		mux.Handle("POST /api/v1/ocr/ktp", authMiddleware(scopeWriteMiddleware(ktpHandler)))

		if ocrStore != nil {
			getOcrHandler := handlers.GetOCRRequestHandler(ocrStore)
			mux.Handle("GET /api/v1/ocr/{id}", authMiddleware(scopeReadMiddleware(getOcrHandler)))
		}
	}

	// Global Middleware: Request ID injection & header emission (PRD Section 20)
	return middleware.RequestID(mux)
}
