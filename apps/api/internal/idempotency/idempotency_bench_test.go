package idempotency_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
)

// BenchmarkMemoryStore_LockOrGet evaluates the O(1) performance of in-memory idempotency locking.
func BenchmarkMemoryStore_LockOrGet(b *testing.B) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()
	orgID := "org-bench-1"
	hash := "bench-hash-1234567890abcdef"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%1000)
		_, _, _ = store.LockOrGet(ctx, orgID, key, hash, time.Hour)
	}
}

// BenchmarkMemoryStore_Complete evaluates the completion and serialization throughput.
func BenchmarkMemoryStore_Complete(b *testing.B) {
	store := idempotency.NewMemoryStore(time.Hour)
	ctx := context.Background()
	orgID := "org-bench-1"
	key := "key-bench-complete"
	hash := "bench-hash-complete"
	_, _, _ = store.LockOrGet(ctx, orgID, key, hash, time.Hour)

	headers := map[string]string{"Content-Type": "application/json"}
	body := []byte(`{"id":"req-123","status":"completed","confidence":0.99}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = store.Complete(ctx, orgID, key, 200, headers, body)
	}
}

// BenchmarkComputePayloadHash measures the O(N) streaming SHA-256 hashing overhead on a simulated 50KB payload.
func BenchmarkComputePayloadHash(b *testing.B) {
	payload := bytes.Repeat([]byte("Indonesian KTP synthetic OCR image payload bytes simulate 50kb..."), 700)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
		_, _ = idempotency.ComputePayloadHash(req, int64(len(payload)+1024))
	}
}
