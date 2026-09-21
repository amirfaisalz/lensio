package idempotency

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// sealMagic marks a blob written by Seal. Rows persisted before encryption was
// introduced carry no prefix, so Open can still return them instead of failing
// a replay that a client is legitimately retrying.
var sealMagic = []byte{0x4C, 0x53, 0x31, 0x00} // "LS1\x00"

// ErrSealedPayloadCorrupt reports a blob that carries the sealed prefix but
// cannot be authenticated with the configured key.
var ErrSealedPayloadCorrupt = errors.New("sealed idempotency payload failed authentication")

// Sealer encrypts cached idempotency response bodies at rest.
//
// Idempotent replay has to return the original response, and for the OCR routes
// that response holds extracted identity fields (NIK, name, address, birth date).
// Persisting it verbatim contradicted the stated zero-PII-at-rest posture, so the
// body is sealed with AES-256-GCM before it reaches PostgreSQL. Anyone reading
// the table — a DBA, a backup, a stolen snapshot — sees ciphertext.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer derives an AES-256-GCM key from secret (typically SESSION_SECRET).
// It returns nil when secret is empty, which callers treat as "do not encrypt".
func NewSealer(secret []byte) (*Sealer, error) {
	if len(secret) == 0 {
		return nil, nil
	}

	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("creating aes cipher for idempotency sealer: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating gcm for idempotency sealer: %w", err)
	}

	return &Sealer{aead: aead}, nil
}

// Seal encrypts plaintext. A nil Sealer returns the input unchanged so that
// tests and local runs without a secret keep working.
func (s *Sealer) Seal(plaintext []byte) ([]byte, error) {
	if s == nil || s.aead == nil || len(plaintext) == 0 {
		return plaintext, nil
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating gcm nonce: %w", err)
	}

	out := make([]byte, 0, len(sealMagic)+len(nonce)+len(plaintext)+s.aead.Overhead())
	out = append(out, sealMagic...)
	out = append(out, nonce...)
	return s.aead.Seal(out, nonce, plaintext, nil), nil
}

// Open reverses Seal. Blobs without the magic prefix are returned as-is so rows
// written before encryption remain replayable.
func (s *Sealer) Open(blob []byte) ([]byte, error) {
	if len(blob) < len(sealMagic) || string(blob[:len(sealMagic)]) != string(sealMagic) {
		return blob, nil
	}
	if s == nil || s.aead == nil {
		return nil, ErrSealedPayloadCorrupt
	}

	body := blob[len(sealMagic):]
	if len(body) < s.aead.NonceSize() {
		return nil, ErrSealedPayloadCorrupt
	}

	nonce, ciphertext := body[:s.aead.NonceSize()], body[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrSealedPayloadCorrupt
	}
	return plaintext, nil
}
