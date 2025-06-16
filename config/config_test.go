package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigParsing(t *testing.T) {
	// Test YAML parsing
	yamlContent := `
server:
  port: 9090
openfga:
  endpoint: "https://test.openfga.com"
  store_id: "test-store-id"
  token: "test-token"
backend:
  type: "postgres"
  dsn: "postgres://test:test@localhost/test"
  mode: "stateful"
service:
  poll_interval: "10s"
  batch_size: 50
`

	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify YAML values
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.OpenFGA.Endpoint != "https://test.openfga.com" {
		t.Errorf("Expected endpoint https://test.openfga.com, got %s", cfg.OpenFGA.Endpoint)
	}
	if cfg.Backend.Mode != StorageModeStateful {
		t.Errorf("Expected stateful mode, got %s", cfg.Backend.Mode)
	}
	if cfg.Service.PollInterval != 10*time.Second {
		t.Errorf("Expected 10s poll interval, got %v", cfg.Service.PollInterval)
	}
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("OPENFGA_ENDPOINT", "https://env.openfga.com")
	os.Setenv("OPENFGA_STORE_ID", "test-store-id")
	os.Setenv("OPENFGA_TOKEN", "test-token") // Add authentication
	os.Setenv("BACKEND_DSN", "postgres://test:test@localhost/test")
	os.Setenv("BACKEND_MODE", "changelog")
	os.Setenv("POLL_INTERVAL", "30s")
	defer func() {
		os.Unsetenv("OPENFGA_ENDPOINT")
		os.Unsetenv("OPENFGA_STORE_ID")
		os.Unsetenv("OPENFGA_TOKEN")
		os.Unsetenv("BACKEND_DSN")
		os.Unsetenv("BACKEND_MODE")
		os.Unsetenv("POLL_INTERVAL")
	}()

	// Load config (will use defaults + env vars)
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variable overrides
	if cfg.OpenFGA.Endpoint != "https://env.openfga.com" {
		t.Errorf("Expected endpoint from env var, got %s", cfg.OpenFGA.Endpoint)
	}
	if cfg.Backend.Mode != StorageModeChangelog {
		t.Errorf("Expected changelog mode from env var, got %s", cfg.Backend.Mode)
	}
	if cfg.Service.PollInterval != 30*time.Second {
		t.Errorf("Expected 30s poll interval from env var, got %v", cfg.Service.PollInterval)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	// Set required fields for validation
	cfg.OpenFGA.StoreID = "test-store-id"
	cfg.Backend.DSN = "postgres://test:test@localhost/test"

	// Test valid config
	if err := cfg.validate(); err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}

	// Test invalid storage mode
	cfg.Backend.Mode = "invalid"
	if err := cfg.validate(); err == nil {
		t.Error("Expected validation error for invalid storage mode")
	}

	// Test missing required fields
	cfg = DefaultConfig()
	cfg.OpenFGA.Endpoint = ""
	if err := cfg.validate(); err == nil {
		t.Error("Expected validation error for missing endpoint")
	}
}

func TestStorageModeMethods(t *testing.T) {
	cfg := DefaultConfig()

	cfg.Backend.Mode = StorageModeChangelog
	if !cfg.IsChangelogMode() {
		t.Error("Expected IsChangelogMode() to return true")
	}
	if cfg.IsStatefulMode() {
		t.Error("Expected IsStatefulMode() to return false")
	}

	cfg.Backend.Mode = StorageModeStateful
	if cfg.IsChangelogMode() {
		t.Error("Expected IsChangelogMode() to return false")
	}
	if !cfg.IsStatefulMode() {
		t.Error("Expected IsStatefulMode() to return true")
	}
}

