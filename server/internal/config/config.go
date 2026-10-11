package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Config holds the server settings read from BELJOT_* environment variables.
type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	CORSOrigins []string
	Environment string

	AccessTokenTTL     time.Duration
	RefreshIdleTTL     time.Duration
	RefreshAbsoluteTTL time.Duration

	AppBaseURL string

	GoogleClientID string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string

	S3Endpoint      string
	S3Region        string
	S3AccessKey     string
	S3SecretKey     string
	S3PublicBucket  string
	PublicAssetsURL string
}

// SMTPConfigured reports whether enough SMTP settings are present to send real
// email. Missing any of host/username/password means we run the log-only
// fallback instead.
func (c *Config) SMTPConfigured() bool {
	return c.SMTPHost != "" && c.SMTPUsername != "" && c.SMTPPassword != ""
}

// AvatarStorageConfigured reports whether every avatar-storage setting is
// present. When false the server runs without an object store and both avatar
// endpoints answer 503 (development only; Load exits outside it).
func (c *Config) AvatarStorageConfigured() bool {
	return len(c.missingAvatarStorageVars()) == 0
}

// missingAvatarStorageVars names every unset avatar-storage variable, in the
// order they are documented.
func (c *Config) missingAvatarStorageVars() []string {
	vars := []struct {
		name  string
		value string
	}{
		{"BELJOT_S3_ENDPOINT", c.S3Endpoint},
		{"BELJOT_S3_REGION", c.S3Region},
		{"BELJOT_S3_ACCESS_KEY", c.S3AccessKey},
		{"BELJOT_S3_SECRET_KEY", c.S3SecretKey},
		{"BELJOT_S3_PUBLIC_BUCKET", c.S3PublicBucket},
		{"BELJOT_PUBLIC_ASSETS_URL", c.PublicAssetsURL},
	}
	var missing []string
	for _, v := range vars {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	return missing
}

// Load reads the config from the environment and exits when a required
// production setting, such as the JWT secret or avatar storage, is missing.
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
		SMTPPassword: stripWhitespace(getEnv("BELJOT_SMTP_PASSWORD", "")),
		SMTPFrom:     strings.TrimSpace(getEnv("BELJOT_SMTP_FROM", "")),
		SMTPFromName: getEnv("BELJOT_SMTP_FROM_NAME", "Beljot.online"),

		S3Endpoint:      strings.TrimSpace(getEnv("BELJOT_S3_ENDPOINT", "")),
		S3Region:        strings.TrimSpace(getEnv("BELJOT_S3_REGION", "")),
		S3AccessKey:     strings.TrimSpace(getEnv("BELJOT_S3_ACCESS_KEY", "")),
		S3SecretKey:     strings.TrimSpace(getEnv("BELJOT_S3_SECRET_KEY", "")),
		S3PublicBucket:  strings.TrimSpace(getEnv("BELJOT_S3_PUBLIC_BUCKET", "")),
		PublicAssetsURL: strings.TrimRight(strings.TrimSpace(getEnv("BELJOT_PUBLIC_ASSETS_URL", "")), "/"),
	}

	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		if cfg.Environment != "development" {
			slog.Error("BELJOT_JWT_SECRET must be set to a secure value in non-development environments")
			os.Exit(1)
		}
		slog.Warn("BELJOT_JWT_SECRET is not set or uses the default value — do not deploy to production without changing it")
	}

	if missing := cfg.missingAvatarStorageVars(); len(missing) > 0 {
		if cfg.Environment != "development" {
			for _, name := range missing {
				slog.Error(name+" must be set in non-development environments", "variable", name)
			}
			os.Exit(1)
		}
		if cfg.S3Endpoint != "" {
			slog.Warn("avatar storage is partly configured — avatar uploads are disabled", "missing", missing)
		}
	}

	if cfg.Environment != "development" && (cfg.AppBaseURL == "" || strings.Contains(cfg.AppBaseURL, "localhost")) {
		slog.Warn("BELJOT_APP_BASE_URL is unset or points at localhost in a non-development environment — password reset links will be broken", "appBaseURL", cfg.AppBaseURL)
	}

	if cfg.Environment != "development" && cfg.GoogleClientID == "" {
		slog.Warn("BELJOT_GOOGLE_CLIENT_ID is not set in a non-development environment — Google sign-in will be unavailable")
	}

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

// getEnvDuration parses a Go duration such as "15m" or "720h" (there is no day
// unit) from the environment, falling back when it is unset, invalid or not positive.
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
