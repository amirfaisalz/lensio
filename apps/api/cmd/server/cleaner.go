package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
)

// startBackgroundCleaner initiates periodic purging of stale rate limiter buckets and expired idempotency keys.
// It gracefully exits when ctx is cancelled and signals completion via the returned channel.
func startBackgroundCleaner(
	ctx context.Context,
	logger *slog.Logger,
	interval time.Duration,
	rateLimiter *ratelimit.Limiter,
	idempStore idempotency.Store,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup(ctx, logger, interval, rateLimiter, idempStore)
			}
		}
	}()
	return done
}

// runCleanup executes a single sweep of stale rate limiter buckets and expired idempotency records.
func runCleanup(ctx context.Context, logger *slog.Logger, maxIdle time.Duration, rateLimiter *ratelimit.Limiter, idempStore idempotency.Store) {
	if rateLimiter != nil {
		if removed := rateLimiter.CleanupStale(maxIdle); removed > 0 && logger != nil {
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