func TestOIDCConfiguration(t *testing.T) {
	// Test valid OIDC configuration
	cfg := DefaultConfig()
	cfg.OpenFGA.Token = "" // Remove token to test OIDC
	cfg.OpenFGA.OIDC = OIDCConfig{
		Issuer:       "https://test.auth0.com/",
		Audience:     "https://api.fga.dev/",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	cfg.OpenFGA.StoreID = "test-store-id"
	cfg.Backend.DSN = "postgres://test:test@localhost/test"

	if err := cfg.validate(); err != nil {
		t.Errorf("Valid OIDC config should pass validation, got error: %v", err)
	}

	// Test missing OIDC issuer
	cfg.OpenFGA.OIDC.Issuer = ""
	if err := cfg.validate(); err == nil {
		t.Error("Expected validation error for missing OIDC issuer")
	}

	// Test missing OIDC audience
	cfg.OpenFGA.OIDC.Issuer = "https://test.auth0.com/"
	cfg.OpenFGA.OIDC.Audience = ""
	if err := cfg.validate(); err == nil {
		t.Error("Expected validation error for missing OIDC audience")
	}

	// Test conflict between token and OIDC
	cfg.OpenFGA.Token = "test-token"
	cfg.OpenFGA.OIDC.Audience = "https://api.fga.dev/"
	if err := cfg.validate(); err == nil {
		t.Error("Expected validation error for both token and OIDC configured")
	}
}

func TestOIDCEnvironmentVariables(t *testing.T) {
	// Clear any existing token to avoid conflicts
	os.Setenv("OPENFGA_TOKEN", "")

	// Set OIDC environment variables
	os.Setenv("OPENFGA_ENDPOINT", "https://api.fga.dev")
	os.Setenv("OPENFGA_STORE_ID", "test-store-id")
	os.Setenv("OPENFGA_OIDC_ISSUER", "https://test.auth0.com/")
	os.Setenv("OPENFGA_OIDC_AUDIENCE", "https://api.fga.dev/")
	os.Setenv("OPENFGA_OIDC_CLIENT_ID", "test-client-id")
	os.Setenv("OPENFGA_OIDC_CLIENT_SECRET", "test-client-secret")
	os.Setenv("OPENFGA_OIDC_SCOPES", "read:tuples,write:tuples")
	os.Setenv("OPENFGA_OIDC_TOKEN_ISSUER", "https://test.auth0.com/")
	os.Setenv("BACKEND_DSN", "postgres://test:test@localhost/test")

	defer func() {
		os.Unsetenv("OPENFGA_TOKEN")
		os.Unsetenv("OPENFGA_ENDPOINT")
		os.Unsetenv("OPENFGA_STORE_ID")
		os.Unsetenv("OPENFGA_OIDC_ISSUER")
		os.Unsetenv("OPENFGA_OIDC_AUDIENCE")
		os.Unsetenv("OPENFGA_OIDC_CLIENT_ID")
		os.Unsetenv("OPENFGA_OIDC_CLIENT_SECRET")
		os.Unsetenv("OPENFGA_OIDC_SCOPES")
		os.Unsetenv("OPENFGA_OIDC_TOKEN_ISSUER")
		os.Unsetenv("BACKEND_DSN")
	}()

	// Load config with OIDC env vars
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load config with OIDC env vars: %v", err)
	}

	// Verify OIDC configuration from environment variables
	if cfg.OpenFGA.OIDC.Issuer != "https://test.auth0.com/" {
		t.Errorf("Expected OIDC issuer from env var, got %s", cfg.OpenFGA.OIDC.Issuer)
	}
	if cfg.OpenFGA.OIDC.ClientID != "test-client-id" {
		t.Errorf("Expected OIDC client ID from env var, got %s", cfg.OpenFGA.OIDC.ClientID)
	}
	if len(cfg.OpenFGA.OIDC.Scopes) != 2 || cfg.OpenFGA.OIDC.Scopes[0] != "read:tuples" {
		t.Errorf("Expected OIDC scopes from env var, got %v", cfg.OpenFGA.OIDC.Scopes)
	}
}
