package config

import (
	"os"
	"strings"
)

// Config represents the application runtime configuration.
type Config struct {
	Port        string
	Env         string
	DatabaseURL string
	LogLevel    string
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

	return &Config{
		Port:        port,
		Env:         env,
		DatabaseURL: dbURL,
		LogLevel:    logLevel,
	}
}
