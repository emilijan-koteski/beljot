package config

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestStripWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"gmail app password with spaces", "ytbk awjy liwm pyzm", "ytbkawjyliwmpyzm"},
		{"already clean", "abcd1234", "abcd1234"},
		{"tabs and newlines", "ab\tcd\nef ", "abcdef"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripWhitespace(tc.in); got != tc.want {
				t.Errorf("stripWhitespace(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSMTPConfigured(t *testing.T) {
	tests := []struct {
		name                 string
		host, user, password string
		want                 bool
	}{
		{"all set", "smtp.gmail.com", "u@x.com", "pw", true},
		{"missing password", "smtp.gmail.com", "u@x.com", "", false},
		{"missing host", "", "u@x.com", "pw", false},
		{"missing username", "smtp.gmail.com", "", "pw", false},
		{"all empty", "", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{SMTPHost: tc.host, SMTPUsername: tc.user, SMTPPassword: tc.password}
			if got := c.SMTPConfigured(); got != tc.want {
				t.Errorf("SMTPConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}

// avatarEnv is a complete, valid avatar-storage environment. Tests delete or
// blank one entry to exercise a single missing variable.
func avatarEnv() map[string]string {
	return map[string]string{
		"BELJOT_S3_ENDPOINT":       "http://localhost:3900",
		"BELJOT_S3_REGION":         "garage",
		"BELJOT_S3_ACCESS_KEY":     "GKtest",
		"BELJOT_S3_SECRET_KEY":     "secret",
		"BELJOT_S3_PUBLIC_BUCKET":  "beljot-public",
		"BELJOT_PUBLIC_ASSETS_URL": "http://assets.localhost:3902/",
	}
}

func TestAvatarStorageConfigured(t *testing.T) {
	full := Config{
		S3Endpoint: "http://localhost:3900", S3Region: "garage", S3AccessKey: "k",
		S3SecretKey: "s", S3PublicBucket: "b", PublicAssetsURL: "http://a",
	}
	if !full.AvatarStorageConfigured() {
		t.Fatal("AvatarStorageConfigured() = false with all six set")
	}
	blank := []func(*Config){
		func(c *Config) { c.S3Endpoint = "" },
		func(c *Config) { c.S3Region = "" },
		func(c *Config) { c.S3AccessKey = "" },
		func(c *Config) { c.S3SecretKey = "" },
		func(c *Config) { c.S3PublicBucket = "" },
		func(c *Config) { c.PublicAssetsURL = "" },
	}
	for i, f := range blank {
		c := full
		f(&c)
		if c.AvatarStorageConfigured() {
			t.Errorf("case %d: AvatarStorageConfigured() = true with one variable blank", i)
		}
	}
}

func TestLoad_AvatarStorageTrimsPublicURL(t *testing.T) {
	t.Setenv("BELJOT_ENV", "development")
	for k, v := range avatarEnv() {
		t.Setenv(k, v)
	}
	cfg := Load()
	if cfg.PublicAssetsURL != "http://assets.localhost:3902" {
		t.Errorf("PublicAssetsURL = %q, want the trailing slash trimmed", cfg.PublicAssetsURL)
	}
	if !cfg.AvatarStorageConfigured() {
		t.Error("AvatarStorageConfigured() = false with all six variables set")
	}
}

func TestLoad_DevelopmentWithoutEndpointDisablesAvatars(t *testing.T) {
	t.Setenv("BELJOT_ENV", "development")
	for k, v := range avatarEnv() {
		t.Setenv(k, v)
	}
	t.Setenv("BELJOT_S3_ENDPOINT", "")
	cfg := Load() // must not exit
	if cfg.AvatarStorageConfigured() {
		t.Error("AvatarStorageConfigured() = true with an empty endpoint")
	}
}

// TestLoad_ProductionMissingAvatarVarExits re-runs this test binary as a child
// process, because Load calls os.Exit(1) on the failure path.
func TestLoad_ProductionMissingAvatarVarExits(t *testing.T) {
	if name := os.Getenv("BELJOT_CONFIG_EXIT_CHILD"); name != "" {
		env := avatarEnv()
		delete(env, name)
		for k, v := range env {
			os.Setenv(k, v)
		}
		os.Unsetenv(name)
		os.Setenv("BELJOT_ENV", "production")
		os.Setenv("BELJOT_JWT_SECRET", "a-secure-test-secret")
		Load()
		os.Exit(0) // not reached when Load exits as it should
	}

	for name := range avatarEnv() {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLoad_ProductionMissingAvatarVarExits$")
			cmd.Env = append(os.Environ(), "BELJOT_CONFIG_EXIT_CHILD="+name)
			out, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("child exit = %v, want status 1; output:\n%s", err, out)
			}
			if !strings.Contains(string(out), name) {
				t.Errorf("output does not name %s:\n%s", name, out)
			}
		})
	}
}
