package apikey_test

import (
	"strings"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
)

func TestGenerate_Success(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		wantPrefix string
		wantEnv    string
	}{
		{
			name:       "default to live when empty",
			env:        "",
			wantPrefix: "nusa_live_",
			wantEnv:    "live",
		},
		{
			name:       "explicit live environment",
			env:        "live",
			wantPrefix: "nusa_live_",
			wantEnv:    "live",
		},
		{
			name:       "explicit test environment",
			env:        "test",
			wantPrefix: "nusa_test_",
			wantEnv:    "test",
		},
		{
			name:       "trimmed and uppercase normalized",
			env:        "  LIVE  ",
			wantPrefix: "nusa_live_",
			wantEnv:    "live",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, err := apikey.Generate(tc.env)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k == nil {
				t.Fatal("expected non-nil GeneratedKey")
			}
			if k.Environment != tc.wantEnv {
				t.Errorf("expected environment %q, got %q", tc.wantEnv, k.Environment)
			}
			if !strings.HasPrefix(k.Plaintext, tc.wantPrefix) {
				t.Errorf("expected plaintext to start with %q, got %q", tc.wantPrefix, k.Plaintext)
			}
			if len(k.KeyHash) != 64 {
				t.Errorf("expected 64-char key hash, got %d", len(k.KeyHash))
			}
			if !strings.HasPrefix(k.Prefix, tc.wantPrefix) {
				t.Errorf("expected prefix to start with %q, got %q", tc.wantPrefix, k.Prefix)
			}
			if len(k.Prefix) != len(tc.wantPrefix)+4 {
				t.Errorf("expected prefix length %d, got %d", len(tc.wantPrefix)+4, len(k.Prefix))
			}

			// Ensure Hash matches plaintext
			expectedHash := apikey.Hash(k.Plaintext)
			if k.KeyHash != expectedHash {
				t.Errorf("hash mismatch: expected %q, got %q", expectedHash, k.KeyHash)
			}

			// Validate generated key passes ValidateFormat
			gotEnv, valid := apikey.ValidateFormat(k.Plaintext)
			if !valid {
				t.Errorf("expected generated key %q to be valid", k.Plaintext)
			}
			if gotEnv != tc.wantEnv {
				t.Errorf("expected validated env %q, got %q", tc.wantEnv, gotEnv)
			}
		})
	}
}

func TestGenerate_InvalidEnv(t *testing.T) {
	invalidEnvs := []string{"prod", "staging", "dev", "123", "live!"}
	for _, env := range invalidEnvs {
		t.Run(env, func(t *testing.T) {
			k, err := apikey.Generate(env)
			if err == nil {
				t.Fatalf("expected error for invalid environment %q, got key: %+v", env, k)
			}
			if k != nil {
				t.Errorf("expected nil key on error, got %+v", k)
			}
		})
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	seenKeys := make(map[string]bool, 100)
	seenHashes := make(map[string]bool, 100)

	for i := 0; i < 100; i++ {
		k, err := apikey.Generate(apikey.EnvLive)
		if err != nil {
			t.Fatalf("failed generating key: %v", err)
		}
		if seenKeys[k.Plaintext] {
			t.Fatalf("duplicate plaintext key generated: %s", k.Plaintext)
		}
		if seenHashes[k.KeyHash] {
			t.Fatalf("duplicate key hash generated: %s", k.KeyHash)
		}
		seenKeys[k.Plaintext] = true
		seenHashes[k.KeyHash] = true
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{prefix: "nusa_live_1234", want: "nusa_live_1234••••••••"},
		{prefix: "nusa_test_abcd", want: "nusa_test_abcd••••••••"},
		{prefix: "", want: "••••••••"},
	}

	for _, tc := range tests {
		got := apikey.Mask(tc.prefix)
		if got != tc.want {
			t.Errorf("Mask(%q) = %q, want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestValidateFormat(t *testing.T) {
	validLive, _ := apikey.Generate(apikey.EnvLive)
	validTest, _ := apikey.Generate(apikey.EnvTest)

	tests := []struct {
		name      string
		key       string
		wantEnv   string
		wantValid bool
	}{
		{
			name:      "valid live key",
			key:       validLive.Plaintext,
			wantEnv:   apikey.EnvLive,
			wantValid: true,
		},
		{
			name:      "valid test key",
			key:       validTest.Plaintext,
			wantEnv:   apikey.EnvTest,
			wantValid: true,
		},
		{
			name:      "invalid prefix",
			key:       "invalid_live_" + strings.Repeat("a", 64),
			wantEnv:   "",
			wantValid: false,
		},
		{
			name:      "too short token",
			key:       "nusa_live_abcdef",
			wantEnv:   "",
			wantValid: false,
		},
		{
			name:      "too long token",
			key:       "nusa_live_" + strings.Repeat("a", 65),
			wantEnv:   "",
			wantValid: false,
		},
		{
			name:      "non-hex characters in live key",
			key:       "nusa_live_" + strings.Repeat("z", 64),
			wantEnv:   "",
			wantValid: false,
		},
		{
			name:      "non-hex characters in test key",
			key:       "nusa_test_" + strings.Repeat("g", 64),
			wantEnv:   "",
			wantValid: false,
		},
		{
			name:      "empty string",
			key:       "",
			wantEnv:   "",
			wantValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, valid := apikey.ValidateFormat(tc.key)
			if valid != tc.wantValid {
				t.Errorf("ValidateFormat(%q) valid = %v, want %v", tc.key, valid, tc.wantValid)
			}
			if env != tc.wantEnv {
				t.Errorf("ValidateFormat(%q) env = %q, want %q", tc.key, env, tc.wantEnv)
			}
		})
	}
}

func BenchmarkAPIKeyGenerate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := apikey.Generate(apikey.EnvLive)
		if err != nil {
			b.Fatalf("failed generate: %v", err)
		}
	}
}

func BenchmarkAPIKeyHash(b *testing.B) {
	k, _ := apikey.Generate(apikey.EnvLive)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = apikey.Hash(k.Plaintext)
	}
}
