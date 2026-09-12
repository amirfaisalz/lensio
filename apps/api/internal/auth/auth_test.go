package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	// Empty password
	if _, err := auth.HashPassword(""); err == nil {
		t.Fatal("expected error for empty password")
	}

	password := "Secret123!Safe"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if !strings.Contains(hash, "$") {
		t.Fatalf("expected hash to contain delimiter '$', got %s", hash)
	}

	// Valid password check
	if !auth.VerifyPassword(password, hash) {
		t.Fatal("expected password to match hash")
	}

	// Invalid password check
	if auth.VerifyPassword("WrongPassword", hash) {
		t.Fatal("expected wrong password to fail verification")
	}

	// Edge cases in VerifyPassword
	if auth.VerifyPassword("", hash) {
		t.Fatal("expected empty password to fail")
	}
	if auth.VerifyPassword(password, "") {
		t.Fatal("expected empty hash to fail")
	}
	if auth.VerifyPassword(password, "invalidhashwithoutdollar") {
		t.Fatal("expected invalid format to fail")
	}
	if auth.VerifyPassword(password, "nothexsalt$hash") {
		t.Fatal("expected invalid hex salt to fail")
	}
	if auth.VerifyPassword(password, "abcd$nothexhash") {
		t.Fatal("expected invalid hex hash to fail")
	}
	if auth.VerifyPassword(password, "$") {
		t.Fatal("expected empty parts to fail")
	}
}

func TestGenerateVerificationToken(t *testing.T) {
	tok1, err := auth.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}
	if len(tok1) != 32 {
		t.Fatalf("expected 32-char hex token, got %d chars: %s", len(tok1), tok1)
	}

	tok2, err := auth.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("unexpected error generating second token: %v", err)
	}
	if tok1 == tok2 {
		t.Fatal("expected distinct verification tokens")
	}
}

func TestCreateSessionToken(t *testing.T) {
	token, err := auth.CreateSessionToken("user-123", "user@lensio.dev", "user", []string{"developer", "ocr:write"}, 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error creating session token: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts in JWT token, got %d", len(parts))
	}

	// Default duration branch
	defaultToken, err := auth.CreateSessionToken("user-123", "user@lensio.dev", "user", []string{"developer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error creating default token: %v", err)
	}
	if len(strings.Split(defaultToken, ".")) != 3 {
		t.Fatal("expected valid 3-part token for default duration")
	}
}
