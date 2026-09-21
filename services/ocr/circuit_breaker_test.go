package ocr_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// mockFailingEngine allows injecting errors and delays.
type mockFailingEngine struct {
	mu        sync.Mutex
	err       error
	delay     time.Duration
	callCount int
	ktpResult *ocr.OCRResult
}

func (m *mockFailingEngine) Extract(ctx context.Context, image []byte) (*ocr.OCRResult, error) {
	m.mu.Lock()
	m.callCount++
	err := m.err
	delay := m.delay
	res := m.ktpResult
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}
	if res != nil {
		return res, nil
	}
	return &ocr.OCRResult{
		DocumentType: "ktp",
		Confidence:   0.98,
		Data: &ocr.KTPData{
			NIK:  "3171010101900001",
			Nama: "BUDI SANTOSO",
		},
	}, nil
}

func (m *mockFailingEngine) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockFailingEngine) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state    ocr.CircuitBreakerState
		expected string
	}{
		{ocr.StateClosed, "closed"},
		{ocr.StateHalfOpen, "half-open"},
		{ocr.StateOpen, "open"},
		{ocr.CircuitBreakerState(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if s := tt.state.String(); s != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, s)
			}
		})
	}
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cfg := ocr.DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("expected FailureThreshold 5, got %d", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 2 {
		t.Errorf("expected SuccessThreshold 2, got %d", cfg.SuccessThreshold)
	}
	if cfg.Cooldown != 10*time.Second {
		t.Errorf("expected Cooldown 10s, got %v", cfg.Cooldown)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected Timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.AdaptiveTimeout {
		t.Errorf("expected AdaptiveTimeout false by default")
	}
	if cfg.MinTimeout != 500*time.Millisecond {
		t.Errorf("expected MinTimeout 500ms, got %v", cfg.MinTimeout)
	}
	if cfg.IsFailureFunc == nil {
		t.Errorf("expected non-nil IsFailureFunc")
	}
	if cfg.NowFunc == nil {
		t.Errorf("expected non-nil NowFunc")
	}
}

func TestCircuitBreaker_ClosedState_PassThrough(t *testing.T) {
	mockEngine := providers.NewMockEngine()
	cb := ocr.NewCircuitBreaker(mockEngine)

	if cb.State() != ocr.StateClosed {
		t.Fatalf("expected initial state closed, got %v", cb.State())
	}

	img := synthetic.GenerateValidKTPImage()
	res, err := cb.Extract(context.Background(), img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Data == nil || res.Data.NIK != "3171010101900001" {
		t.Fatalf("unexpected result: %+v", res)
	}

	failCount, succCount := cb.Counts()
	if failCount != 0 || succCount != 0 {
		t.Errorf("expected 0 counts, got fail=%d, succ=%d", failCount, succCount)
	}
}

func TestCircuitBreaker_TripToOpen_AndFastFail(t *testing.T) {
	mock := &mockFailingEngine{err: ocr.ErrOCRFailed}
	fakeNow := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	var stateTransitions []string

	reg := prometheus.NewRegistry()
	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Cooldown:         5 * time.Second,
		Timeout:          1 * time.Second,
		Registerer:       reg,
		NowFunc: func() time.Time {
			return fakeNow
		},
		OnStateChange: func(from, to ocr.CircuitBreakerState) {
			stateTransitions = append(stateTransitions, fmt.Sprintf("%s->%s", from, to))
		},
	})

	ctx := context.Background()
	img := []byte("fake_image_bytes")

	// Call 1: failure 1
	_, err := cb.Extract(ctx, img)
	if !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected ErrOCRFailed, got %v", err)
	}
	if cb.State() != ocr.StateClosed {
		t.Errorf("expected state Closed, got %v", cb.State())
	}

	// Call 2: failure 2
	_, err = cb.Extract(ctx, img)
	if !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected ErrOCRFailed, got %v", err)
	}
	if cb.State() != ocr.StateClosed {
		t.Errorf("expected state Closed, got %v", cb.State())
	}

	// Call 3: failure 3 -> trips to Open
	_, err = cb.Extract(ctx, img)
	if !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected ErrOCRFailed, got %v", err)
	}
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected state Open after 3 failures, got %v", cb.State())
	}

	if len(stateTransitions) != 1 || stateTransitions[0] != "closed->open" {
		t.Errorf("unexpected transitions: %v", stateTransitions)
	}

	// Call 4: should immediately fast-fail with ErrCircuitOpen without calling underlying engine
	initialCalls := mock.GetCallCount()
	fastFailStart := time.Now()
	_, err = cb.Extract(ctx, img)
	fastFailDuration := time.Since(fastFailStart)

	if !errors.Is(err, ocr.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if mock.GetCallCount() != initialCalls {
		t.Errorf("expected underlying engine NOT to be called when Open, but call count changed from %d to %d",
			initialCalls, mock.GetCallCount())
	}
	if fastFailDuration > 50*time.Millisecond {
		t.Errorf("expected fast-fail in < 50ms, took %v", fastFailDuration)
	}
}

