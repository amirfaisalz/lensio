package http

import (
	"net/http"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/quota"
	"github.com/amirfaisalz/nusaid/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/internal/usage"
	"github.com/amirfaisalz/nusaid/services/ocr"
	"github.com/amirfaisalz/nusaid/services/ocr/providers"
)

// RouterDeps encapsulates optional and required dependencies for the HTTP API router.
type RouterDeps struct {
	Pinger        store.Pinger
	KeyStore      store.APIKeyStore
	OCREngine     ocr.OCREngine
	OCRStore      store.OCRRequestStore
	UsageStore    store.UsageStore
	AccountStore  store.AccountStore
	AuditStore    store.AuditStore
	RateLimiter   *ratelimit.Limiter
	UsageRecorder *usage.Recorder
}

// NewRouter constructs the root HTTP handler for backward compatibility.
func NewRouter(pinger store.Pinger, keyStore store.APIKeyStore, ocrEngine ocr.OCREngine, ocrStore store.OCRRequestStore) http.Handler {
	return NewRouterWithDeps(RouterDeps{
		Pinger:    pinger,
		KeyStore:  keyStore,
		OCREngine: ocrEngine,
		OCRStore:  ocrStore,
	})
}

// NewRouterWithDeps constructs the root HTTP handler with standard probes, documentation,
// security middleware, and API v1 endpoints registered.
func NewRouterWithDeps(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	// Probes (PRD Section 17)
	mux.HandleFunc("GET /health", handlers.HealthHandler())
	mux.HandleFunc("GET /ready", handlers.ReadyHandler(deps.Pinger))

	// Documentation & Contract (PRD Section 29)
	mux.HandleFunc("GET /openapi", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /docs", handlers.DocsHandler("/openapi.yaml"))

	// API v1 Routes (PRD Section 6)
	if deps.KeyStore != nil {
		// API Key Lifecycle (PRD Section 7 & 8)
		mux.HandleFunc("POST /api/v1/auth/api-keys", handlers.CreateAPIKeyHandler(deps.KeyStore, deps.AuditStore, handlers.DefaultOrgID))
		mux.HandleFunc("GET /api/v1/auth/api-keys", handlers.ListAPIKeysHandler(deps.KeyStore, handlers.DefaultOrgID))
		mux.HandleFunc("DELETE /api/v1/auth/api-keys/{id}", handlers.RevokeAPIKeyHandler(deps.KeyStore, deps.AuditStore, handlers.DefaultOrgID))

		authMiddleware := middleware.Authenticate(deps.KeyStore)
		scopeWriteMiddleware := middleware.RequireScope("ocr:write")
		scopeReadMiddleware := middleware.RequireScope("ocr:read")
		scopeUsageMiddleware := middleware.RequireScope("usage:read")

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
		if deps.OCREngine == nil {
			deps.OCREngine = providers.NewMockEngine()
		}

		var quotaChecker handlers.QuotaChecker
		if deps.AccountStore != nil && deps.UsageStore != nil {
			quotaChecker = quota.NewEnforcer(deps.AccountStore, deps.UsageStore)
		}

		ktpHandler := handlers.KTPOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/ktp", authMiddleware(scopeWriteMiddleware(ktpHandler)))

		if deps.OCRStore != nil {
			getOcrHandler := handlers.GetOCRRequestHandler(deps.OCRStore)
			mux.Handle("GET /api/v1/ocr/{id}", authMiddleware(scopeReadMiddleware(getOcrHandler)))
		}

		// Usage Analytics (PRD Section 11)
		mux.Handle("GET /api/v1/usage", authMiddleware(scopeUsageMiddleware(handlers.UsageSummaryHandler(deps.UsageStore, deps.AccountStore, handlers.DefaultOrgID))))
		mux.Handle("GET /api/v1/usage/daily", authMiddleware(scopeUsageMiddleware(handlers.DailyUsageHandler(deps.UsageStore, handlers.DefaultOrgID))))
		mux.Handle("GET /api/v1/usage/endpoints", authMiddleware(scopeUsageMiddleware(handlers.EndpointUsageHandler(deps.UsageStore, handlers.DefaultOrgID))))

		// Account & Plan Management (PRD Section 5 & 30)
		if deps.AccountStore != nil {
			mux.Handle("GET /api/v1/account", authMiddleware(handlers.AccountDetailsHandler(deps.AccountStore, handlers.DefaultOrgID)))
			mux.Handle("GET /api/v1/account/plan", authMiddleware(handlers.AccountPlanHandler(deps.AccountStore, handlers.DefaultOrgID)))
			mux.Handle("PUT /api/v1/account/plan", authMiddleware(handlers.UpdatePlanHandler(deps.AccountStore, deps.AuditStore, handlers.DefaultOrgID)))
		}
	}

	var rootHandler http.Handler = mux

	// Rate Limiter Middleware (PRD Section 9)
	if deps.RateLimiter != nil {
		rlMw := middleware.NewRateLimitMiddleware(deps.RateLimiter, deps.AccountStore, handlers.DefaultOrgID)
		rootHandler = rlMw.Handler(rootHandler)
	}

	// Usage Metering Middleware (PRD Section 11)
	if deps.UsageRecorder != nil {
		rootHandler = middleware.UsageMetering(deps.UsageRecorder, handlers.DefaultOrgID)(rootHandler)
	}

	// Global Middleware: Request ID injection & header emission (PRD Section 20)
	return middleware.RequestID(rootHandler)
}
