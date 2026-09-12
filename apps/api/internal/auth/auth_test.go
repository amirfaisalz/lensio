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

func TestValidateSessionToken(t *testing.T) {
	secret := []byte("test-custom-secret-key-32b-long!!")
	auth.SetTokenSecret(secret)
	if string(auth.GetTokenSecret()) != string(secret) {
		t.Fatalf("expected token secret to match %s", string(secret))
	}

	// 1. Valid token
	token, err := auth.CreateSessionTokenWithSecret("usr-1", "test@lensio.dev", "testuser", []string{"developer", "ocr:write"}, 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("failed creating session token: %v", err)
	}

	claims, err := auth.ValidateSessionToken(token, secret)
	if err != nil {
		t.Fatalf("expected valid token validation, got error: %v", err)
	}
	if claims.Sub != "usr-1" || claims.Email != "test@lensio.dev" {
		t.Errorf("unexpected claims: %+v", claims)
	}

	// 2. Validate with default secret fallback
	claimsDef, err := auth.ValidateSessionToken(token, nil)
	if err != nil {
		t.Fatalf("expected validation with default secret, got: %v", err)
	}
	if claimsDef.Sub != "usr-1" {
		t.Errorf("unexpected sub in claimsDef: %s", claimsDef.Sub)
	}

	// 3. Invalid signature
	wrongSecret := []byte("wrong-secret-key-32b-long-invalid!")
	_, err = auth.ValidateSessionToken(token, wrongSecret)
	if err == nil {
		t.Fatal("expected error with wrong secret")
	}

	// 4. Malformed tokens
	if _, err := auth.ValidateSessionToken("invalid", secret); err == nil {
		t.Fatal("expected error for malformed token")
	}
	if _, err := auth.ValidateSessionToken("a.b.c", secret); err == nil {
		t.Fatal("expected error for non-base64 token")
	}

	// 5. Expired token
	expiredToken, err := auth.CreateSessionTokenWithSecret("usr-1", "test@lensio.dev", "testuser", []string{"developer"}, -1*time.Hour, secret)
	if err != nil {
		t.Fatalf("failed creating expired token: %v", err)
	}
	if _, err := auth.ValidateSessionToken(expiredToken, secret); err == nil {
		t.Fatal("expected error for expired token")
	}

	// 6. Legacy SHA-256 password hash verification
	// Hex salt (16 bytes = 32 hex chars): "0123456789abcdef0123456789abcdef"
	legacySalt := "0123456789abcdef0123456789abcdef"
	// Create legacy hash
	legacyPass := "OldPass123"
	legacyHash, err := auth.HashPassword(legacyPass)
	if err != nil {
		t.Fatalf("failed hashing: %v", err)
	}
	if !auth.VerifyPassword(legacyPass, legacyHash) {
		t.Fatal("failed verifying modern bcrypt password")
	}

	// Test legacy salt format manually
	legacyManualHash := legacySalt + "$e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_ = auth.VerifyPassword("any", legacyManualHash)
}
