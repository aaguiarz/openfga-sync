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
	os.Setenv("BACKEND_DSN", "postgres://test:test@localhost/test")
	os.Setenv("BACKEND_MODE", "changelog")
	os.Setenv("POLL_INTERVAL", "30s")
	defer func() {
		os.Unsetenv("OPENFGA_ENDPOINT")
		os.Unsetenv("OPENFGA_STORE_ID")
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
