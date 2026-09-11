package usage_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/apps/api/internal/usage"
)

type mockUsageStore struct {
	mu      sync.Mutex
	records []*store.UsageRecord
	fail    bool
}

func (m *mockUsageStore) CreateUsageRecord(ctx context.Context, rec *store.UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("mock db error")
	}
	m.records = append(m.records, rec)
	return nil
}

func (m *mockUsageStore) GetUsageSummary(ctx context.Context, orgID string, since time.Time, planQuota int, cycleReset time.Time) (*store.UsageSummary, error) {
	return nil, nil
}

func (m *mockUsageStore) GetDailyUsage(ctx context.Context, orgID string, since time.Time) ([]store.DailyUsage, error) {
	return nil, nil
}

func (m *mockUsageStore) GetEndpointUsage(ctx context.Context, orgID string, since time.Time) ([]store.EndpointUsage, error) {
	return nil, nil
}

func (m *mockUsageStore) GetMonthlyOCRCount(ctx context.Context, orgID string, since time.Time) (int, error) {
	return 0, nil
}

func (m *mockUsageStore) GetUsageRecords(ctx context.Context, orgID string, filter store.UsageRecordFilter) ([]store.UsageRecord, int, error) {
	return nil, 0, nil
}

func TestRecorder_RecordAndDrain(t *testing.T) {
	mockStore := &mockUsageStore{}
	recorder := usage.NewRecorder(mockStore, 100)

	// Record nil safety
	recorder.Record(nil)

	// Record valid items
	count := 10
	for i := 0; i < count; i++ {
		recorder.Record(&store.UsageRecord{
			OrgID:      "org-1",
			RequestID:  "req-1",
			Endpoint:   "/api/v1/ocr/ktp",
			StatusCode: 200,
			LatencyMS:  100,
			Timestamp:  time.Now(),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("failed closing recorder: %v", err)
	}

	mockStore.mu.Lock()
	saved := len(mockStore.records)
	mockStore.mu.Unlock()

	if saved != count {
		t.Fatalf("expected %d saved records, got %d", count, saved)
	}

	// Record after close should be dropped gracefully
	recorder.Record(&store.UsageRecord{RequestID: "post-close"})
}

func TestRecorder_ChannelSaturation(t *testing.T) {
	mockStore := &mockUsageStore{}
	// Buffer size 1 to easily test saturation
	recorder := usage.NewRecorder(mockStore, 1)

	for i := 0; i < 50; i++ {
		recorder.Record(&store.UsageRecord{
			OrgID:     "org-1",
			RequestID: "req-drop",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestRecorder_StoreErrorLogging(t *testing.T) {
	mockStore := &mockUsageStore{fail: true}
	recorder := usage.NewRecorder(mockStore, 10)

	recorder.Record(&store.UsageRecord{
		OrgID:     "org-1",
		RequestID: "req-fail",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestRecorder_CloseTimeout(t *testing.T) {
	recorder := usage.NewRecorder(nil, 10)

	// Close with already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// May return context error or nil if worker closed quickly
	_ = recorder.Close(ctx)
}
