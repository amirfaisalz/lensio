package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type mockFailingIdempotencyStore struct {
	err error
}

func (m *mockFailingIdempotencyStore) LockOrGet(ctx context.Context, orgID, key, requestHash string, ttl time.Duration) (*idempotency.Record, bool, error) {
	return nil, false, m.err
}

func (m *mockFailingIdempotencyStore) Complete(ctx context.Context, orgID, key string, statusCode int, headers map[string]string, body []byte) error {
	return m.err
}

func (m *mockFailingIdempotencyStore) Release(ctx context.Context, orgID, key string) error {
	return m.err
}

func TestIdempotencyMiddleware_NilStoreAndNoHeader(t *testing.T) {
	handlerCalled := false
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	t.Run("nil store passes through", func(t *testing.T) {
		handlerCalled = false
		mw := middleware.Idempotency(nil)(dummyHandler)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("body")))
		req.Header.Set("Idempotency-Key", "test-key-1")
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if !handlerCalled || rec.Code != http.StatusOK {
			t.Fatalf("expected handler called with 200, got called=%v code=%d", handlerCalled, rec.Code)
		}
	})

	t.Run("no idempotency header passes through without caching", func(t *testing.T) {
		handlerCalled = false
		memStore := idempotency.NewMemoryStore(time.Hour)
		mw := middleware.Idempotency(memStore)(dummyHandler)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("body")))
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if !handlerCalled || rec.Code != http.StatusOK {
			t.Fatalf("expected handler called with 200, got called=%v code=%d", handlerCalled, rec.Code)
		}
		if memStore.Count() != 0 {
			t.Fatalf("expected 0 cached records, got %d", memStore.Count())
		}
	})
}

func TestIdempotencyMiddleware_InvalidKeyLength(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Idempotency(memStore)(dummyHandler)

	longKey := strings.Repeat("k", 257)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("body")))
	req.Header.Set("Idempotency-Key", longKey)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized key, got %d", rec.Code)
	}

	var errEnv response.ErrorEnvelope
	_ = json.NewDecoder(rec.Body).Decode(&errEnv)
	if errEnv.Error.Code != response.CodeInvalidRequest {
		t.Errorf("expected code invalid_request, got %s", errEnv.Error.Code)
	}
}

func TestIdempotencyMiddleware_StoreError(t *testing.T) {
	failStore := &mockFailingIdempotencyStore{err: errors.New("db disconnected")}
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Idempotency(failStore)(dummyHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("body")))
	req.Header.Set("Idempotency-Key", "key-fail")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when store fails, got %d", rec.Code)
	}
}

func TestIdempotencyMiddleware_ReplayAndMismatch(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	callCount := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-val")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	})

	mw := middleware.Idempotency(memStore)(handler)
	keyID := "idemp-key-12345"
	payload := []byte("original payload bytes")

	// 1. Initial execution
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req1.Header.Set("Idempotency-Key", keyID)
	rec1 := httptest.NewRecorder()

	mw.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 on initial request, got %d", rec1.Code)
	}
	if rec1.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatal("Idempotent-Replayed should NOT be true on initial execution")
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}

	// 2. Replay with identical key and identical payload
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req2.Header.Set("Idempotency-Key", keyID)
	rec2 := httptest.NewRecorder()

	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on replay, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true, got %s", rec2.Header().Get("Idempotent-Replayed"))
	}
	if rec2.Header().Get("X-Custom-Header") != "test-val" {
		t.Fatalf("expected cached header X-Custom-Header: test-val, got %s", rec2.Header().Get("X-Custom-Header"))
	}
	if rec2.Body.String() != `{"message":"success"}` {
		t.Fatalf("expected cached body, got %s", rec2.Body.String())
	}
	if callCount != 1 {
		t.Fatalf("handler should NOT be called on cached replay; callCount=%d", callCount)
	}

	// 3. Replay with SAME key but DIFFERENT payload -> 422 Unprocessable Entity
	alteredPayload := []byte("altered payload bytes")
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(alteredPayload))
	req3.Header.Set("Idempotency-Key", keyID)
	rec3 := httptest.NewRecorder()

	mw.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity for altered payload, got %d", rec3.Code)
	}

	var errEnv response.ErrorEnvelope
	_ = json.NewDecoder(rec3.Body).Decode(&errEnv)
	if errEnv.Error.Code != response.CodeIdempotencyMismatch {
		t.Errorf("expected code %s, got %s", response.CodeIdempotencyMismatch, errEnv.Error.Code)
	}
	if callCount != 1 {
		t.Fatalf("handler should NOT be called on mismatch; callCount=%d", callCount)
	}
}

