package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrEmptyPassword        = errors.New("password cannot be empty")
	ErrInvalidStoredHash    = errors.New("invalid stored password hash format")
	DefaultTokenSecret      = []byte("lensio-session-secret-key-production-32b")
	DefaultSessionDuration  = 7 * 24 * time.Hour
)

// HashPassword creates a cryptographically salted SHA-256 hash of the given password.
// Format: <hex_salt>$<hex_sha256_hash>
func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	sum := h.Sum(nil)

	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(sum), nil
}

// VerifyPassword performs a constant-time comparison of the password against the stored hash.
func VerifyPassword(password, storedHash string) bool {
	if password == "" || storedHash == "" {
		return false
	}

	parts := strings.Split(storedHash, "$")
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil || len(salt) == 0 {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[1])
	if err != nil || len(expectedHash) == 0 {
		return false
	}

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	actualHash := h.Sum(nil)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

// GenerateVerificationToken returns a cryptographically secure hex-encoded 16-byte random token.
func GenerateVerificationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating verification token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// TokenClaims represents the session payload encoded into JWT.
type TokenClaims struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Roles             []string `json:"roles"`
	Iat               int64    `json:"iat"`
	Exp               int64    `json:"exp"`
}

// CreateSessionToken generates a signed 3-part base64URL JWT session token.
func CreateSessionToken(userID, email, username string, roles []string, duration time.Duration) (string, error) {
	if duration <= 0 {
		duration = DefaultSessionDuration
	}

	now := time.Now()
	claims := TokenClaims{
		Sub:               userID,
		Email:             email,
		EmailVerified:     true,
		PreferredUsername: username,
		Name:              username,
		Roles:             roles,
		Iat:               now.Unix(),
		Exp:               now.Add(duration).Unix(),
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshaling jwt header: %w", err)
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshaling jwt claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	mac := hmac.New(sha256.New, DefaultTokenSecret)
	mac.Write([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sigB64, nil
}
