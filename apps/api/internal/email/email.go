// Package email delivers the transactional messages the auth flows depend on.
//
// Until this package existed, registration generated a verification token,
// stored it, and then dropped it on the floor: nothing ever sent it anywhere.
// Self-service signup could not complete in production, because logging in
// requires a verified address and no one could ever receive the token.
package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// ErrNotConfigured reports that no transport was configured.
var ErrNotConfigured = errors.New("email sender is not configured")

// Message is a single outbound transactional email.
type Message struct {
	To      string
	Subject string
	Body    string // plain text; these messages carry a link and nothing else
}

// Sender delivers transactional email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// LogSender writes messages to the application log instead of delivering them.
// It exists for local development; it is never selected in production, where a
// missing SMTP configuration is a startup error rather than a silent downgrade.
type LogSender struct {
	Logger *slog.Logger
}

// Send records the message, including the link, so a developer can complete the
// flow without an inbox.
func (s *LogSender) Send(_ context.Context, msg Message) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("email not delivered: no SMTP transport configured, logging instead",
		slog.String("to", msg.To),
		slog.String("subject", msg.Subject),
		slog.String("body", msg.Body),
	)
	return nil
}

// SMTPSender delivers mail over SMTP with STARTTLS, using only the standard library.
type SMTPSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	Timeout  time.Duration
}

// NewSMTPSender validates the configuration and returns a ready sender.
func NewSMTPSender(host, port, username, password, from string) (*SMTPSender, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("%w: SMTP_HOST is empty", ErrNotConfigured)
	}
	if port = strings.TrimSpace(port); port == "" {
		port = "587"
	}
	if from = strings.TrimSpace(from); from == "" {
		return nil, fmt.Errorf("%w: SMTP_FROM is empty", ErrNotConfigured)
	}

	return &SMTPSender{
		Host:     host,
		Port:     port,
		Username: strings.TrimSpace(username),
		Password: password,
		From:     from,
		Timeout:  10 * time.Second,
	}, nil
}

// Send delivers one message. Header values are rejected if they contain CR or LF
// so a crafted address cannot inject extra headers or recipients.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	to := strings.TrimSpace(msg.To)
	if to == "" {
		return errors.New("recipient address is empty")
	}
	if containsCRLF(to) || containsCRLF(msg.Subject) || containsCRLF(s.From) {
		return errors.New("email header values must not contain CR or LF")
	}

	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	payload := "From: " + s.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + msg.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		msg.Body + "\r\n"

	addr := net.JoinHostPort(s.Host, s.Port)

	// smtp.SendMail blocks, so honour cancellation by running it alongside ctx.
	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, auth, s.From, []string{to}, []byte(payload))
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("sending mail via %s: %w", addr, err)
		}
		return nil
	}
}

func containsCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// VerificationMessage builds the address-confirmation email.
func VerificationMessage(baseURL, to, token string) Message {
	link := fmt.Sprintf("%s/verify-email?email=%s&token=%s", strings.TrimRight(baseURL, "/"), to, token)
	return Message{
		To:      to,
		Subject: "Verifikasi alamat email Lensio Anda",
		Body: "Halo,\n\n" +
			"Konfirmasi alamat email ini untuk mengaktifkan akun Lensio Anda:\n\n" +
			link + "\n\n" +
			"Abaikan email ini jika Anda tidak mendaftar. Tanpa konfirmasi, akun tidak dapat digunakan.\n",
	}
}

// PasswordResetMessage builds the password-reset email.
func PasswordResetMessage(baseURL, to, token string, validFor time.Duration) Message {
	link := fmt.Sprintf("%s/reset-password?email=%s&token=%s", strings.TrimRight(baseURL, "/"), to, token)
	return Message{
		To:      to,
		Subject: "Atur ulang kata sandi Lensio Anda",
		Body: "Halo,\n\n" +
			"Gunakan tautan berikut untuk mengatur ulang kata sandi Lensio Anda:\n\n" +
			link + "\n\n" +
			fmt.Sprintf("Tautan ini berlaku %s dan hanya dapat dipakai satu kali.\n", validFor) +
			"Abaikan email ini jika Anda tidak meminta pengaturan ulang; kata sandi Anda tidak berubah.\n",
	}
}