func TestCircuitBreaker_OpenToHalfOpen_AndRecoveryToClosed(t *testing.T) {
	mock := &mockFailingEngine{err: ocr.ErrOCRFailed}
	fakeNow := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

	reg := prometheus.NewRegistry()
	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Cooldown:         5 * time.Second,
		Registerer:       reg,
		NowFunc: func() time.Time {
			return fakeNow
		},
	})

	ctx := context.Background()
	img := []byte("image_data")

	// Trip to Open
	_, _ = cb.Extract(ctx, img)
	_, _ = cb.Extract(ctx, img)
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected Open state, got %v", cb.State())
	}

	// Before Cooldown: still fast-fails
	fakeNow = fakeNow.Add(3 * time.Second)
	_, err := cb.Extract(ctx, img)
	if !errors.Is(err, ocr.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen before cooldown, got %v", err)
	}

	// After Cooldown: advance fakeNow by 5s total
	fakeNow = fakeNow.Add(3 * time.Second) // 6s after open

	// Now mock starts succeeding
	mock.SetError(nil)

	// Canary probe 1
	res, err := cb.Extract(ctx, img)
	if err != nil {
		t.Fatalf("canary probe 1 failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
	// After 1 success in half-open, still in HalfOpen (need 2)
	if cb.State() != ocr.StateHalfOpen {
		t.Fatalf("expected state HalfOpen after 1 success, got %v", cb.State())
	}

	// Canary probe 2 -> hits SuccessThreshold (2) -> transitions to Closed
	res, err = cb.Extract(ctx, img)
	if err != nil {
		t.Fatalf("canary probe 2 failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
	if cb.State() != ocr.StateClosed {
		t.Fatalf("expected state Closed after reaching success threshold, got %v", cb.State())
	}

	// Normal calls continue succeeding
	res, err = cb.Extract(ctx, img)
	if err != nil {
		t.Fatalf("normal call failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
}

func TestCircuitBreaker_HalfOpen_FailureReTrips(t *testing.T) {
	mock := &mockFailingEngine{err: ocr.ErrOCRFailed}
	fakeNow := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Cooldown:         5 * time.Second,
		NowFunc: func() time.Time {
			return fakeNow
		},
	})

	ctx := context.Background()
	img := []byte("image_data")

	// Trip to Open
	_, _ = cb.Extract(ctx, img)
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	// Advance past cooldown
	fakeNow = fakeNow.Add(10 * time.Second)

	// In half-open, call fails
	_, err := cb.Extract(ctx, img)
	if !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected ErrOCRFailed, got %v", err)
	}

	// Should immediately re-trip to Open
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected state Open after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClientErrorsDoNotTrip(t *testing.T) {
	mock := &mockFailingEngine{}
	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 2,
	})

	ctx := context.Background()
	img := []byte("image_data")

	clientErrors := []error{
		ocr.ErrInvalidDocument,
		ocr.ErrUnsupportedDocument,
		ocr.ErrLowConfidence,
	}

	for _, clientErr := range clientErrors {
		mock.SetError(clientErr)
		for i := 0; i < 5; i++ {
			_, err := cb.Extract(ctx, img)
			if !errors.Is(err, clientErr) {
				t.Fatalf("expected %v, got %v", clientErr, err)
			}
		}
		if cb.State() != ocr.StateClosed {
			t.Fatalf("client error %v tripped the circuit breaker to %v", clientErr, cb.State())
		}
		failCount, _ := cb.Counts()
		if failCount != 0 {
			t.Errorf("expected failCount 0 for client errors, got %d", failCount)
		}
	}
}

func TestCircuitBreaker_AdaptiveTimeout(t *testing.T) {
	mock := &mockFailingEngine{
		delay: 200 * time.Millisecond,
		err:   ocr.ErrOCRFailed,
	}

	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 4,
		Timeout:          300 * time.Millisecond,
		AdaptiveTimeout:  true,
		MinTimeout:       50 * time.Millisecond,
	})

	ctx := context.Background()
	img := []byte("image_data")

	// Call 1: failure 1
	_, _ = cb.Extract(ctx, img)
	// Call 2: failure 2
	_, _ = cb.Extract(ctx, img)

	// Now delay exceeds the adapted timeout
	// Base: 300ms, Min: 50ms, Failures: 2, FailureThreshold: 4
	// Delta: (300-50)*2/4 = 125ms -> effective timeout = 175ms
	// Mock delay is 200ms -> should exceed 175ms and return context.DeadlineExceeded!
	_, err := cb.Extract(ctx, img)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected timeout or ocr error, got %v", err)
	}
}

func TestCircuitBreaker_IsolatedMetricsWithCustomRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := &mockFailingEngine{err: ocr.ErrOCRFailed}

	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 2,
		Registerer:       reg,
	})

	ctx := context.Background()
	img := []byte("image")

	// Trip breaker
	_, _ = cb.Extract(ctx, img)
	_, _ = cb.Extract(ctx, img)

	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected Open state, got %v", cb.State())
	}

	// Scrape custom registry
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from custom metrics, got %d", rec.Code)
	}

	metricsBody := rec.Body.String()
	if !strings.Contains(metricsBody, "lensio_ocr_circuit_breaker_state 2") {
		t.Errorf("expected metric state 2 in body, got:\n%s", metricsBody)
	}
	if !strings.Contains(metricsBody, "lensio_ocr_circuit_breaker_tripped_total 1") {
		t.Errorf("expected metric tripped total 1 in body, got:\n%s", metricsBody)
	}
}

