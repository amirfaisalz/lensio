package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// retentionPurger deletes rows past the retention windows published in PRIVACY.md.
type retentionPurger interface {
	PurgeExpiredRecords(ctx context.Context, now time.Time, batchSize int) (store.RetentionResult, error)
}

// rateWindowSweeper deletes elapsed shared rate-limit windows.
type rateWindowSweeper interface {
	CleanupStale(ctx context.Context) (int64, error)
}

// sweepRateWindows removes rate-limit rows whose window has passed, so the table
// stays proportional to active tenants rather than to uptime.
func sweepRateWindows(ctx context.Context, logger *slog.Logger, sweeper rateWindowSweeper) {
	if sweeper == nil {
		return
	}
	sweepCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := sweeper.CleanupStale(sweepCtx)
	if err != nil {
		if logger != nil {
			logger.Warn("failed sweeping elapsed rate limit windows", slog.String("error", err.Error()))
		}
		return
	}
	if rows > 0 && logger != nil {
		logger.Debug("purged elapsed rate limit windows", slog.Int64("rows", rows))
	}
}

// retentionInterval is how often the retention sweep runs. Retention is measured
// in months, so an hourly sweep is ample and keeps each batch small.
const retentionInterval = 1 * time.Hour

// startBackgroundCleaner initiates periodic purging of stale rate limiter buckets and expired idempotency keys.
// It gracefully exits when ctx is cancelled and signals completion via the returned channel.
func startBackgroundCleaner(
	ctx context.Context,
	logger *slog.Logger,
	interval time.Duration,
	rateLimiter ratelimit.RateLimiter,
	idempStore idempotency.Store,
	purger retentionPurger,
	windowSweeper rateWindowSweeper,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		retentionTicker := time.NewTicker(retentionInterval)
		defer retentionTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup(ctx, logger, interval, rateLimiter, idempStore)
				sweepRateWindows(ctx, logger, windowSweeper)
			case <-retentionTicker.C:
				runRetention(ctx, logger, purger)
			}
		}
	}()
	return done
}

// runRetention deletes records past their published retention window. PRIVACY.md
// promises 90 days for OCR metadata and 12 months for usage and audit records;
// nothing enforced that until this sweep existed.
func runRetention(ctx context.Context, logger *slog.Logger, purger retentionPurger) {
	if purger == nil {
		return
	}

	purgeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := purger.PurgeExpiredRecords(purgeCtx, time.Now(), 5000)
	if err != nil {
		if logger != nil {
			logger.Error("retention sweep failed", slog.String("error", err.Error()))
		}
		return
	}
	if res.Total() > 0 && logger != nil {
		logger.Info("purged records past their retention window",
			slog.Int64("ocr_requests", res.OCRRequests),
			slog.Int64("usage_records", res.UsageRecords),
			slog.Int64("audit_logs", res.AuditLogs),
		)
	}
}

// runCleanup executes a single sweep of stale rate limiter buckets and expired idempotency records.
func runCleanup(ctx context.Context, logger *slog.Logger, maxIdle time.Duration, rateLimiter ratelimit.RateLimiter, idempStore idempotency.Store) {
	// Only the in-memory limiter accumulates per-key state that needs pruning.
	if mem, ok := rateLimiter.(*ratelimit.Limiter); ok && mem != nil {
		if removed := mem.CleanupStale(maxIdle); removed > 0 && logger != nil {
			logger.Debug("purged stale rate limit buckets", slog.Int("count", removed))
		}
	}

	if idempStore != nil {
		switch s := idempStore.(type) {
		case *idempotency.MemoryStore:
			if removed := s.CleanupStale(); removed > 0 && logger != nil {
				logger.Debug("purged stale memory idempotency keys", slog.Int("count", removed))
			}
		case *idempotency.PostgresStore:
			cleanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if rows, err := s.CleanupStale(cleanCtx); err == nil && rows > 0 && logger != nil {
				logger.Debug("purged stale postgres idempotency keys", slog.Int64("rows", rows))
			}
			cancel()
		}
	}
}
