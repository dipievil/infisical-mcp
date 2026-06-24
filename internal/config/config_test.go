package config_test

import (
	"os"
	"testing"

	"github.com/dipievil/infisical-mcp/internal/config"
)

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"INFISICAL_HOST":          "http://192.168.1.10:8080",
		"INFISICAL_CLIENT_ID":     "test-client-id",
		"INFISICAL_CLIENT_SECRET": "test-client-secret",
		"INFISICAL_PROJECT_ID":    "test-project-id",
		"INFISICAL_ENVIRONMENT":   "dev",
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.InfisicalHost != "http://192.168.1.10:8080" {
		t.Errorf("unexpected host: %q", cfg.InfisicalHost)
	}
	if cfg.ClientID != "test-client-id" {
		t.Errorf("unexpected client ID: %q", cfg.ClientID)
	}
	if cfg.SafeMode {
		t.Error("expected SafeMode to be false by default")
	}
}

func TestLoad_TrailingSlashStripped(t *testing.T) {
	env := validEnv()
	env["INFISICAL_HOST"] = "http://192.168.1.10:8080/"
	setEnv(t, env)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InfisicalHost != "http://192.168.1.10:8080" {
		t.Errorf("trailing slash not stripped: %q", cfg.InfisicalHost)
	}
}

func TestLoad_SafeModeTrue(t *testing.T) {
	env := validEnv()
	env["INFISICAL_SAFE_MODE"] = "true"
	setEnv(t, env)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.SafeMode {
		t.Error("expected SafeMode to be true")
	}
}

func TestLoad_SafeModeCaseInsensitive(t *testing.T) {
	env := validEnv()
	env["INFISICAL_SAFE_MODE"] = "TRUE"
	setEnv(t, env)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.SafeMode {
		t.Error("expected SafeMode to be true with uppercase TRUE")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	required := []string{
		"INFISICAL_HOST",
		"INFISICAL_CLIENT_ID",
		"INFISICAL_CLIENT_SECRET",
		"INFISICAL_PROJECT_ID",
		"INFISICAL_ENVIRONMENT",
	}

	for _, missing := range required {
		t.Run("missing_"+missing, func(t *testing.T) {
			env := validEnv()
			delete(env, missing)
			for k, v := range env {
				t.Setenv(k, v)
			}
			// Unset the missing one explicitly
			os.Unsetenv(missing) //nolint:errcheck

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error when %s is missing", missing)
			}
		})
	}
}

func TestErrSafeModeEnabled(t *testing.T) {
	if config.ErrSafeModeEnabled == nil {
		t.Fatal("ErrSafeModeEnabled should not be nil")
	}
	if config.ErrSafeModeEnabled.Error() == "" {
		t.Error("ErrSafeModeEnabled should have a non-empty message")
	}
}
