package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	CORSOrigins []string
	Environment string

	// Token lifetimes. AccessTokenTTL keeps access JWTs short (default 15m).
	// RefreshIdleTTL is the sliding window an active session may go unused
	// before it dies (default 30d); RefreshAbsoluteTTL is the hard cap from
	// login regardless of activity (default 180d), forcing a periodic re-login.
	AccessTokenTTL     time.Duration
	RefreshIdleTTL     time.Duration
	RefreshAbsoluteTTL time.Duration

	// AppBaseURL is the public origin of the frontend (no trailing slash). Used
	// to build absolute links in outgoing email (e.g. the password-reset link).
	AppBaseURL string

	// GoogleClientID is the Google OAuth client ID used to verify GIS ID tokens
	// (the token audience). Empty means the google SSO provider is simply not
	// registered — the client hides the button independently via its own env.
	GoogleClientID string

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

		AccessTokenTTL:     getEnvDuration("BELJOT_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshIdleTTL:     getEnvDuration("BELJOT_REFRESH_IDLE_TTL", 30*24*time.Hour),
		RefreshAbsoluteTTL: getEnvDuration("BELJOT_REFRESH_ABSOLUTE_TTL", 180*24*time.Hour),

		AppBaseURL: strings.TrimRight(getEnv("BELJOT_APP_BASE_URL", "http://localhost:5173"), "/"),

		GoogleClientID: strings.TrimSpace(getEnv("BELJOT_GOOGLE_CLIENT_ID", "")),

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

	if cfg.Environment != "development" && cfg.GoogleClientID == "" {
		slog.Warn("BELJOT_GOOGLE_CLIENT_ID is not set in a non-development environment — Google sign-in will be unavailable")
	}

	// The idle window should not exceed the absolute cap; otherwise the cap is
	// the only thing that ever ends an active session and the sliding idle
	// window is meaningless.
	if cfg.RefreshIdleTTL > cfg.RefreshAbsoluteTTL {
		slog.Warn("BELJOT_REFRESH_IDLE_TTL exceeds BELJOT_REFRESH_ABSOLUTE_TTL — idle expiry will never fire before the absolute cap",
			"idleTTL", cfg.RefreshIdleTTL, "absoluteTTL", cfg.RefreshAbsoluteTTL)
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

// getEnvDuration parses a Go duration string (e.g. "15m", "720h") from the
// environment, falling back to the default on absence, a parse error, or a
// non-positive value (a zero/negative TTL would mint already-expired tokens).
// Note Go's time.ParseDuration has no day unit — express days as hours
// (30d = 720h).
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(value)); err == nil && d > 0 {
			return d
		}
		slog.Warn("invalid or non-positive duration env var, using fallback", "key", key, "value", value, "fallback", fallback)
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
