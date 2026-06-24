// Package config provides configuration management for the Infisical MCP server.
// Configuration is loaded from environment variables, with optional support for
// a .env file in the working directory via the INFISICAL_MCP_ENV_FILE variable.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds the runtime configuration for the MCP server.
type Config struct {
	// InfisicalHost is the base URL of the Infisical instance (e.g. http://192.168.1.10:8080).
	InfisicalHost string

	// ClientID is the Universal Auth client ID used to obtain an access token.
	ClientID string

	// ClientSecret is the Universal Auth client secret used to obtain an access token.
	ClientSecret string

	// ProjectID is the Infisical project (workspace) ID to operate on.
	ProjectID string

	// Environment is the Infisical environment slug (e.g. "dev", "staging", "prod").
	Environment string

	// SafeMode controls whether set_secret is allowed.
	// When false (default), set_secret is enabled.
	// When true, set_secret calls are rejected.
	SafeMode bool
}

// Load reads configuration from environment variables.
//
// Required variables:
//   - INFISICAL_HOST        – base URL of the Infisical instance
//   - INFISICAL_CLIENT_ID   – Universal Auth client ID
//   - INFISICAL_CLIENT_SECRET – Universal Auth client secret
//   - INFISICAL_PROJECT_ID  – project / workspace ID
//   - INFISICAL_ENVIRONMENT – environment slug (e.g. dev)
//
// Optional variables:
//   - INFISICAL_SAFE_MODE   – set to "true" to disable set_secret (default: false)
func Load() (*Config, error) {
	cfg := &Config{
		InfisicalHost: strings.TrimRight(getenv("INFISICAL_HOST", ""), "/"),
		ClientID:      os.Getenv("INFISICAL_CLIENT_ID"),
		ClientSecret:  os.Getenv("INFISICAL_CLIENT_SECRET"),
		ProjectID:     os.Getenv("INFISICAL_PROJECT_ID"),
		Environment:   os.Getenv("INFISICAL_ENVIRONMENT"),
		SafeMode:      strings.ToLower(os.Getenv("INFISICAL_SAFE_MODE")) == "true",
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that all required fields are non-empty.
func (c *Config) validate() error {
	var missing []string

	if c.InfisicalHost == "" {
		missing = append(missing, "INFISICAL_HOST")
	}
	if c.ClientID == "" {
		missing = append(missing, "INFISICAL_CLIENT_ID")
	}
	if c.ClientSecret == "" {
		missing = append(missing, "INFISICAL_CLIENT_SECRET")
	}
	if c.ProjectID == "" {
		missing = append(missing, "INFISICAL_PROJECT_ID")
	}
	if c.Environment == "" {
		missing = append(missing, "INFISICAL_ENVIRONMENT")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// getenv returns the value of the environment variable named key, falling back
// to fallback if the variable is not set or is empty.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// ErrSafeModeEnabled is returned when set_secret is called while safe mode is on.
var ErrSafeModeEnabled = errors.New("safe mode is enabled: write operations are not allowed")
