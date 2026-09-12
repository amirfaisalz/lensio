package idempotency

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStore provides a thread-safe, in-memory implementation of Store.
// Operations have O(1) time complexity and O(K) space complexity where K is the number of active keys.
type MemoryStore struct {
	mu         sync.RWMutex
	records    map[string]*Record
	defaultTTL time.Duration
	nowFunc    func() time.Time
}

// NewMemoryStore initializes an in-memory idempotency store with the given default TTL.
func NewMemoryStore(defaultTTL time.Duration) *MemoryStore {
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &MemoryStore{
		records:    make(map[string]*Record),
		defaultTTL: defaultTTL,
		nowFunc:    time.Now,
	}
}

func makeStorageKey(orgID, key string) string {
	return fmt.Sprintf("%s:%s", orgID, key)
}

// LockOrGet attempts to atomically acquire the idempotency key for an organization.
func (m *MemoryStore) LockOrGet(_ context.Context, orgID, key, requestHash string, ttl time.Duration) (*Record, bool, error) {
	if orgID == "" {
		return nil, false, ErrOrgIDRequired
	}
	if key == "" {
		return nil, false, ErrKeyRequired
	}
	if ttl <= 0 {
		ttl = m.defaultTTL
	}

	storageKey := makeStorageKey(orgID, key)
	now := m.nowFunc()

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, found := m.records[storageKey]; found {
		if !now.After(existing.ExpiresAt) {
			// Record is still valid
			copyRec := *existing
			return &copyRec, false, nil
		}
		// Record has expired, allow overwrite
	}

	newRec := &Record{
		Key:         key,
		OrgID:       orgID,
		RequestHash: requestHash,
		Status:      StatusInProgress,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	m.records[storageKey] = newRec

	copyRec := *newRec
	return &copyRec, true, nil
}

// Complete updates an in-progress record to completed status with the cached response data.
func (m *MemoryStore) Complete(_ context.Context, orgID, key string, statusCode int, headers map[string]string, body []byte) error {
	if orgID == "" {
		return ErrOrgIDRequired
	}
	if key == "" {
		return ErrKeyRequired
	}

	storageKey := makeStorageKey(orgID, key)

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, found := m.records[storageKey]
	if !found {
		return nil
	}

	headerCopy := make(map[string]string, len(headers))
	for k, v := range headers {
		headerCopy[k] = v
	}

	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	rec.Status = StatusCompleted
	rec.StatusCode = statusCode
	rec.Headers = headerCopy
	rec.ResponseBody = bodyCopy

	return nil
}

// Release removes an in-progress record if processing failed with a transient error.
func (m *MemoryStore) Release(_ context.Context, orgID, key string) error {
	if orgID == "" {
		return ErrOrgIDRequired
	}
	if key == "" {
		return ErrKeyRequired
	}

	storageKey := makeStorageKey(orgID, key)

	m.mu.Lock()
	defer m.mu.Unlock()

	if rec, found := m.records[storageKey]; found && rec.Status == StatusInProgress {
		delete(m.records, storageKey)
	}

	return nil
}

// CleanupStale removes all records that have passed their expiration timestamp.
func (m *MemoryStore) CleanupStale() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFunc()
	removed := 0
	for k, rec := range m.records {
		if now.After(rec.ExpiresAt) {
			delete(m.records, k)
			removed++
		}
	}
	return removed
}

// Count returns the current number of cached records in memory.
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.records)
}
