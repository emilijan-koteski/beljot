package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	CORSOrigins []string
	Environment string

	// AppBaseURL is the public origin of the frontend (no trailing slash). Used
	// to build absolute links in outgoing email (e.g. the password-reset link).
	AppBaseURL string

	// SMTP settings for outgoing transactional email. When SMTPConfigured() is
	// false the app falls back to a log-only mailer (see cmd/api wiring).
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
}

// SMTPConfigured reports whether enough SMTP settings are present to send real
// email. Missing any of host/username/password means we run the log-only
// fallback instead.
func (c *Config) SMTPConfigured() bool {
	return c.SMTPHost != "" && c.SMTPUsername != "" && c.SMTPPassword != ""
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL: getEnv("BELJOT_DB_URL", "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"),
		JWTSecret:   getEnv("BELJOT_JWT_SECRET", "change-me-in-production"),
		Port:        getEnv("BELJOT_PORT", "8080"),
		CORSOrigins: parseOrigins(getEnv("BELJOT_CORS_ORIGINS", "http://localhost:5173")),
		Environment: getEnv("BELJOT_ENV", "development"),

		AppBaseURL: strings.TrimRight(getEnv("BELJOT_APP_BASE_URL", "http://localhost:5173"), "/"),

		SMTPHost:     strings.TrimSpace(getEnv("BELJOT_SMTP_HOST", "")),
		SMTPPort:     getEnvInt("BELJOT_SMTP_PORT", 587),
		SMTPUsername: strings.TrimSpace(getEnv("BELJOT_SMTP_USERNAME", "")),
		// Gmail App passwords are shown as four space-separated groups but must be
		// sent without spaces — strip all whitespace so a value pasted verbatim
		// ("abcd efgh ijkl mnop") still authenticates.
		SMTPPassword: stripWhitespace(getEnv("BELJOT_SMTP_PASSWORD", "")),
		SMTPFrom:     strings.TrimSpace(getEnv("BELJOT_SMTP_FROM", "")),
		SMTPFromName: getEnv("BELJOT_SMTP_FROM_NAME", "Beljot.online"),
	}

	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		if cfg.Environment != "development" {
			slog.Error("BELJOT_JWT_SECRET must be set to a secure value in non-development environments")
			os.Exit(1)
		}
		slog.Warn("BELJOT_JWT_SECRET is not set or uses the default value — do not deploy to production without changing it")
	}

	if cfg.Environment != "development" && (cfg.AppBaseURL == "" || strings.Contains(cfg.AppBaseURL, "localhost")) {
		slog.Warn("BELJOT_APP_BASE_URL is unset or points at localhost in a non-development environment — password reset links will be broken", "appBaseURL", cfg.AppBaseURL)
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return n
		}
		slog.Warn("invalid integer env var, using fallback", "key", key, "value", value, "fallback", fallback)
	}
	return fallback
}

// stripWhitespace removes every whitespace rune from s. Used for the SMTP
// password so a Gmail App password pasted with its display spaces still works.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
