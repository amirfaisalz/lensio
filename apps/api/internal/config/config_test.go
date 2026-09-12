package config_test

import (
	"os"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing relevant environment variables
	os.Unsetenv("PORT")
	os.Unsetenv("ENV")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("OCR_PROVIDER")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GEMINI_MODEL")
	os.Unsetenv("KEYCLOAK_JWKS_URL")
	os.Unsetenv("KEYCLOAK_ISSUER")
	os.Unsetenv("KEYCLOAK_AUDIENCE")
	os.Unsetenv("SPICEDB_ENDPOINT")
	os.Unsetenv("SPICEDB_PRESHARED_KEY")

	cfg := config.Load()
	if cfg.Port != "8080" {
		t.Fatalf("expected Port '8080', got '%s'", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Fatalf("expected Env 'development', got '%s'", cfg.Env)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty DatabaseURL, got '%s'", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.OCRProvider != "mock" {
		t.Fatalf("expected OCRProvider 'mock', got '%s'", cfg.OCRProvider)
	}
	if cfg.GeminiAPIKey != "" {
		t.Fatalf("expected empty GeminiAPIKey, got '%s'", cfg.GeminiAPIKey)
	}
	if cfg.GeminiModel != "gemini-3.6-flash" {
		t.Fatalf("expected GeminiModel 'gemini-3.6-flash', got '%s'", cfg.GeminiModel)
	}
	if cfg.KeycloakJWKSURL != "" {
		t.Fatalf("expected empty KeycloakJWKSURL, got '%s'", cfg.KeycloakJWKSURL)
	}
	if cfg.KeycloakIssuer != "" {
		t.Fatalf("expected empty KeycloakIssuer, got '%s'", cfg.KeycloakIssuer)
	}
	if cfg.KeycloakAudience != "" {
		t.Fatalf("expected empty KeycloakAudience, got '%s'", cfg.KeycloakAudience)
	}
	if cfg.SpiceDBEndpoint != "" {
		t.Fatalf("expected empty SpiceDBEndpoint, got '%s'", cfg.SpiceDBEndpoint)
	}
	if cfg.SpiceDBPresharedKey != "" {
		t.Fatalf("expected empty SpiceDBPresharedKey, got '%s'", cfg.SpiceDBPresharedKey)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/lensio?sslmode=disable")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("OCR_PROVIDER", "gemini_flash")
	t.Setenv("GEMINI_API_KEY", "test-key-123")
	t.Setenv("GEMINI_MODEL", "gemini-1.5-flash")
	t.Setenv("KEYCLOAK_JWKS_URL", "http://keycloak:8080/realms/lensio/protocol/openid-connect/certs")
	t.Setenv("KEYCLOAK_AUDIENCE", "lensio-api")
	t.Setenv("SPICEDB_ENDPOINT", "http://spicedb:8443")
	t.Setenv("SPICEDB_PRESHARED_KEY", "lensio_spicedb_key")

	cfg := config.Load()
	if cfg.Port != "9090" {
		t.Fatalf("expected Port '9090', got '%s'", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Fatalf("expected Env 'production', got '%s'", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/lensio?sslmode=disable" {
		t.Fatalf("expected custom DatabaseURL, got '%s'", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected LogLevel 'debug', got '%s'", cfg.LogLevel)
	}
	if cfg.OCRProvider != "gemini_flash" {
		t.Fatalf("expected OCRProvider 'gemini_flash', got '%s'", cfg.OCRProvider)
	}
	if cfg.GeminiAPIKey != "test-key-123" {
		t.Fatalf("expected GeminiAPIKey 'test-key-123', got '%s'", cfg.GeminiAPIKey)
	}
	if cfg.GeminiModel != "gemini-1.5-flash" {
		t.Fatalf("expected GeminiModel 'gemini-1.5-flash', got '%s'", cfg.GeminiModel)
	}
	if cfg.KeycloakJWKSURL != "http://keycloak:8080/realms/lensio/protocol/openid-connect/certs" {
		t.Fatalf("expected KeycloakJWKSURL, got '%s'", cfg.KeycloakJWKSURL)
	}
	if cfg.KeycloakIssuer != "http://keycloak:8080/realms/lensio" {
		t.Fatalf("expected derived KeycloakIssuer 'http://keycloak:8080/realms/lensio', got '%s'", cfg.KeycloakIssuer)
	}
	if cfg.KeycloakAudience != "lensio-api" {
		t.Fatalf("expected KeycloakAudience 'lensio-api', got '%s'", cfg.KeycloakAudience)
	}
	if cfg.SpiceDBEndpoint != "http://spicedb:8443" {
		t.Fatalf("expected SpiceDBEndpoint 'http://spicedb:8443', got '%s'", cfg.SpiceDBEndpoint)
	}
	if cfg.SpiceDBPresharedKey != "lensio_spicedb_key" {
		t.Fatalf("expected SpiceDBPresharedKey 'lensio_spicedb_key', got '%s'", cfg.SpiceDBPresharedKey)
	}
}