func TestIdempotencyMiddleware_ConcurrentRequestsReturn409(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	keyID := "concurrent-test-key"
	payload := []byte("concurrent payload")

	gate := make(chan struct{})
	startedFirst := make(chan struct{})

	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(startedFirst)
		<-gate // block until second request has arrived and completed
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"done"}`))
	})

	mw := middleware.Idempotency(memStore)(slowHandler)

	var rec1 *httptest.ResponseRecorder
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
		req1.Header.Set("Idempotency-Key", keyID)
		rec1 = httptest.NewRecorder()
		mw.ServeHTTP(rec1, req1)
	}()

	// Wait for the first request to enter the handler and hold in_progress status
	<-startedFirst

	// Fire second concurrent request with identical key and payload
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req2.Header.Set("Idempotency-Key", keyID)
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)

	// Second request must receive 409 Conflict
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for concurrent in-progress request, got %d", rec2.Code)
	}

	var errEnv response.ErrorEnvelope
	_ = json.NewDecoder(rec2.Body).Decode(&errEnv)
	if errEnv.Error.Code != response.CodeRequestInProgress {
		t.Errorf("expected code %s, got %s", response.CodeRequestInProgress, errEnv.Error.Code)
	}

	// Release first request
	close(gate)
	wg.Wait()

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to finish 200, got %d", rec1.Code)
	}
}

func TestIdempotencyMiddleware_TransientFailureReleasesLock(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	shouldFail := true

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server crashed"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("recovered"))
	})

	mw := middleware.Idempotency(memStore)(handler)
	keyID := "transient-key"
	payload := []byte("transient payload")

	// 1. First request fails with 500
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req1.Header.Set("Idempotency-Key", keyID)
	rec1 := httptest.NewRecorder()

	mw.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec1.Code)
	}

	// 2. Client retries with same key -> because lock was released, second request succeeds
	shouldFail = false
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req2.Header.Set("Idempotency-Key", keyID)
	rec2 := httptest.NewRecorder()

	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on retry after 500 failure, got %d", rec2.Code)
	}
	if rec2.Body.String() != "recovered" {
		t.Fatalf("expected recovered body, got %s", rec2.Body.String())
	}
}

func TestIdempotencyMiddleware_PanicReleasesLock(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	keyID := "panic-key"
	payload := []byte("panic payload")

	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected crash")
	})

	mw := middleware.Idempotency(memStore)(panickingHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(payload))
	req.Header.Set("Idempotency-Key", keyID)
	rec := httptest.NewRecorder()

	// Handler must cleanly recover from panic without re-panicking (Zero Panics rule), release lock, and return 500
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
	}

	// Verify lock was released despite panic
	if memStore.Count() != 0 {
		t.Fatalf("expected store count 0 after panic release, got %d", memStore.Count())
	}
}

func TestIdempotencyMiddleware_WithAuthContext(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mw := middleware.Idempotency(memStore)(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("body")))
	req.Header.Set("Idempotency-Key", "auth-key-1")
	key := &store.APIKey{
		ID:    "key-1",
		OrgID: "org-auth-test",
	}
	req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify the record was stored under org-auth-test
	recStored, isNew, err := memStore.LockOrGet(context.Background(), "org-auth-test", "auth-key-1", "", time.Hour)
	if err != nil || isNew || recStored.Status != idempotency.StatusCompleted {
		t.Fatalf("expected record stored under org-auth-test: isNew=%v, err=%v", isNew, err)
	}
}

func TestIdempotencyMiddleware_RateLimitNotCached(t *testing.T) {
	memStore := idempotency.NewMemoryStore(time.Hour)
	calls := 0
	limitedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("quota exceeded"))
	})
	mw := middleware.Idempotency(memStore)(limitedHandler)

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/sim", bytes.NewReader([]byte("same-payload")))
		req.Header.Set("Idempotency-Key", "quota-key-1")
		return req
	}

	first := httptest.NewRecorder()
	mw.ServeHTTP(first, newReq())
	second := httptest.NewRecorder()
	mw.ServeHTTP(second, newReq())

	if first.Code != http.StatusTooManyRequests || second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 twice, got %d then %d", first.Code, second.Code)
	}
	if calls != 2 {
		t.Fatalf("expected handler executed twice (429 must release, never replay), got %d", calls)
	}
	if second.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatal("429 response must not be replayed from cache")
	}
	if memStore.Count() != 0 {
		t.Fatalf("expected store count 0 after 429 release, got %d", memStore.Count())
	}
}
