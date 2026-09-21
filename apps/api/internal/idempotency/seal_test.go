package idempotency_test

import (
	"bytes"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
)

// A cached OCR response body, i.e. exactly the shape of the data that must never
// sit in the database in the clear.
var piiBody = []byte(`{"data":{"nik":"3174012345678901","nama":"BUDI SANTOSO","alamat":"JL MERDEKA 1"}}`)

func TestSealer_RoundTripsAndHidesPlaintext(t *testing.T) {
	sealer, err := idempotency.NewSealer([]byte("a-test-session-secret"))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	sealed, err := sealer.Seal(piiBody)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if bytes.Contains(sealed, []byte("3174012345678901")) || bytes.Contains(sealed, []byte("BUDI SANTOSO")) {
		t.Fatal("sealed blob still contains plaintext PII")
	}

	opened, err := sealer.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(opened, piiBody) {
		t.Fatalf("round trip mismatch: got %q", opened)
	}
}

func TestSealer_NonceIsFresh(t *testing.T) {
	sealer, _ := idempotency.NewSealer([]byte("a-test-session-secret"))
	a, _ := sealer.Seal(piiBody)
	b, _ := sealer.Seal(piiBody)
	if bytes.Equal(a, b) {
		t.Fatal("identical plaintext produced identical ciphertext; nonce is not fresh")
	}
}

func TestSealer_RejectsWrongKey(t *testing.T) {
	good, _ := idempotency.NewSealer([]byte("a-test-session-secret"))
	other, _ := idempotency.NewSealer([]byte("a-different-secret"))

	sealed, _ := good.Seal(piiBody)
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("expected authentication failure when opening with a different key")
	}
}

func TestSealer_TamperedCiphertextIsRejected(t *testing.T) {
	sealer, _ := idempotency.NewSealer([]byte("a-test-session-secret"))
	sealed, _ := sealer.Seal(piiBody)

	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 0xFF
	if _, err := sealer.Open(tampered); err == nil {
		t.Fatal("expected GCM to reject a tampered blob")
	}
}

// Rows written before encryption carry no magic prefix and must stay replayable.
func TestSealer_ReadsLegacyPlaintextRows(t *testing.T) {
	sealer, _ := idempotency.NewSealer([]byte("a-test-session-secret"))
	opened, err := sealer.Open(piiBody)
	if err != nil {
		t.Fatalf("legacy plaintext row should open: %v", err)
	}
	if !bytes.Equal(opened, piiBody) {
		t.Fatal("legacy plaintext row was altered")
	}
}

// A nil Sealer is the "encryption disabled" path used by tests.
func TestSealer_NilIsPassThrough(t *testing.T) {
	var sealer *idempotency.Sealer
	sealed, err := sealer.Seal(piiBody)
	if err != nil || !bytes.Equal(sealed, piiBody) {
		t.Fatalf("nil sealer must pass through: %v %q", err, sealed)
	}
	opened, err := sealer.Open(piiBody)
	if err != nil || !bytes.Equal(opened, piiBody) {
		t.Fatalf("nil sealer must pass through on open: %v %q", err, opened)
	}
}

func TestSealer_EmptySecretDisablesEncryption(t *testing.T) {
	sealer, err := idempotency.NewSealer(nil)
	if err != nil {
		t.Fatalf("NewSealer(nil): %v", err)
	}
	if sealer != nil {
		t.Fatal("expected nil sealer for an empty secret")
	}
}
