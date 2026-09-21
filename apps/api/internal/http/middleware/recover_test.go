package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
)

func TestRecover_PanicBecomesErrorEnvelope(t *testing.T) {
	handler := middleware.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/ktp", nil)
	rec := httptest.NewRecorder()

	// Must not propagate: net/http would otherwise drop the connection with no response.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected a JSON error envelope, got %q", rec.Body.String())
	}
	if body["error"]["code"] == "" {
		t.Errorf("expected an error code in the envelope, got %v", body)
	}
	// The stack trace belongs in the logs, never in the response.
	if got := rec.Body.String(); strings.Contains(got, "goroutine") || strings.Contains(got, "boom") {
		t.Errorf("panic detail leaked to the client: %s", got)
	}
}

func TestRecover_PassesThroughHealthyHandlers(t *testing.T) {
	handler := middleware.Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`ok`))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "ok" {
		t.Fatalf("healthy handler was altered: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecover_ReraisesErrAbortHandler(t *testing.T) {
	handler := middleware.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("ErrAbortHandler must propagate to net/http, got %v", recovered)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// stubRevoker reports a fixed cutoff for every user.
type stubRevoker struct {
	validFrom time.Time
	err       error
	calls     int
}

func (s *stubRevoker) SessionsValidFrom(context.Context, string) (time.Time, error) {
	s.calls++
	return s.validFrom, s.err
}

func TestSessionTokenValidator_RejectsRevokedSessions(t *testing.T) {
	secret := []byte("session-revocation-test-secret")
	token, err := auth.CreateSessionTokenWithSecret("user-1", "u@example.com", "u", []string{"developer"}, time.Hour, secret)
	if err != nil {
		t.Fatalf("CreateSessionTokenWithSecret: %v", err)
	}

	v := middleware.NewSessionTokenValidator(secret)

	// Without a revocation checker the token is accepted (pre-existing behaviour).
	if _, err := v.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected the token to validate before revocation: %v", err)
	}

	// Cutoff in the future: the token was issued before it, so it is revoked.
	v.SetRevocationChecker(&stubRevoker{validFrom: time.Now().Add(time.Minute)})
	if _, err := v.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected a revoked session to be refused")
	}

	// Cutoff in the past: still valid.
	v.SetRevocationChecker(&stubRevoker{validFrom: time.Now().Add(-time.Hour)})
	if _, err := v.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected a token issued after the cutoff to pass: %v", err)
	}
}

func TestSessionTokenValidator_FailsOpenOnLookupError(t *testing.T) {
	secret := []byte("session-revocation-test-secret")
	token, _ := auth.CreateSessionTokenWithSecret("user-2", "u2@example.com", "u2", nil, time.Hour, secret)

	v := middleware.NewSessionTokenValidator(secret)
	v.SetRevocationChecker(&stubRevoker{err: context.DeadlineExceeded})

	// A database blip must not sign every user out.
	if _, err := v.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected fail-open on lookup error, got %v", err)
	}
}

func TestSessionTokenValidator_CachesRevocationLookups(t *testing.T) {
	secret := []byte("session-revocation-test-secret")
	token, _ := auth.CreateSessionTokenWithSecret("user-3", "u3@example.com", "u3", nil, time.Hour, secret)

	stub := &stubRevoker{validFrom: time.Now().Add(-time.Hour)}
	v := middleware.NewSessionTokenValidator(secret)
	v.SetRevocationChecker(stub)

	for i := 0; i < 5; i++ {
		if _, err := v.ValidateToken(context.Background(), token); err != nil {
			t.Fatalf("validate %d: %v", i, err)
		}
	}
	if stub.calls != 1 {
		t.Fatalf("expected the revocation lookup to be cached, got %d calls", stub.calls)
	}

	v.InvalidateRevocationCache("user-3")
	if _, err := v.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("validate after invalidation: %v", err)
	}
	if stub.calls != 2 {
		t.Fatalf("expected a fresh lookup after invalidation, got %d calls", stub.calls)
	}
}
