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
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmptyPassword       = errors.New("password cannot be empty")
	ErrInvalidStoredHash   = errors.New("invalid stored password hash format")
	ErrMalformedToken      = errors.New("malformed session token")
	ErrInvalidSignature    = errors.New("invalid session token signature")
	ErrTokenExpired        = errors.New("session token has expired")
	DefaultSessionDuration = 7 * 24 * time.Hour

	tokenSecretMu sync.RWMutex
	tokenSecret   = []byte("lensio-session-secret-key-development-32b")
)

// SetTokenSecret configures the HMAC signing key for internal session tokens.
func SetTokenSecret(secret []byte) {
	tokenSecretMu.Lock()
	defer tokenSecretMu.Unlock()
	if len(secret) > 0 {
		tokenSecret = secret
	}
}

// GetTokenSecret returns the current HMAC signing key for session tokens.
func GetTokenSecret() []byte {
	tokenSecretMu.RLock()
	defer tokenSecretMu.RUnlock()
	return tokenSecret
}

// HashPassword creates a secure bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrEmptyPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("generating bcrypt hash: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword verifies a plaintext password against a stored hash.
// Supports modern bcrypt ($2a$, $2b$, $2y$) and maintains legacy salted SHA-256 compatibility.
func VerifyPassword(password, storedHash string) bool {
	if password == "" || storedHash == "" {
		return false
	}

	// 1. Check bcrypt hashes
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") || strings.HasPrefix(storedHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}

	// 2. Legacy fallback for salted SHA-256: <hex_salt>$<hex_sha256_hash>
	parts := strings.Split(storedHash, "$")
	if len(parts) == 2 {
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

	return false
}

// dummyPasswordHash is a real bcrypt hash of a throwaway value, used only to
// spend the same CPU time when no account matches the submitted email.
// #nosec G101 -- not a credential: it hashes a fixed non-secret string.
const dummyPasswordHash = "$2a$10$F7aZtg0LKefNVGskqE1GGuVpCGqodSFMcn9iLRPmfVU..8MBJztXm"

// EqualizeLoginTiming performs a throwaway bcrypt comparison so that a login
// attempt for an unknown email costs roughly the same as one for a real
// account. Without it, "no such user" returned in microseconds while a real
// account spent ~80ms in bcrypt, which is a reliable account-enumeration oracle.
func EqualizeLoginTiming(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
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

// CreateSessionToken generates a signed 3-part base64URL JWT session token using the default secret.
func CreateSessionToken(userID, email, username string, roles []string, duration time.Duration) (string, error) {
	return CreateSessionTokenWithSecret(userID, email, username, roles, duration, GetTokenSecret())
}

// CreateSessionTokenWithSecret generates a signed 3-part base64URL JWT session token using the specified secret.
func CreateSessionTokenWithSecret(userID, email, username string, roles []string, duration time.Duration, secret []byte) (string, error) {
	if duration == 0 {
		duration = DefaultSessionDuration
	}
	if len(secret) == 0 {
		secret = GetTokenSecret()
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

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sigB64, nil
}

// ValidateSessionToken strictly validates an HS256 JWT session token against the secret.
func ValidateSessionToken(token string, secret []byte) (*TokenClaims, error) {
	if len(secret) == 0 {
		secret = GetTokenSecret()
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid header base64", ErrMalformedToken)
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Alg != "HS256" {
		return nil, fmt.Errorf("%w: expected HS256 algorithm", ErrMalformedToken)
	}

	// Verify HMAC-SHA256 signature
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sigBytes, expectedSig) {
		return nil, ErrInvalidSignature
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid payload base64", ErrMalformedToken)
	}

	var claims TokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid payload json", ErrMalformedToken)
	}

	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}
