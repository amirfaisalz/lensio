package ratelimit

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// WindowSize is the fixed rate-limit window. Plan limits are expressed per
// minute, so the window matches them exactly.
const WindowSize = time.Minute

var _ RateLimiter = (*PostgresLimiter)(nil)

// PostgresLimiter enforces a limit shared by every API replica.
//
// It is a fixed-window counter: one row per key per minute, incremented by a
// single INSERT .. ON CONFLICT DO UPDATE .. RETURNING. That is atomic across
// replicas without a transaction, which matters because this runs on every
// request.
//
// Known ceiling: a fixed window permits up to 2x the limit across a window
// boundary (all of one minute's allowance late, all of the next early). A
// sliding window or a distributed token bucket removes that, at the cost of
// either two rows per check or a Lua-scripted store. The 2x boundary burst is a
// far smaller error than the Nx replica multiplication it replaces.
type PostgresLimiter struct {
	db *sql.DB
	// fallback absorbs database failures. Losing the limiter entirely would let
	// an outage become an open door, while failing requests closed would turn a
	// limiter blip into an outage; a per-replica bucket is the middle ground.
	fallback *Limiter
	nowFunc  func() time.Time
	logger   *slog.Logger
}

// NewPostgresLimiter creates a cluster-wide limiter backed by db.
func NewPostgresLimiter(db *sql.DB, logger *slog.Logger) *PostgresLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostgresLimiter{
		db:       db,
		fallback: NewLimiter(),
		nowFunc:  time.Now,
		logger:   logger,
	}
}

// Allow records one request against key and reports whether it is permitted.
// Complexity: one indexed upsert, O(1) time and space per call.
func (l *PostgresLimiter) Allow(key string, limitPerMinute int) Result {
	if limitPerMinute <= 0 {
		limitPerMinute = 10
	}
	if l == nil || l.db == nil {
		return l.fallbackAllow(key, limitPerMinute)
	}

	now := l.nowFunc().UTC()
	windowStart := now.Truncate(WindowSize)
	resetTime := windowStart.Add(WindowSize)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var hits int
	err := l.db.QueryRowContext(ctx, `
		INSERT INTO rate_limit_counters (bucket_key, window_start, hits)
		VALUES ($1, $2, 1)
		ON CONFLICT (bucket_key, window_start)
		DO UPDATE SET hits = rate_limit_counters.hits + 1
		RETURNING hits;`,
		key, windowStart,
	).Scan(&hits)
	if err != nil {
		l.logger.Warn("rate limit counter unavailable; falling back to per-replica limiting",
			slog.String("error", err.Error()))
		return l.fallbackAllow(key, limitPerMinute)
	}

	remaining := limitPerMinute - hits
	if remaining < 0 {
		remaining = 0
	}

	if hits > limitPerMinute {
		retryAfter := int(time.Until(resetTime).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		return Result{
			Allowed:    false,
			Limit:      limitPerMinute,
			Remaining:  0,
			ResetTime:  resetTime.Unix(),
			RetryAfter: retryAfter,
		}
	}

	return Result{
		Allowed:   true,
		Limit:     limitPerMinute,
		Remaining: remaining,
		ResetTime: resetTime.Unix(),
	}
}

func (l *PostgresLimiter) fallbackAllow(key string, limitPerMinute int) Result {
	if l == nil || l.fallback == nil {
		return Result{Allowed: true, Limit: limitPerMinute, Remaining: limitPerMinute}
	}
	return l.fallback.Allow(key, limitPerMinute)
}

// CleanupStale removes windows that have already elapsed.
func (l *PostgresLimiter) CleanupStale(ctx context.Context) (int64, error) {
	if l == nil || l.db == nil {
		return 0, nil
	}
	cutoff := l.nowFunc().UTC().Add(-2 * WindowSize)
	res, err := l.db.ExecContext(ctx,
		`DELETE FROM rate_limit_counters WHERE window_start < $1;`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
