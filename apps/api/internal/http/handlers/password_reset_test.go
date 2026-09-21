package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/email"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

type resetSpy struct {
	knownEmail  string
	storedHash  string
	storedFor   string
	consumeErr  error
	consumedFor string
	consumeHash string
	newHash     string
}

func (s *resetSpy) SetPasswordResetToken(_ context.Context, email, tokenHash string, _ time.Time) error {
	if s.knownEmail != "" && !strings.EqualFold(email, s.knownEmail) {
		return store.ErrNotFound
	}
	s.storedFor, s.storedHash = email, tokenHash
	return nil
}

func (s *resetSpy) ConsumePasswordResetToken(_ context.Context, email, tokenHash, newPasswordHash string, _ time.Time) error {
	s.consumedFor, s.consumeHash, s.newHash = email, tokenHash, newPasswordHash
	return s.consumeErr
}

type capturingSender struct{ sent []email.Message }

func (c *capturingSender) Send(_ context.Context, msg email.Message) error {
	c.sent = append(c.sent, msg)
	return nil
}

func postJSON(t *testing.T, h http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The reset endpoint is unauthenticated, so it must answer identically for
// registered and unregistered addresses or it becomes an enumeration oracle.
func TestRequestPasswordReset_DoesNotRevealAccountExistence(t *testing.T) {
	known := &resetSpy{knownEmail: "real@example.com"}
	sender := &capturingSender{}
	h := handlers.RequestPasswordResetHandler(known, sender, "https://app.lensio.dev")

	hit := postJSON(t, h, "/api/v1/auth/password-reset", `{"email":"real@example.com"}`)
	miss := postJSON(t, h, "/api/v1/auth/password-reset", `{"email":"nobody@example.com"}`)

	if hit.Code != miss.Code {
		t.Errorf("status differs for known (%d) and unknown (%d) addresses", hit.Code, miss.Code)
	}
	if hit.Body.String() != miss.Body.String() {
		t.Errorf("body differs:\n known: %s\n unknown: %s", hit.Body.String(), miss.Body.String())
	}

	// Only the real account should actually receive mail.
	if len(sender.sent) != 1 || sender.sent[0].To != "real@example.com" {
		t.Errorf("expected exactly one email to the registered address, got %+v", sender.sent)
	}
}

func TestRequestPasswordReset_StoresOnlyTheTokenHash(t *testing.T) {
	spy := &resetSpy{}
	sender := &capturingSender{}
	h := handlers.RequestPasswordResetHandler(spy, sender, "https://app.lensio.dev")

	if rec := postJSON(t, h, "/api/v1/auth/password-reset", `{"email":"user@example.com"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected one email, got %d", len(sender.sent))
	}

	// Recover the plaintext token from the emailed link and confirm the store
	// only ever saw its hash.
	body := sender.sent[0].Body
	idx := strings.Index(body, "token=")
	if idx < 0 {
		t.Fatalf("no token in the email body:\n%s", body)
	}
	token := strings.Fields(body[idx+len("token="):])[0]

	if spy.storedHash == token {
		t.Fatal("the plaintext reset token was persisted; only its hash may be stored")
	}
	if spy.storedHash != handlers.HashResetToken(token) {
		t.Errorf("stored hash %q does not match the emailed token", spy.storedHash)
	}
	if len(spy.storedHash) != 64 {
		t.Errorf("expected a SHA-256 hex digest, got %d characters", len(spy.storedHash))
	}
}

func TestRequestPasswordReset_RejectsMalformedInput(t *testing.T) {
	h := handlers.RequestPasswordResetHandler(&resetSpy{}, &capturingSender{}, "https://app.lensio.dev")

	for _, body := range []string{`not json`, `{"email":""}`, `{"email":"not-an-email"}`} {
		if rec := postJSON(t, h, "/api/v1/auth/password-reset", body); rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %s, got %d", body, rec.Code)
		}
	}
}

func TestConfirmPasswordReset_Success(t *testing.T) {
	spy := &resetSpy{}
	h := handlers.ConfirmPasswordResetHandler(spy)

	rec := postJSON(t, h, "/api/v1/auth/password-reset/confirm",
		`{"email":"user@example.com","token":"plaintext-token","new_password":"a-strong-password"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if spy.consumeHash != handlers.HashResetToken("plaintext-token") {
		t.Error("the handler must look the token up by hash, not plaintext")
	}
	if spy.newHash == "a-strong-password" || !strings.HasPrefix(spy.newHash, "$2") {
		t.Errorf("the new password must be bcrypt hashed before storage, got %q", spy.newHash)
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if msg, _ := body["message"].(string); !strings.Contains(msg, "sesi lama") {
		t.Errorf("the response should state that existing sessions were invalidated, got %q", msg)
	}
}

func TestConfirmPasswordReset_RejectsBadTokens(t *testing.T) {
	spy := &resetSpy{consumeErr: store.ErrNotFound}
	h := handlers.ConfirmPasswordResetHandler(spy)

	rec := postJSON(t, h, "/api/v1/auth/password-reset/confirm",
		`{"email":"user@example.com","token":"wrong","new_password":"a-strong-password"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	// Expired, already used and simply wrong must be indistinguishable.
	if !strings.Contains(rec.Body.String(), "kedaluwarsa") {
		t.Errorf("expected a single generic message, got %s", rec.Body.String())
	}
}

func TestConfirmPasswordReset_EnforcesPasswordPolicy(t *testing.T) {
	spy := &resetSpy{}
	h := handlers.ConfirmPasswordResetHandler(spy)

	for _, body := range []string{
		`{"email":"user@example.com","token":"t","new_password":"short"}`,
		`{"email":"user@example.com","token":"","new_password":"a-strong-password"}`,
		`{"email":"","token":"t","new_password":"a-strong-password"}`,
		`not json`,
	} {
		if rec := postJSON(t, h, "/api/v1/auth/password-reset/confirm", body); rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %s, got %d", body, rec.Code)
		}
	}
	if spy.consumedFor != "" {
		t.Error("the store must not be touched when the request is malformed")
	}
}

func TestConfirmPasswordReset_SurfacesStoreFailures(t *testing.T) {
	spy := &resetSpy{consumeErr: errors.New("connection reset")}
	h := handlers.ConfirmPasswordResetHandler(spy)

	rec := postJSON(t, h, "/api/v1/auth/password-reset/confirm",
		`{"email":"user@example.com","token":"t","new_password":"a-strong-password"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Error("internal error detail leaked to the client")
	}
}

func TestConfirmPasswordReset_NilStore(t *testing.T) {
	h := handlers.ConfirmPasswordResetHandler(nil)
	if rec := postJSON(t, h, "/api/v1/auth/password-reset/confirm", `{}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for a nil store, got %d", rec.Code)
	}
}
