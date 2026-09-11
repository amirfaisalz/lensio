package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	EnvLive = "live"
	EnvTest = "test"

	PrefixLive = "nusa_live_"
	PrefixTest = "nusa_test_"

	tokenByteLen = 32
)

// GeneratedKey holds the result of generating a new API key.
// Plaintext is sensitive and must only be returned once to the caller.
type GeneratedKey struct {
	Plaintext   string
	KeyHash     string
	Prefix      string
	Environment string
}

// Generate creates a cryptographically secure API key.
// Plaintext format: nusa_{live|test}_{64-hex-token}.
// KeyHash: SHA-256 hex of the plaintext key.
// Prefix: nusa_{live|test}_{first 4 hex chars} for safe identification.
func Generate(env string) (*GeneratedKey, error) {
	cleanEnv := strings.ToLower(strings.TrimSpace(env))
	if cleanEnv == "" {
		cleanEnv = EnvLive
	}
	if cleanEnv != EnvLive && cleanEnv != EnvTest {
		return nil, fmt.Errorf("invalid environment %q: must be %q or %q", env, EnvLive, EnvTest)
	}

	tokenBytes := make([]byte, tokenByteLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}

	tokenHex := hex.EncodeToString(tokenBytes)
	plaintext := fmt.Sprintf("nusa_%s_%s", cleanEnv, tokenHex)
	keyHash := Hash(plaintext)
	prefix := fmt.Sprintf("nusa_%s_%s", cleanEnv, tokenHex[:4])

	return &GeneratedKey{
		Plaintext:   plaintext,
		KeyHash:     keyHash,
		Prefix:      prefix,
		Environment: cleanEnv,
	}, nil
}

// Hash returns the hex-encoded SHA-256 hash of the plaintext key.
func Hash(plaintextKey string) string {
	sum := sha256.Sum256([]byte(plaintextKey))
	return hex.EncodeToString(sum[:])
}

// Mask produces a safe masked representation of an API key for dashboard/listing display.
// Example: "nusa_live_1234••••••••"
func Mask(prefix string) string {
	if prefix == "" {
		return "••••••••"
	}
	return prefix + "••••••••"
}

// ValidateFormat checks if a string has the valid structure of a NusaID API key.
func ValidateFormat(key string) (env string, valid bool) {
	if strings.HasPrefix(key, PrefixLive) {
		token := strings.TrimPrefix(key, PrefixLive)
		if len(token) == tokenByteLen*2 && isHex(token) {
			return EnvLive, true
		}
	} else if strings.HasPrefix(key, PrefixTest) {
		token := strings.TrimPrefix(key, PrefixTest)
		if len(token) == tokenByteLen*2 && isHex(token) {
			return EnvTest, true
		}
	}
	return "", false
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
