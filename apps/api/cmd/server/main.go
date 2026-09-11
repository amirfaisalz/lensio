package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/config"
	internalhttp "github.com/amirfaisalz/nusaid/apps/api/internal/http"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/services/ocr"
	"github.com/amirfaisalz/nusaid/services/ocr/providers"
)

func main() {
	// Initialize structured JSON logging (Ponytail standard library log/slog)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("starting nusaid api service",
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
		}
	} else {
		logger.Warn("DATABASE_URL not configured, running in ephemeral mode")
	}

	var ocrEngine ocr.OCREngine
	if cfg.OCRProvider == "gemini_flash" && cfg.GeminiAPIKey != "" {
		logger.Info("initializing Gemini Flash OCR provider", slog.String("model", cfg.GeminiModel))
		ocrEngine = providers.NewGeminiEngine(cfg.GeminiAPIKey, cfg.GeminiModel)
	} else {
		logger.Info("initializing Mock OCR engine (deterministic fixtures)")
		ocrEngine = providers.NewMockEngine()
	}

	router := internalhttp.NewRouter(pinger, db, ocrEngine, db)

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

	if db != nil {
		if err := db.Close(); err != nil {
			logger.Error("error closing database pool", slog.String("error", err.Error()))
		}
	}
}
