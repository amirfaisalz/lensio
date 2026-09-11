package config_test

import (
	"os"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing relevant environment variables
	os.Unsetenv("PORT")
	os.Unsetenv("ENV")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LOG_LEVEL")

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
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/nusaid?sslmode=disable")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := config.Load()
	if cfg.Port != "9090" {
		t.Fatalf("expected Port '9090', got '%s'", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Fatalf("expected Env 'production', got '%s'", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/nusaid?sslmode=disable" {
		t.Fatalf("expected custom DatabaseURL, got '%s'", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected LogLevel 'debug', got '%s'", cfg.LogLevel)
	}
}