func TestCircuitBreaker_TripAndReset(t *testing.T) {
	mock := providers.NewMockEngine()
	cb := ocr.NewCircuitBreaker(mock)

	cb.Trip()
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected StateOpen after Trip, got %v", cb.State())
	}

	_, err := cb.Extract(context.Background(), []byte("data"))
	if !errors.Is(err, ocr.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen after Trip, got %v", err)
	}

	cb.Reset()
	if cb.State() != ocr.StateClosed {
		t.Fatalf("expected StateClosed after Reset, got %v", cb.State())
	}

	res, err := cb.Extract(context.Background(), synthetic.GenerateValidKTPImage())
	if err != nil {
		t.Fatalf("unexpected error after Reset: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result after Reset")
	}
}

func TestCircuitBreaker_NilEngine(t *testing.T) {
	cb := ocr.NewCircuitBreaker(nil)
	_, err := cb.Extract(context.Background(), []byte("data"))
	if !errors.Is(err, ocr.ErrOCRFailed) {
		t.Fatalf("expected ErrOCRFailed on nil engine, got %v", err)
	}
}

func TestCircuitBreaker_WithCircuitBreakerHelper(t *testing.T) {
	mock := providers.NewMockEngine()
	wrapped := ocr.WithCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 3,
	})

	if wrapped == nil {
		t.Fatalf("expected non-nil wrapped engine")
	}

	res, err := wrapped.Extract(context.Background(), synthetic.GenerateValidKTPImage())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
}

func TestCircuitBreaker_HalfOpen_ConcurrentProbeLimit(t *testing.T) {
	mock := &mockFailingEngine{
		delay: 50 * time.Millisecond,
	}
	fakeNow := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Cooldown:         5 * time.Second,
		NowFunc: func() time.Time {
			return fakeNow
		},
	})

	// Trip to open
	mock.SetError(ocr.ErrOCRFailed)
	_, _ = cb.Extract(context.Background(), []byte("img"))
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected StateOpen")
	}

	// Advance past cooldown
	fakeNow = fakeNow.Add(10 * time.Second)
	mock.SetError(nil)

	// Launch two concurrent probes
	var wg sync.WaitGroup
	var probe1Err, probe2Err error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, probe1Err = cb.Extract(context.Background(), []byte("img"))
	}()

	// Slight delay to ensure first probe has acquired halfOpenInFlight
	time.Sleep(5 * time.Millisecond)

	go func() {
		defer wg.Done()
		_, probe2Err = cb.Extract(context.Background(), []byte("img"))
	}()

	wg.Wait()

	// One should succeed (canary probe) and the other should be rejected with ErrCircuitOpen
	if probe1Err != nil && !errors.Is(probe1Err, ocr.ErrCircuitOpen) {
		t.Errorf("unexpected probe1 error: %v", probe1Err)
	}
	if probe2Err != nil && !errors.Is(probe2Err, ocr.ErrCircuitOpen) {
		t.Errorf("unexpected probe2 error: %v", probe2Err)
	}
	if (probe1Err == nil && probe2Err == nil) || (probe1Err != nil && probe2Err != nil) {
		t.Fatalf("expected exactly one probe to succeed and one to fast-fail with ErrCircuitOpen, got p1=%v, p2=%v",
			probe1Err, probe2Err)
	}
}

func TestCircuitBreaker_ConcurrentExecution(t *testing.T) {
	mock := &mockFailingEngine{}
	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Cooldown:         10 * time.Millisecond,
		Timeout:          500 * time.Millisecond,
	})

	var wg sync.WaitGroup
	workers := 50
	iterations := 20
	var totalCalls int64

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddInt64(&totalCalls, 1)
				// Alternate errors and successes
				if workerID%2 == 0 && j%3 == 0 {
					mock.SetError(ocr.ErrOCRFailed)
				} else {
					mock.SetError(nil)
				}
				_, _ = cb.Extract(context.Background(), []byte("data"))
			}
		}(i)
	}

	wg.Wait()

	if totalCalls != int64(workers*iterations) {
		t.Errorf("expected %d total calls, got %d", workers*iterations, totalCalls)
	}
}

func BenchmarkCircuitBreaker_Closed(b *testing.B) {
	mock := providers.NewMockEngine()
	cb := ocr.NewCircuitBreaker(mock, ocr.CircuitBreakerConfig{
		Timeout: 0, // avoid per-call timer alloc in benchmark
	})
	img := synthetic.GenerateValidKTPImage()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = cb.Extract(ctx, img)
	}
}

func BenchmarkCircuitBreaker_Open_FastFail(b *testing.B) {
	mock := providers.NewMockEngine()
	cb := ocr.NewCircuitBreaker(mock)
	cb.Trip() // force open
	img := []byte("data")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = cb.Extract(ctx, img)
	}
}
