package config

import (
	"os"
	"strings"
)

// Config represents the application runtime configuration.
type Config struct {
	Port         string
	Env          string
	DatabaseURL  string
	LogLevel     string
	OCRProvider  string
	GeminiAPIKey        string
	GeminiModel         string
	KeycloakJWKSURL     string
	KeycloakIssuer      string
	KeycloakAudience    string
	SpiceDBEndpoint     string
	SpiceDBPresharedKey string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	env := strings.TrimSpace(os.Getenv("ENV"))
	if env == "" {
		env = "development"
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))

	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}

	ocrProvider := strings.TrimSpace(os.Getenv("OCR_PROVIDER"))
	if ocrProvider == "" {
		ocrProvider = "mock"
	}

	geminiAPIKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))

	geminiModel := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if geminiModel == "" {
		geminiModel = "gemini-2.0-flash"
	}

	keycloakJWKSURL := strings.TrimSpace(os.Getenv("KEYCLOAK_JWKS_URL"))
	keycloakIssuer := strings.TrimSpace(os.Getenv("KEYCLOAK_ISSUER"))
	if keycloakIssuer == "" && keycloakJWKSURL != "" && strings.HasSuffix(keycloakJWKSURL, "/protocol/openid-connect/certs") {
		keycloakIssuer = strings.TrimSuffix(keycloakJWKSURL, "/protocol/openid-connect/certs")
	}
	keycloakAudience := strings.TrimSpace(os.Getenv("KEYCLOAK_AUDIENCE"))

	spiceDBEndpoint := strings.TrimSpace(os.Getenv("SPICEDB_ENDPOINT"))
	spiceDBPresharedKey := strings.TrimSpace(os.Getenv("SPICEDB_PRESHARED_KEY"))

	return &Config{
		Port:                port,
		Env:                 env,
		DatabaseURL:         dbURL,
		LogLevel:            logLevel,
		OCRProvider:         ocrProvider,
		GeminiAPIKey:        geminiAPIKey,
		GeminiModel:         geminiModel,
		KeycloakJWKSURL:     keycloakJWKSURL,
		KeycloakIssuer:      keycloakIssuer,
		KeycloakAudience:    keycloakAudience,
		SpiceDBEndpoint:     spiceDBEndpoint,
		SpiceDBPresharedKey: spiceDBPresharedKey,
	}
}
