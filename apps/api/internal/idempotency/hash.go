package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DefaultMaxBodySize limits buffered payload read size to 10MB to prevent memory exhaustion.
const DefaultMaxBodySize int64 = 10 * 1024 * 1024

// ComputePayloadHash reads the request body up to maxBytes, computes its SHA-256 hash,
// and resets r.Body with an io.NopCloser so downstream handlers can read it normally.
func ComputePayloadHash(r *http.Request, maxBytes int64) (string, error) {
	if r == nil {
		return "", errors.New("http request is nil")
	}
	if r.Body == nil || r.Body == http.NoBody {
		h := sha256.Sum256(nil)
		return hex.EncodeToString(h[:]), nil
	}

	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodySize
	}

	limitedReader := io.LimitReader(r.Body, maxBytes+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("reading request body: %w", err)
	}

	if int64(len(bodyBytes)) > maxBytes {
		return "", fmt.Errorf("request body exceeds maximum allowed size of %d bytes", maxBytes)
	}

	// Restore request body for downstream handlers
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	hash := sha256.Sum256(bodyBytes)
	return hex.EncodeToString(hash[:]), nil
}
