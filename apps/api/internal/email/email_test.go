package email_test

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/email"
)

// fakeSMTPServer speaks just enough SMTP for smtp.SendMail to complete, so the
// happy path is proven against a real conversation rather than a mock.
type fakeSMTPServer struct {
	addr     string
	listener net.Listener
	mu       sync.Mutex
	received []string
	wg       sync.WaitGroup
}

func startFakeSMTP(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := &fakeSMTPServer{addr: ln.Addr().String(), listener: ln}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	r := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }

	write("220 fake.lensio.test ESMTP")
	var body strings.Builder
	inData := false

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			if trimmed == "." {
				inData = false
				s.mu.Lock()
				s.received = append(s.received, body.String())
				s.mu.Unlock()
				write("250 OK")
				continue
			}
			body.WriteString(trimmed + "\n")
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "EHLO"), strings.HasPrefix(trimmed, "HELO"):
			write("250-fake.lensio.test")
			write("250 AUTH PLAIN")
		case strings.HasPrefix(trimmed, "AUTH"):
			write("235 accepted")
		case strings.HasPrefix(trimmed, "MAIL FROM"), strings.HasPrefix(trimmed, "RCPT TO"):
			write("250 OK")
		case trimmed == "DATA":
			inData = true
			write("354 send it")
		case trimmed == "QUIT":
			write("221 bye")
			return
		default:
			write("250 OK")
		}
	}
}

func (s *fakeSMTPServer) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

func TestSMTPSender_DeliversMessage(t *testing.T) {
	srv := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)

	sender, err := email.NewSMTPSender(host, port, "", "", "noreply@lensio.dev")
	if err != nil {
		t.Fatalf("NewSMTPSender: %v", err)
	}

	err = sender.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Verifikasi",
		Body:    "https://app.lensio.dev/verify-email?token=abc",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	msgs := srv.messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 delivered message, got %d", len(msgs))
	}
	for _, want := range []string{
		"From: noreply@lensio.dev",
		"To: user@example.com",
		"Subject: Verifikasi",
		"https://app.lensio.dev/verify-email?token=abc",
	} {
		if !strings.Contains(msgs[0], want) {
			t.Errorf("delivered message missing %q:\n%s", want, msgs[0])
		}
	}
}

// A crafted recipient or subject must not be able to append headers.
func TestSMTPSender_RejectsHeaderInjection(t *testing.T) {
	srv := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	sender, _ := email.NewSMTPSender(host, port, "", "", "noreply@lensio.dev")

	cases := []email.Message{
		{To: "victim@example.com\r\nBcc: attacker@evil.test", Subject: "Hi", Body: "x"},
		{To: "victim@example.com", Subject: "Hi\r\nBcc: attacker@evil.test", Body: "x"},
		{To: "victim@example.com\nCc: attacker@evil.test", Subject: "Hi", Body: "x"},
	}
	for _, msg := range cases {
		if err := sender.Send(context.Background(), msg); err == nil {
			t.Errorf("expected rejection for header injection via %q / %q", msg.To, msg.Subject)
		}
	}

	if got := len(srv.messages()); got != 0 {
		t.Errorf("no message should have been delivered, got %d", got)
	}
}

func TestSMTPSender_RejectsEmptyRecipient(t *testing.T) {
	sender, _ := email.NewSMTPSender("localhost", "587", "", "", "noreply@lensio.dev")
	if err := sender.Send(context.Background(), email.Message{To: "   "}); err == nil {
		t.Error("expected an error for an empty recipient")
	}
}

func TestSMTPSender_HonoursContextCancellation(t *testing.T) {
	// Unroutable address so the dial hangs; cancellation must win.
	sender, _ := email.NewSMTPSender("192.0.2.1", "587", "", "", "noreply@lensio.dev")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sender.Send(ctx, email.Message{To: "user@example.com", Subject: "s", Body: "b"})
	if err == nil {
		t.Fatal("expected an error when the context expires")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("Send ignored context cancellation, took %s", elapsed)
	}
}

func TestNewSMTPSender_Validation(t *testing.T) {
	if _, err := email.NewSMTPSender("", "587", "", "", "a@b.dev"); err == nil {
		t.Error("an empty host must be rejected")
	}
	if _, err := email.NewSMTPSender("smtp.test", "587", "", "", ""); err == nil {
		t.Error("an empty From must be rejected")
	}
	s, err := email.NewSMTPSender("smtp.test", "", "", "", "a@b.dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Port != "587" {
		t.Errorf("expected the default submission port 587, got %q", s.Port)
	}
}

func TestLogSender_NeverFails(t *testing.T) {
	// Development fallback: it must always succeed so signup is not blocked.
	sender := &email.LogSender{Logger: slog.New(slog.NewTextHandler(discard{}, nil))}
	if err := sender.Send(context.Background(), email.Message{To: "a@b.dev", Subject: "s", Body: "b"}); err != nil {
		t.Errorf("LogSender must not fail: %v", err)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestMessageBuilders_IncludeActionableLink(t *testing.T) {
	v := email.VerificationMessage("https://app.lensio.dev/", "user@example.com", "tok123")
	if !strings.Contains(v.Body, "https://app.lensio.dev/verify-email?email=user@example.com&token=tok123") {
		t.Errorf("verification link malformed:\n%s", v.Body)
	}
	if v.To != "user@example.com" || v.Subject == "" {
		t.Error("verification message missing recipient or subject")
	}

	p := email.PasswordResetMessage("https://app.lensio.dev", "user@example.com", "tok456", time.Hour)
	if !strings.Contains(p.Body, "https://app.lensio.dev/reset-password?email=user@example.com&token=tok456") {
		t.Errorf("reset link malformed:\n%s", p.Body)
	}
	if !strings.Contains(p.Body, "satu kali") {
		t.Error("reset message should say the link is single use")
	}
}
