package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
	"github.com/amirfaisalz/lensio/apps/api/internal/authz"
	"github.com/amirfaisalz/lensio/apps/api/internal/config"
	"github.com/amirfaisalz/lensio/apps/api/internal/email"
	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
	"github.com/amirfaisalz/lensio/apps/api/internal/usage"
	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
)

func main() {
	cfg := config.Load()

	// Initialize OpenTelemetry Tracing, Metrics & Prometheus Exporter (PRD Section 16)
	tel, err := telemetry.Init(context.Background(), telemetry.Config{
		ServiceName:    "lensio-api",
		ServiceVersion: "1.0.0",
		Environment:    cfg.Env,
	})
	if err != nil {
		slog.Error("failed initializing OpenTelemetry", slog.String("error", err.Error()))
	}

	// Initialize structured JSON logging with strict PII sanitization and OpenTelemetry correlation
	logger := telemetry.InitLogger(slog.LevelInfo, os.Stdout)
	slog.SetDefault(logger)

	logger.Info("starting lensio api service",
		slog.String("env", cfg.Env),
		slog.String("port", cfg.Port),
	)

	if cfg.Env == "production" || cfg.Env == "staging" {
		if strings.TrimSpace(os.Getenv("SESSION_SECRET")) == "" {
			logger.Error("SESSION_SECRET must be set in production/staging; refusing to start with dev fallback secret")
			os.Exit(1)
		}
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			logger.Error("DATABASE_URL must be set in production/staging; refusing to start in ephemeral mode")
			os.Exit(1)
		}
	}

	var (
		db     *store.DB
		pinger store.Pinger
	)

	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var err error
		db, err = store.New(ctx, cfg.DatabaseURL)
		cancel()

		if err != nil {
			logger.Error("failed to connect to database", slog.String("error", err.Error()))
		} else {
			logger.Info("connected to database successfully")
			pinger = db

			// Apply database migrations on startup
			if err := store.RunMigrationsUp(db.DB); err != nil {
				logger.Error("failed to run database migrations", slog.String("error", err.Error()))
			} else {
				logger.Info("database migrations applied successfully")
			}

			// Register database connection pool telemetry metrics (PRD Section 16)
			if err := telemetry.RegisterDBStats(db.DB); err != nil {
				logger.Warn("failed registering db pool metrics", slog.String("error", err.Error()))
			}
		}
	} else {
		logger.Warn("DATABASE_URL not configured, running in ephemeral mode")
	}

	var ocrEngine ocr.OCREngine
	isGemini := strings.EqualFold(cfg.OCRProvider, "gemini_flash") ||
		strings.EqualFold(cfg.OCRProvider, "gemini") ||
		strings.EqualFold(cfg.OCRProvider, "gemini-flash")

	if isGemini {
		if cfg.GeminiAPIKey != "" {
			logger.Info("initializing Gemini Flash OCR provider", slog.String("model", cfg.GeminiModel))
			ocrEngine = providers.NewGeminiEngine(cfg.GeminiAPIKey, cfg.GeminiModel)
		} else {
			logger.Warn("Gemini OCR provider selected but GEMINI_API_KEY is empty, falling back to Mock OCR engine")
			ocrEngine = providers.NewMockEngine()
		}
	} else {
		logger.Info("initializing Mock OCR engine (deterministic fixtures)")
		ocrEngine = providers.NewMockEngine()
	}

	// Protect OCR engine with adaptive Circuit Breaker (PRD Phase 11.6)
	cbCfg := ocr.DefaultCircuitBreakerConfig()
	if isGemini {
		// Multimodal Vision AI network roundtrips require adequate headroom
		cbCfg.Timeout = 25 * time.Second
	}
	ocrEngine = ocr.NewCircuitBreaker(ocrEngine, cbCfg)
	logger.Info("initialized OCR circuit breaker protection",
		slog.Int("failure_threshold", cbCfg.FailureThreshold),
		slog.Duration("cooldown", cbCfg.Cooldown),
		slog.Duration("timeout", cbCfg.Timeout),
	)

	// Prefer the shared counter: it enforces the advertised plan limit across every
	// replica. The in-memory limiter only bounds one process, so with N replicas a
	// tenant could burst N times its plan; RATE_LIMIT_REPLICAS divides the limit as
	// a stopgap and is only needed when there is no database to share state through.
	var rateLimiter ratelimit.RateLimiter
	replicaDivisor := cfg.RateLimitReplicas
	var pgLimiter *ratelimit.PostgresLimiter
	if db != nil {
		pgLimiter = ratelimit.NewPostgresLimiter(db.DB, logger)
		rateLimiter = pgLimiter
		replicaDivisor = 1 // the counter is already cluster-wide
		logger.Info("rate limiting via shared postgres counter (cluster-wide)")
	} else {
		rateLimiter = ratelimit.NewLimiter()
		if replicaDivisor > 1 {
			logger.Warn("no database: rate limits are per-process and divided by the configured replica count",
				slog.Int("replicas", replicaDivisor))
		}
	}

	var usageRecorder *usage.Recorder
	if db != nil {
		usageRecorder = usage.NewRecorder(db, 1024)
	}

	var idempotencyStore idempotency.Store
	if db != nil {
		idempotencyStore = idempotency.NewPostgresStore(db.DB, idempotency.DefaultReplayTTL)
	} else {
		idempotencyStore = idempotency.NewMemoryStore(idempotency.DefaultReplayTTL)
	}

	// Cached OCR responses carry extracted identity fields; seal them before they
	// reach the database so the idempotency table never holds plaintext PII.
	idempotencySealer, err := idempotency.NewSealer([]byte(cfg.SessionSecret))
	if err != nil {
		logger.Error("failed initializing idempotency sealer; refusing to start with unencrypted response cache",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Transactional email. Registration and password reset are dead ends without
	// it, so production and staging refuse to start rather than accept signups
	// whose verification mail can never arrive.
	var emailSender email.Sender
	if cfg.SMTPHost != "" {
		smtpSender, err := email.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		if err != nil {
			logger.Error("invalid SMTP configuration", slog.String("error", err.Error()))
			os.Exit(1)
		}
		emailSender = smtpSender
		logger.Info("configured SMTP email sender",
			slog.String("host", cfg.SMTPHost), slog.String("from", cfg.SMTPFrom))
	} else if cfg.Env == "production" || cfg.Env == "staging" {
		logger.Error("SMTP_HOST must be set in production/staging: without it verification and password-reset mail is never delivered and self-service signup cannot complete")
		os.Exit(1)
	} else {
		logger.Warn("no SMTP_HOST configured; verification and reset links will be written to the log instead of sent")
		emailSender = &email.LogSender{Logger: logger}
	}

	// Configure session secret and validators
	auth.SetTokenSecret([]byte(cfg.SessionSecret))
	sessionVal := middleware.NewSessionTokenValidator([]byte(cfg.SessionSecret))
	if db != nil {
		// Enables server-side session revocation on logout.
		sessionVal.SetRevocationChecker(db)
	}

	// DevTokenValidator accepts unsigned JWTs, so it requires an explicit opt-in
	// and is refused outright in production/staging regardless of the flag.
	devAuthEnabled := cfg.EnableDevAuth && cfg.Env != "production" && cfg.Env != "staging"
	if cfg.EnableDevAuth && !devAuthEnabled {
		logger.Error("ENABLE_DEV_AUTH is set but ignored: unsigned dev tokens are never permitted in production/staging")
	}

	var oidcValidator middleware.TokenValidator
	if cfg.KeycloakJWKSURL != "" {
		logger.Info("initializing Keycloak OIDC validator", slog.String("jwks_url", cfg.KeycloakJWKSURL))
		validator := middleware.NewOIDCValidator(cfg.KeycloakJWKSURL, nil)
		if cfg.KeycloakIssuer != "" {
			validator.SetExpectedIssuer(cfg.KeycloakIssuer)
		}
		if cfg.KeycloakAudience != "" {
			validator.SetExpectedAudience(cfg.KeycloakAudience)
		}
		if devAuthEnabled {
			oidcValidator = middleware.NewCompositeTokenValidator(validator, sessionVal, middleware.NewDevTokenValidator())
		} else {
			oidcValidator = middleware.NewCompositeTokenValidator(validator, sessionVal)
		}
	} else if devAuthEnabled {
		logger.Warn("ENABLE_DEV_AUTH is on: DevTokenValidator accepts UNSIGNED tokens. Never enable this outside local development")
		oidcValidator = middleware.NewCompositeTokenValidator(sessionVal, middleware.NewDevTokenValidator())
	} else {
		oidcValidator = sessionVal
	}

	var authorizer authz.Authorizer
	if cfg.SpiceDBEndpoint != "" {
		logger.Info("initializing SpiceDB authorizer", slog.String("endpoint", cfg.SpiceDBEndpoint))
		spicedbClient := authz.NewClient(cfg.SpiceDBEndpoint, cfg.SpiceDBPresharedKey, nil)
		authorizer = spicedbClient

		// Bootstrap Zanzibar schema if schema file is readable
		schemaPath := "infra/spicedb/schema.zed"
		if schemaBytes, err := os.ReadFile(schemaPath); err == nil {
			initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := spicedbClient.WriteSchema(initCtx, string(schemaBytes)); err != nil {
				logger.Warn("could not bootstrap SpiceDB schema", slog.String("error", err.Error()))
			} else {
				logger.Info("bootstrapped SpiceDB Zanzibar schema successfully")
			}
			cancel()
		}
	}

	// db is a *store.DB, which is nil-typed rather than nil when the pool failed
	// to open; assign through an interface variable so the handler's nil check works.
	var sessionRevoker handlers.SessionRevoker
	if db != nil {
		sessionRevoker = db
	}

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		Pinger:                  pinger,
		KeyStore:                db,
		OCREngine:               ocrEngine,
		OCRStore:                db,
		UsageStore:              db,
		AccountStore:            db,
		AuditStore:              db,
		RateLimiter:             rateLimiter,
		UsageRecorder:           usageRecorder,
		IdempotencyStore:        idempotencyStore,
		IdempotencySealer:       idempotencySealer,
		EmailSender:             emailSender,
		AppBaseURL:              cfg.AppBaseURL,
		SessionRevoker:          sessionRevoker,
		SessionCacheInvalidator: sessionVal,
		OIDCValidator:           oidcValidator,
		Authorizer:              authorizer,
		CORSAllowedOrigins:      cfg.CORSAllowedOrigins,
		RateLimitReplicas:       replicaDivisor,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Initialize background cleaner for stale rate limiter buckets and expired idempotency keys (Issue #4)
	cleanerCtx, cleanerCancel := context.WithCancel(context.Background())
	defer cleanerCancel()
	var purger retentionPurger
	if db != nil {
		purger = db
	}
	cleanerDone := startBackgroundCleaner(cleanerCtx, logger, 10*time.Minute, rateLimiter, idempotencyStore, purger, pgLimiter)

	// Server runner goroutine
	go func() {
		logger.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for OS termination signals
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	sig := <-shutdownChan
	logger.Info("shutdown signal received", slog.String("signal", sig.String()))

	// Stop background cleaner
	cleanerCancel()
	<-cleanerDone

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful server shutdown failed", slog.String("error", err.Error()))
	} else {
		logger.Info("server exited cleanly")
	}

	if usageRecorder != nil {
		if err := usageRecorder.Close(shutdownCtx); err != nil {
			logger.Error("error draining usage recorder", slog.String("error", err.Error()))
		}
	}

	if db != nil {
		if err := db.Close(); err != nil {
			logger.Error("error closing database pool", slog.String("error", err.Error()))
		}
	}

	if tel != nil {
		if err := tel.Shutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down telemetry", slog.String("error", err.Error()))
		}
	}
}
