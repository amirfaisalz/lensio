package config

import (
	"os"
	"strconv"
	"strings"
)

// Config represents the application runtime configuration.
type Config struct {
	Port                string
	Env                 string
	DatabaseURL         string
	LogLevel            string
	OCRProvider         string
	GeminiAPIKey        string
	GeminiModel         string
	KeycloakJWKSURL     string
	KeycloakIssuer      string
	KeycloakAudience    string
	SpiceDBEndpoint     string
	SpiceDBPresharedKey string
	CORSAllowedOrigins  []string
	SessionSecret       string
	// RateLimitReplicas is how many API instances share the load. Token buckets
	// are per-process, so the plan limit is divided by this to approximate the
	// advertised rate. Defaults to 1.
	RateLimitReplicas int
	// EnableDevAuth switches on DevTokenValidator, which accepts unsigned JWTs.
	// It must be opted into explicitly; ENV alone is not enough to enable it.
	EnableDevAuth bool
	// SMTP transport for verification and password-reset mail. Without a host the
	// API cannot complete self-service signup, so production/staging refuse to start.
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	// AppBaseURL is the dashboard origin used to build links inside those emails.
	AppBaseURL string
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

	// Left empty on purpose: the provider owns its own default model so the
	// fallback is not duplicated in two packages that can drift apart.
	geminiModel := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))

	keycloakJWKSURL := strings.TrimSpace(os.Getenv("KEYCLOAK_JWKS_URL"))
	keycloakIssuer := strings.TrimSpace(os.Getenv("KEYCLOAK_ISSUER"))
	if keycloakIssuer == "" && keycloakJWKSURL != "" && strings.HasSuffix(keycloakJWKSURL, "/protocol/openid-connect/certs") {
		keycloakIssuer = strings.TrimSuffix(keycloakJWKSURL, "/protocol/openid-connect/certs")
	}
	keycloakAudience := strings.TrimSpace(os.Getenv("KEYCLOAK_AUDIENCE"))

	spiceDBEndpoint := strings.TrimSpace(os.Getenv("SPICEDB_ENDPOINT"))
	spiceDBPresharedKey := strings.TrimSpace(os.Getenv("SPICEDB_PRESHARED_KEY"))

	corsRaw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	var corsOrigins []string
	if corsRaw != "" {
		for _, part := range strings.Split(corsRaw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	} else if env == "development" || env == "test" {
		corsOrigins = []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:8080"}
	}

	rateLimitReplicas := 1
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_REPLICAS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			rateLimitReplicas = n
		}
	}

	// Default deny: only an explicit opt-in enables the unsigned-token dev
	// validator. Relying on ENV alone meant a single missing variable (ENV
	// defaults to "development") silently accepted forged tokens.
	enableDevAuth := strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_DEV_AUTH")), "true")

	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpPort := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if smtpPort == "" {
		smtpPort = "587"
	}
	smtpUsername := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := strings.TrimSpace(os.Getenv("SMTP_FROM"))

	appBaseURL := strings.TrimSpace(os.Getenv("APP_BASE_URL"))
	if appBaseURL == "" {
		appBaseURL = "http://localhost:5173"
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		// #nosec G101 -- default dev fallback secret
		sessionSecret = "lensio-session-secret-key-development-32b"
	}

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
		CORSAllowedOrigins:  corsOrigins,
		SessionSecret:       sessionSecret,
		RateLimitReplicas:   rateLimitReplicas,
		EnableDevAuth:       enableDevAuth,
		SMTPHost:            smtpHost,
		SMTPPort:            smtpPort,
		SMTPUsername:        smtpUsername,
		SMTPPassword:        smtpPassword,
		SMTPFrom:            smtpFrom,
		AppBaseURL:          appBaseURL,
	}
}
