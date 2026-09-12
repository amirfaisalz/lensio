package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// KeyToucher throttles and pools asynchronous updates to API key last_used_at timestamps.
// This prevents database connection pool exhaustion under high burst traffic (e.g. 1,000-5,000 RPS).
type KeyToucher struct {
	mu          sync.Mutex
	lastTouched map[string]time.Time
	interval    time.Duration
	sem         chan struct{}
}

// NewKeyToucher creates a new KeyToucher with specified throttle interval and max concurrent database writes.
func NewKeyToucher(interval time.Duration, maxConcurrency int) *KeyToucher {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	return &KeyToucher{
		lastTouched: make(map[string]time.Time),
		interval:    interval,
		sem:         make(chan struct{}, maxConcurrency),
	}
}

var defaultKeyToucher = NewKeyToucher(5*time.Minute, 10)

// Touch updates last_used_at for keyID if it has not been updated within the throttle interval.
// Returns true if an update was dispatched, or false if throttled or pool is saturated.
func (t *KeyToucher) Touch(ctx context.Context, keyStore store.APIKeyStore, keyID string) bool {
	if keyStore == nil || keyID == "" {
		return false
	}

	now := time.Now()

	t.mu.Lock()
	if last, ok := t.lastTouched[keyID]; ok && now.Sub(last) < t.interval {
		t.mu.Unlock()
		return false
	}
	t.lastTouched[keyID] = now

	// Prune expired entries if map grows to prevent unbounded memory growth
	if len(t.lastTouched) > 2000 {
		for k, ts := range t.lastTouched {
			if now.Sub(ts) >= t.interval {
				delete(t.lastTouched, k)
			}
		}
	}
	t.mu.Unlock()

	// Acquire semaphore without blocking; if busy, drop touch to protect DB pool
	select {
	case t.sem <- struct{}{}:
		go func() {
			defer func() { <-t.sem }()
			touchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = keyStore.TouchAPIKeyLastUsed(touchCtx, keyID, now)
		}()
		return true
	default:
		return false
	}
}

// Reset clears the throttle memory cache (useful for tests).
func (t *KeyToucher) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastTouched = make(map[string]time.Time)
}

// touchKeyAsync is the internal helper invoked by Authenticate and DualAuth middlewares.
func touchKeyAsync(ctx context.Context, keyStore store.APIKeyStore, keyID string) {
	defaultKeyToucher.Touch(ctx, keyStore, keyID)
}
