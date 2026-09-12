package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Result contains the rate limiting evaluation decision and standard RFC header values.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetTime  int64 // Unix timestamp (seconds)
	RetryAfter int   // Seconds to wait before retry (when Allowed is false)
}

// RateLimiter defines the pluggable contract for evaluating rate limits across local and distributed topologies.
type RateLimiter interface {
	Allow(key string, limitPerMinute int) Result
}

var _ RateLimiter = (*Limiter)(nil)

type bucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// Limiter manages thread-safe, in-memory token bucket rate limiters per entity key (e.g. orgID).
type Limiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	nowFunc func() time.Time
}

// NewLimiter initializes an in-memory token bucket rate limiter.
func NewLimiter() *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		nowFunc: time.Now,
	}
}

// Allow evaluates whether an incoming request from the specified key is allowed under limitPerMinute.
// Complexity: O(1) time complexity, O(1) auxiliary space.
func (l *Limiter) Allow(key string, limitPerMinute int) Result {
	if limitPerMinute <= 0 {
		limitPerMinute = 10 // fallback default free tier
	}

	now := l.nowFunc()

	b := l.getOrCreateBucket(key, limitPerMinute, now)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on elapsed duration
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(b.capacity, b.tokens+(elapsed*b.refillRate))
		b.lastRefill = now
	}

	// Calculate reset time (when bucket will be back at full capacity)
	tokensNeeded := b.capacity - b.tokens
	secondsToFull := int64(math.Ceil(tokensNeeded / b.refillRate))
	if secondsToFull <= 0 {
		secondsToFull = 60
	}
	resetTime := now.Unix() + secondsToFull

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return Result{
			Allowed:    true,
			Limit:      limitPerMinute,
			Remaining:  int(math.Floor(b.tokens)),
			ResetTime:  resetTime,
			RetryAfter: 0,
		}
	}

	// Rate limit exceeded: calculate seconds until at least 1 token is available
	missing := 1.0 - b.tokens
	retryAfter := int(math.Ceil(missing / b.refillRate))
	if retryAfter < 1 {
		retryAfter = 1
	}

	return Result{
		Allowed:    false,
		Limit:      limitPerMinute,
		Remaining:  0,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
	}
}

// Reset clears state for a specific key (useful for tests or administrative quota resets).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// CleanupStale removes buckets that have not been accessed within maxIdle duration to bound memory.
func (l *Limiter) CleanupStale(maxIdle time.Duration) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc()
	removed := 0
	for key, b := range l.buckets {
		b.mu.Lock()
		idle := now.Sub(b.lastRefill)
		b.mu.Unlock()

		if idle > maxIdle {
			delete(l.buckets, key)
			removed++
		}
	}

	return removed
}

func (l *Limiter) getOrCreateBucket(key string, limitPerMinute int, now time.Time) *bucket {
	l.mu.RLock()
	b, exists := l.buckets[key]
	l.mu.RUnlock()

	if exists {
		// Update capacity and rate if the plan changed
		b.mu.Lock()
		capFloat := float64(limitPerMinute)
		if b.capacity != capFloat {
			b.capacity = capFloat
			b.refillRate = capFloat / 60.0
			if b.tokens > b.capacity {
				b.tokens = b.capacity
			}
		}
		b.mu.Unlock()
		return b
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check under write lock
	if b, exists = l.buckets[key]; exists {
		return b
	}

	capFloat := float64(limitPerMinute)
	b = &bucket{
		tokens:     capFloat,
		capacity:   capFloat,
		refillRate: capFloat / 60.0,
		lastRefill: now,
	}
	l.buckets[key] = b
	return b
}
