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

	"github.com/amirfaisalz/lensio/apps/api/internal/config"
	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
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

	rateLimiter := ratelimit.NewLimiter()

	var usageRecorder *usage.Recorder
	if db != nil {
		usageRecorder = usage.NewRecorder(db, 1024)
	}

	var idempotencyStore idempotency.Store
	if db != nil {
		idempotencyStore = idempotency.NewPostgresStore(db.DB, 24*time.Hour)
	} else {
		idempotencyStore = idempotency.NewMemoryStore(24 * time.Hour)
	}

	var oidcValidator middleware.TokenValidator
	if cfg.KeycloakJWKSURL != "" {
		logger.Info("initializing Keycloak OIDC validator", slog.String("jwks_url", cfg.KeycloakJWKSURL))
		oidcValidator = middleware.NewOIDCValidator(cfg.KeycloakJWKSURL, nil)
	}

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		Pinger:           pinger,
		KeyStore:         db,
		OCREngine:        ocrEngine,
		OCRStore:         db,
		UsageStore:       db,
		AccountStore:     db,
		AuditStore:       db,
		RateLimiter:      rateLimiter,
		UsageRecorder:    usageRecorder,
		IdempotencyStore: idempotencyStore,
		OIDCValidator:    oidcValidator,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
