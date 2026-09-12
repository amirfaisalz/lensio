package middleware_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type countingKeyStore struct {
	mu         sync.Mutex
	touchCount int64
	touchedIDs map[string]int
}

func newCountingKeyStore() *countingKeyStore {
	return &countingKeyStore{
		touchedIDs: make(map[string]int),
	}
}

func (c *countingKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	return nil
}
func (c *countingKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	return nil, store.ErrNotFound
}
func (c *countingKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	return nil, nil
}
func (c *countingKeyStore) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	return nil
}
func (c *countingKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	atomic.AddInt64(&c.touchCount, 1)
	c.mu.Lock()
	c.touchedIDs[keyID]++
	c.mu.Unlock()
	return nil
}

func TestKeyToucher_Throttling(t *testing.T) {
	kStore := newCountingKeyStore()
	toucher := middleware.NewKeyToucher(100*time.Millisecond, 5)

	// First touch: should dispatch
	dispatched := toucher.Touch(context.Background(), kStore, "key-1")
	if !dispatched {
		t.Fatal("expected first touch to be dispatched")
	}

	// Immediate second touch for same key: should be throttled
	dispatched2 := toucher.Touch(context.Background(), kStore, "key-1")
	if dispatched2 {
		t.Fatal("expected immediate second touch to be throttled")
	}

	// Touch for different key: should be dispatched
	dispatchedDiff := toucher.Touch(context.Background(), kStore, "key-2")
	if !dispatchedDiff {
		t.Fatal("expected different key touch to be dispatched")
	}

	// Wait for goroutines to complete
	time.Sleep(30 * time.Millisecond)

	kStore.mu.Lock()
	count1 := kStore.touchedIDs["key-1"]
	count2 := kStore.touchedIDs["key-2"]
	kStore.mu.Unlock()

	if count1 != 1 {
		t.Errorf("expected key-1 touched exactly once, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected key-2 touched exactly once, got %d", count2)
	}

	// Wait for interval to pass
	time.Sleep(110 * time.Millisecond)

	// After interval, touching key-1 should dispatch again
	dispatchedAfter := toucher.Touch(context.Background(), kStore, "key-1")
	if !dispatchedAfter {
		t.Fatal("expected touch after interval to be dispatched")
	}
}

func TestKeyToucher_Saturation(t *testing.T) {
	// Create toucher with maxConcurrency = 1 and slow store
	toucher := middleware.NewKeyToucher(1*time.Millisecond, 1)

	slowStore := &countingKeyStore{
		touchedIDs: make(map[string]int),
	}

	var blockChan = make(chan struct{})
	blockStore := &blockingKeyStore{
		countingKeyStore: slowStore,
		block:            blockChan,
	}

	// First touch acquires the single slot and blocks
	toucher.Touch(context.Background(), blockStore, "key-slow-1")

	// Wait for worker goroutine to enter TouchAPIKeyLastUsed
	time.Sleep(10 * time.Millisecond)

	// Second touch for different key should be dropped due to saturation
	dropped := toucher.Touch(context.Background(), blockStore, "key-slow-2")
	if dropped {
		t.Fatal("expected second touch to be dropped when semaphore is saturated")
	}

	// Unblock first worker
	close(blockChan)
	time.Sleep(20 * time.Millisecond)
}

type blockingKeyStore struct {
	*countingKeyStore
	block chan struct{}
}

func (b *blockingKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	<-b.block
	return b.countingKeyStore.TouchAPIKeyLastUsed(ctx, keyID, lastUsed)
}

func TestKeyToucher_NilAndEmpty(t *testing.T) {
	toucher := middleware.NewKeyToucher(0, 0)
	if toucher.Touch(context.Background(), nil, "key-1") {
		t.Error("expected false for nil store")
	}
	if toucher.Touch(context.Background(), newCountingKeyStore(), "") {
		t.Error("expected false for empty keyID")
	}
	toucher.Reset()
}
