package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/ratelimit"
)

func TestLimiter_Allow_Basic(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	key := "test-org-1"

	// Limit 3 req/min
	limit := 3

	// 1st request allowed
	res1 := limiter.Allow(key, limit)
	if !res1.Allowed || res1.Remaining != 2 || res1.Limit != 3 {
		t.Fatalf("unexpected res1: %+v", res1)
	}

	// 2nd request allowed
	res2 := limiter.Allow(key, limit)
	if !res2.Allowed || res2.Remaining != 1 {
		t.Fatalf("unexpected res2: %+v", res2)
	}

	// 3rd request allowed
	res3 := limiter.Allow(key, limit)
	if !res3.Allowed || res3.Remaining != 0 {
		t.Fatalf("unexpected res3: %+v", res3)
	}

	// 4th request rejected
	res4 := limiter.Allow(key, limit)
	if res4.Allowed {
		t.Fatalf("expected 4th request to be rejected, got %+v", res4)
	}
	if res4.Remaining != 0 || res4.RetryAfter <= 0 {
		t.Errorf("expected RetryAfter > 0, got %d", res4.RetryAfter)
	}
}

func TestLimiter_DefaultLimit(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	// limit <= 0 should fallback to 10
	res := limiter.Allow("test-org-default", 0)
	if !res.Allowed || res.Limit != 10 || res.Remaining != 9 {
		t.Fatalf("unexpected default limit res: %+v", res)
	}
}

func TestLimiter_PlanChange(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	key := "test-org-upgrade"

	// Start with 1 req/min and exhaust it
	res1 := limiter.Allow(key, 1)
	if !res1.Allowed {
		t.Fatal("expected 1st request allowed")
	}
	res2 := limiter.Allow(key, 1)
	if res2.Allowed {
		t.Fatal("expected 2nd request rejected on limit 1")
	}

	// Upgrade plan to 100 req/min
	res3 := limiter.Allow(key, 100)
	// After increasing capacity, tokens should be replenished or allow headroom
	if res3.Limit != 100 {
		t.Fatalf("expected limit to be updated to 100, got %d", res3.Limit)
	}
}

func TestLimiter_Reset(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	key := "test-org-reset"

	_ = limiter.Allow(key, 1)
	res := limiter.Allow(key, 1)
	if res.Allowed {
		t.Fatal("expected rejected")
	}

	limiter.Reset(key)

	resAfterReset := limiter.Allow(key, 1)
	if !resAfterReset.Allowed {
		t.Fatal("expected allowed after reset")
	}
}

func TestLimiter_CleanupStale(t *testing.T) {
	limiter := ratelimit.NewLimiter()

	_ = limiter.Allow("active-org", 10)
	_ = limiter.Allow("stale-org", 10)

	// Sleep tiny bit so idle duration > 0
	time.Sleep(10 * time.Millisecond)

	// Clean up with maxIdle of 5ms
	removed := limiter.CleanupStale(5 * time.Millisecond)
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	limiter := ratelimit.NewLimiter()
	key := "test-org-concurrent"
	limit := 50

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
		denied  int
	)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := limiter.Allow(key, limit)
			mu.Lock()
			if res.Allowed {
				allowed++
			} else {
				denied++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if allowed != limit {
		t.Fatalf("expected exactly %d allowed requests, got %d", limit, allowed)
	}
	if denied != 50 {
		t.Fatalf("expected exactly 50 denied requests, got %d", denied)
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := ratelimit.NewLimiter()
	key := "benchmark-org"
	limit := 1000000

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		limiter.Allow(key, limit)
	}
}

func BenchmarkRateLimiter_Allow_Parallel(b *testing.B) {
	limiter := ratelimit.NewLimiter()
	limit := 1000000

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		id := fmt.Sprintf("bench-org-%d", time.Now().UnixNano()%10)
		for pb.Next() {
			limiter.Allow(id, limit)
		}
	})
}
