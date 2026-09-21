package http

import (
	"net/http"

	"github.com/amirfaisalz/lensio/apps/api/internal/authz"
	"github.com/amirfaisalz/lensio/apps/api/internal/email"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/quota"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
	"github.com/amirfaisalz/lensio/apps/api/internal/usage"
	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
)

// RouterDeps encapsulates optional and required dependencies for the HTTP API router.
type RouterDeps struct {
	Pinger           store.Pinger
	KeyStore         store.APIKeyStore
	OCREngine        ocr.OCREngine
	OCRStore         store.OCRRequestStore
	UsageStore       store.UsageStore
	AccountStore     store.AccountStore
	AuditStore       store.AuditStore
	RateLimiter      ratelimit.RateLimiter
	UsageRecorder    *usage.Recorder
	IdempotencyStore idempotency.Store
	// IdempotencySealer encrypts cached response bodies at rest. Supply one in
	// any deployment processing real documents; nil stores them in plaintext.
	IdempotencySealer *idempotency.Sealer
	// SessionRevoker invalidates a user's session tokens on logout. Nil leaves
	// logout cookie-only, which cannot stop an already-copied token.
	SessionRevoker          handlers.SessionRevoker
	SessionCacheInvalidator handlers.SessionCacheInvalidator
	// RateLimitReplicas divides plan limits across API instances; 0 or 1 means single-instance.
	RateLimitReplicas int
	// EmailSender delivers verification and password-reset mail. Without one the
	// self-service signup and recovery flows cannot complete.
	EmailSender email.Sender
	// AppBaseURL is the dashboard origin used to build links in those emails.
	AppBaseURL         string
	OIDCValidator      middleware.TokenValidator
	Authorizer         authz.Authorizer
	CORSAllowedOrigins []string
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

	// Probes (PRD Section 17 & 16)
	mux.HandleFunc("GET /health", handlers.HealthHandler())
	mux.HandleFunc("GET /ready", handlers.ReadyHandler(deps.Pinger))
	mux.Handle("GET /metrics", telemetry.PrometheusHandler())

	// Documentation & Contract (PRD Section 29)
	mux.HandleFunc("GET /openapi", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPIHandler())
	mux.HandleFunc("GET /docs", handlers.DocsHandler("/openapi.yaml"))

	// Rate limiting is built once and shared by the public (unauthenticated) and
	// authenticated route groups so both draw on the same token buckets.
	var rlMw *middleware.RateLimitMiddleware
	if deps.RateLimiter != nil {
		rlMw = middleware.NewRateLimitMiddleware(deps.RateLimiter, deps.AccountStore, "")
		rlMw.SetReplicaCount(deps.RateLimitReplicas)
	}

	// throttle applies the limiter when one is configured. Public auth routes are
	// unauthenticated, so the limiter keys them by client IP. Without this they
	// were reachable without any bucket at all, leaving login an unthrottled
	// password-guessing and bcrypt-CPU oracle.
	throttle := func(h http.Handler) http.Handler {
		if rlMw == nil {
			return h
		}
		return rlMw.Handler(h)
	}

	// Public Authentication Endpoints (Registration, Verification, Login, Logout)
	if deps.AccountStore != nil {
		mux.Handle("POST /api/v1/auth/register", throttle(handlers.RegisterHandler(deps.AccountStore, deps.EmailSender, deps.AppBaseURL)))
		mux.Handle("POST /api/v1/auth/verify-email", throttle(handlers.VerifyEmailHandler(deps.AccountStore)))
		mux.Handle("POST /api/v1/auth/login", throttle(handlers.LoginHandler(deps.AccountStore)))
		mux.Handle("POST /api/v1/auth/logout", handlers.LogoutHandler(deps.SessionRevoker, deps.SessionCacheInvalidator))

		// Password recovery. Both are public and unauthenticated, so both are
		// throttled; a reset request is also a way to send mail on demand.
		if resetStore, ok := deps.AccountStore.(handlers.PasswordResetStore); ok {
			mux.Handle("POST /api/v1/auth/password-reset",
				throttle(handlers.RequestPasswordResetHandler(resetStore, deps.EmailSender, deps.AppBaseURL)))
			mux.Handle("POST /api/v1/auth/password-reset/confirm",
				throttle(handlers.ConfirmPasswordResetHandler(resetStore)))
		}
	}

	// API v1 Routes (PRD Section 6)
	if deps.KeyStore != nil {
		var baseAuthMiddleware func(http.Handler) http.Handler
		if deps.OIDCValidator != nil {
			baseAuthMiddleware = middleware.DualAuth(deps.KeyStore, deps.OIDCValidator)
		} else {
			baseAuthMiddleware = middleware.Authenticate(deps.KeyStore)
		}

		authMiddleware := baseAuthMiddleware
		if rlMw != nil {
			authMiddleware = func(next http.Handler) http.Handler {
				return baseAuthMiddleware(rlMw.Handler(next))
			}
		}

		// API Key Lifecycle (PRD Section 7 & 8; Phase 11.5 ReBAC)
		createKeyHandler := handlers.CreateAPIKeyHandler(deps.KeyStore, deps.AuditStore, deps.Authorizer, handlers.DefaultOrgID)
		revokeKeyHandler := handlers.RevokeAPIKeyHandler(deps.KeyStore, deps.AuditStore, deps.Authorizer, handlers.DefaultOrgID)
		listKeyHandler := handlers.ListAPIKeysHandler(deps.KeyStore, handlers.DefaultOrgID)

		mux.Handle("POST /api/v1/auth/api-keys", authMiddleware(createKeyHandler))
		mux.Handle("DELETE /api/v1/auth/api-keys/{id}", authMiddleware(revokeKeyHandler))
		mux.Handle("GET /api/v1/auth/api-keys", authMiddleware(listKeyHandler))
		mux.Handle("GET /api/v1/auth/me", authMiddleware(handlers.MeHandler(deps.AccountStore)))

		scopeWriteMiddleware := middleware.RequireScope("ocr:write")
		scopeReadMiddleware := middleware.RequireScope("ocr:read")
		scopeUsageMiddleware := middleware.RequireScope("usage:read")

		// Protected verification probe for testing Auth & Scopes
		verifyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user := middleware.GetOIDCUser(r.Context()); user != nil {
				response.JSON(w, http.StatusOK, map[string]any{
					"status":             "authenticated",
					"auth_type":          "oidc",
					"subject":            user.Subject,
					"email":              user.Email,
					"preferred_username": user.PreferredUsername,
					"roles":              user.Roles,
				})
				return
			}
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

		if deps.IdempotencyStore == nil {
			deps.IdempotencyStore = idempotency.NewMemoryStore(idempotency.DefaultReplayTTL)
		}
		idempotencyMiddleware := middleware.Idempotency(deps.IdempotencyStore, deps.IdempotencySealer)
		sessionOrgMiddleware := middleware.SessionOrg(deps.AccountStore)

		ktpHandler := handlers.KTPOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/ktp", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(ktpHandler)))))

		simHandler := handlers.SIMOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/sim", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(simHandler)))))

		passportHandler := handlers.PassportOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/passport", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(passportHandler)))))

		npwpHandler := handlers.NPWPOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/npwp", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(npwpHandler)))))

		kkHandler := handlers.KKOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/kk", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(kkHandler)))))

		invoiceHandler := handlers.InvoiceOCRHandler(deps.OCREngine, deps.OCRStore, quotaChecker)
		mux.Handle("POST /api/v1/ocr/invoice", authMiddleware(sessionOrgMiddleware(scopeWriteMiddleware(idempotencyMiddleware(invoiceHandler)))))

		if deps.OCRStore != nil {
			getOcrHandler := handlers.GetOCRRequestHandler(deps.OCRStore)
			mux.Handle("GET /api/v1/ocr/{id}", authMiddleware(scopeReadMiddleware(getOcrHandler)))
		}

		// Usage Analytics (PRD Section 11)
		mux.Handle("GET /api/v1/usage", authMiddleware(scopeUsageMiddleware(handlers.UsageSummaryHandler(deps.UsageStore, deps.AccountStore, handlers.DefaultOrgID))))
		mux.Handle("GET /api/v1/usage/daily", authMiddleware(scopeUsageMiddleware(handlers.DailyUsageHandler(deps.UsageStore, deps.AccountStore, handlers.DefaultOrgID))))
		mux.Handle("GET /api/v1/usage/endpoints", authMiddleware(scopeUsageMiddleware(handlers.EndpointUsageHandler(deps.UsageStore, deps.AccountStore, handlers.DefaultOrgID))))
		mux.Handle("GET /api/v1/usage/records", authMiddleware(scopeUsageMiddleware(handlers.UsageRecordsHandler(deps.UsageStore, deps.AccountStore, handlers.DefaultOrgID))))

		// Account & Plan Management (PRD Section 5 & 30)
		if deps.AccountStore != nil {
			mux.Handle("GET /api/v1/account", authMiddleware(handlers.AccountDetailsHandler(deps.AccountStore, handlers.DefaultOrgID)))
			mux.Handle("GET /api/v1/account/plan", authMiddleware(handlers.AccountPlanHandler(deps.AccountStore, handlers.DefaultOrgID)))
			mux.Handle("PUT /api/v1/account/plan", authMiddleware(handlers.UpdatePlanHandler(deps.AccountStore, deps.AuditStore, handlers.DefaultOrgID)))
			mux.Handle("GET /api/v1/account/members", authMiddleware(handlers.AccountMembersHandler(deps.AccountStore, handlers.DefaultOrgID)))
			mux.Handle("POST /api/v1/account/organizations", authMiddleware(handlers.CreateOrganizationHandler(deps.AccountStore)))
		}
	}

	var rootHandler http.Handler = middleware.Recover(mux)

	// Usage Metering & Metrics Middleware (PRD Section 11 & 16)
	rootHandler = middleware.UsageMetering(deps.UsageRecorder, handlers.DefaultOrgID)(rootHandler)

	// Global Middleware: panic recovery, Request ID injection, Distributed Tracing, and CORS (PRD Section 16 & 20)
	// Recover sits inside UsageMetering so a panicking handler is still metered
	// with its 500 status instead of dropping the connection with no response.
	corsHandler := middleware.NewCORSMiddleware(deps.CORSAllowedOrigins)(rootHandler)
	return middleware.RequestID(middleware.Tracing(nil)(corsHandler))
}
