package idempotency_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
)

func TestComputePayloadHash_NilRequest(t *testing.T) {
	_, err := idempotency.ComputePayloadHash(nil, 1024)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestComputePayloadHash_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	hash, err := idempotency.ComputePayloadHash(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash for empty body")
	}
}

func TestComputePayloadHash_ConsistencyAndRestoration(t *testing.T) {
	content := []byte("test payload for idempotency hashing 12345")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(content))

	hash1, err := idempotency.ComputePayloadHash(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error on first hash: %v", err)
	}

	// Verify request body can still be read completely after hashing
	restoredBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed reading restored body: %v", err)
	}
	if !bytes.Equal(restoredBody, content) {
		t.Fatalf("body content altered: expected %q, got %q", content, restoredBody)
	}

	// Reset body and rehash to verify deterministic hash
	req.Body = io.NopCloser(bytes.NewReader(content))
	hash2, err := idempotency.ComputePayloadHash(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error on second hash: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected matching hashes, got %s vs %s", hash1, hash2)
	}

	// Different payload must produce different hash
	diffReq := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("different payload")))
	diffHash, err := idempotency.ComputePayloadHash(diffReq, 1024)
	if err != nil {
		t.Fatalf("unexpected error on diff hash: %v", err)
	}
	if hash1 == diffHash {
		t.Fatalf("expected different hashes for different payloads, got same: %s", hash1)
	}
}

func TestComputePayloadHash_ExceedsMaxSize(t *testing.T) {
	largeContent := []byte(strings.Repeat("a", 2048))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(largeContent))

	_, err := idempotency.ComputePayloadHash(req, 1024)
	if err == nil {
		t.Fatal("expected error when payload exceeds max size, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum allowed size") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
