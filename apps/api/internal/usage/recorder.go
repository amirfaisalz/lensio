package usage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// Recorder asynchronously consumes usage records via a buffered channel and persists them to PostgreSQL.
type Recorder struct {
	store      store.UsageStore
	recordChan chan *store.UsageRecord
	wg         sync.WaitGroup
	mu         sync.RWMutex
	closed     bool
}

// NewRecorder initializes an asynchronous non-blocking usage recorder worker.
func NewRecorder(usageStore store.UsageStore, bufferSize int) *Recorder {
	if bufferSize <= 0 {
		bufferSize = 1024
	}

	r := &Recorder{
		store:      usageStore,
		recordChan: make(chan *store.UsageRecord, bufferSize),
	}

	r.wg.Add(1)
	go r.worker()

	return r
}

// Record dispatches a usage metric non-blockingly. If the channel is saturated or closed,
// the event is dropped without delaying the client request.
func (r *Recorder) Record(rec *store.UsageRecord) {
	if rec == nil {
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		slog.Warn("usage recorder is closed; dropping metric",
			slog.String("request_id", rec.RequestID),
		)
		return
	}

	select {
	case r.recordChan <- rec:
	default:
		slog.Warn("usage recorder channel saturated; dropping metric to protect throughput",
			slog.String("request_id", rec.RequestID),
			slog.String("org_id", rec.OrgID),
		)
	}
}

// worker continuously reads records from the channel and writes them to PostgreSQL.
func (r *Recorder) worker() {
	defer r.wg.Done()

	for rec := range r.recordChan {
		if r.store == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := r.store.CreateUsageRecord(ctx, rec); err != nil {
			slog.Error("failed persisting async usage record",
				slog.String("error", err.Error()),
				slog.String("request_id", rec.RequestID),
				slog.String("org_id", rec.OrgID),
			)
		}
		cancel()
	}
}

// Close signals the worker to finish processing remaining records in the buffer and waits up to ctx deadline.
func (r *Recorder) Close(ctx context.Context) error {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.recordChan)
	}
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
