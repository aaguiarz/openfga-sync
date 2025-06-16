package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// StorageMode represents the storage mode for the service
type StorageMode string

const (
	StorageModeChangelog StorageMode = "changelog"
	StorageModeStateful  StorageMode = "stateful"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	OpenFGA       OpenFGAConfig       `yaml:"openfga"`
	Backend       BackendConfig       `yaml:"backend"`
	Logging       LoggingConfig       `yaml:"logging"`
	Observability ObservabilityConfig `yaml:"observability"`
	Service       ServiceConfig       `yaml:"service"`
	Leadership    LeadershipConfig    `yaml:"leadership"`
}

// ServerConfig contains server-specific configuration
type ServerConfig struct {
	Port int `yaml:"port" env:"SERVER_PORT"`
}

// OpenFGAConfig contains OpenFGA-specific configuration
type OpenFGAConfig struct {
	Endpoint string     `yaml:"endpoint" env:"OPENFGA_ENDPOINT"`
	Token    string     `yaml:"token" env:"OPENFGA_TOKEN"`
	StoreID  string     `yaml:"store_id" env:"OPENFGA_STORE_ID"`
	OIDC     OIDCConfig `yaml:"oidc"`
}

// OIDCConfig contains OIDC authentication configuration for OpenFGA
type OIDCConfig struct {
	Issuer       string   `yaml:"issuer" env:"OPENFGA_OIDC_ISSUER"`
	Audience     string   `yaml:"audience" env:"OPENFGA_OIDC_AUDIENCE"`
	ClientID     string   `yaml:"client_id" env:"OPENFGA_OIDC_CLIENT_ID"`
	ClientSecret string   `yaml:"client_secret" env:"OPENFGA_OIDC_CLIENT_SECRET"`
	Scopes       []string `yaml:"scopes" env:"OPENFGA_OIDC_SCOPES"`
	TokenIssuer  string   `yaml:"token_issuer" env:"OPENFGA_OIDC_TOKEN_ISSUER"`
}

// BackendConfig contains backend storage configuration
type BackendConfig struct {
	Type string      `yaml:"type" env:"BACKEND_TYPE"`
	DSN  string      `yaml:"dsn" env:"BACKEND_DSN"`
	Mode StorageMode `yaml:"mode" env:"BACKEND_MODE"`
}

// LoggingConfig contains logging-specific configuration
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
}

// ObservabilityConfig contains observability configuration
type ObservabilityConfig struct {
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry"`
	Metrics       MetricsConfig       `yaml:"metrics"`
}

// OpenTelemetryConfig contains OpenTelemetry configuration
type OpenTelemetryConfig struct {
	Endpoint    string `yaml:"endpoint" env:"OTEL_ENDPOINT"`
	ServiceName string `yaml:"service_name" env:"OTEL_SERVICE_NAME"`
	Enabled     bool   `yaml:"enabled" env:"OTEL_ENABLED"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" env:"METRICS_ENABLED"`
	Path    string `yaml:"path" env:"METRICS_PATH"`
}

// ServiceConfig contains service-specific configuration
type ServiceConfig struct {
	PollInterval     time.Duration `yaml:"poll_interval" env:"POLL_INTERVAL"`
	BatchSize        int32         `yaml:"batch_size" env:"BATCH_SIZE"`
	MaxRetries       int           `yaml:"max_retries" env:"MAX_RETRIES"`
	RetryDelay       time.Duration `yaml:"retry_delay" env:"RETRY_DELAY"`
	MaxChanges       int           `yaml:"max_changes" env:"MAX_CHANGES"`
	RequestTimeout   time.Duration `yaml:"request_timeout" env:"REQUEST_TIMEOUT"`
	MaxRetryDelay    time.Duration `yaml:"max_retry_delay" env:"MAX_RETRY_DELAY"`
	BackoffFactor    float64       `yaml:"backoff_factor" env:"BACKOFF_FACTOR"`
	RateLimitDelay   time.Duration `yaml:"rate_limit_delay" env:"RATE_LIMIT_DELAY"`
	EnableValidation bool          `yaml:"enable_validation" env:"ENABLE_VALIDATION"`
}

// LeadershipConfig contains leader election configuration
type LeadershipConfig struct {
	Enabled   bool   `yaml:"enabled" env:"LEADERSHIP_ENABLED"`
	Namespace string `yaml:"namespace" env:"LEADERSHIP_NAMESPACE"`
	LockName  string `yaml:"lock_name" env:"LEADERSHIP_LOCK_NAME"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		OpenFGA: OpenFGAConfig{
			Endpoint: "http://localhost:8080",
			Token:    "development-token", // Default token for development
		},
		Backend: BackendConfig{
			Type: "postgres",
			Mode: StorageModeChangelog,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Observability: ObservabilityConfig{
			OpenTelemetry: OpenTelemetryConfig{
				Endpoint:    "http://localhost:4318",
				ServiceName: "openfga-sync",
				Enabled:     false,
			},
			Metrics: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
			},
		},
		Service: ServiceConfig{
			PollInterval:     5 * time.Second,
			BatchSize:        100,
			MaxRetries:       3,
			RetryDelay:       1 * time.Second,
			MaxChanges:       0, // No limit by default
			RequestTimeout:   30 * time.Second,
			MaxRetryDelay:    5 * time.Second,
			BackoffFactor:    2.0,
			RateLimitDelay:   50 * time.Millisecond,
			EnableValidation: true,
		},
		Leadership: LeadershipConfig{
			Enabled:   false,
			Namespace: "default",
			LockName:  "openfga-sync-leader",
		},
	}
}

// LoadConfig loads configuration from YAML file and environment variables
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	// Load from YAML file if provided
	if path != "" {
		if err := loadFromYAML(config, path); err != nil {
			return nil, fmt.Errorf("failed to load YAML config: %w", err)
		}
	}

	// Override with environment variables
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load environment config: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadFromYAML loads configuration from a YAML file
func loadFromYAML(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, continue with defaults
			return nil
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) error {
	// Server configuration
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}

	// OpenFGA configuration
	if endpoint := os.Getenv("OPENFGA_ENDPOINT"); endpoint != "" {
		config.OpenFGA.Endpoint = endpoint
	}
	if token, exists := os.LookupEnv("OPENFGA_TOKEN"); exists {
		config.OpenFGA.Token = token // Allow empty string to clear default token
	}
	if storeID := os.Getenv("OPENFGA_STORE_ID"); storeID != "" {
		config.OpenFGA.StoreID = storeID
	}

	// OpenFGA OIDC configuration
	if issuer := os.Getenv("OPENFGA_OIDC_ISSUER"); issuer != "" {
		config.OpenFGA.OIDC.Issuer = issuer
	}
	if audience := os.Getenv("OPENFGA_OIDC_AUDIENCE"); audience != "" {
		config.OpenFGA.OIDC.Audience = audience
	}
	if clientID := os.Getenv("OPENFGA_OIDC_CLIENT_ID"); clientID != "" {
		config.OpenFGA.OIDC.ClientID = clientID
	}
	if clientSecret := os.Getenv("OPENFGA_OIDC_CLIENT_SECRET"); clientSecret != "" {
		config.OpenFGA.OIDC.ClientSecret = clientSecret
	}
	if scopes := os.Getenv("OPENFGA_OIDC_SCOPES"); scopes != "" {
		config.OpenFGA.OIDC.Scopes = strings.Split(scopes, ",")
		// Trim whitespace from each scope
		for i, scope := range config.OpenFGA.OIDC.Scopes {
			config.OpenFGA.OIDC.Scopes[i] = strings.TrimSpace(scope)
		}
	}
	if tokenIssuer := os.Getenv("OPENFGA_OIDC_TOKEN_ISSUER"); tokenIssuer != "" {
		config.OpenFGA.OIDC.TokenIssuer = tokenIssuer
	}

	// Backend configuration
	if backendType := os.Getenv("BACKEND_TYPE"); backendType != "" {
		config.Backend.Type = backendType
	}
	if dsn := os.Getenv("BACKEND_DSN"); dsn != "" {
		config.Backend.DSN = dsn
	}
	if mode := os.Getenv("BACKEND_MODE"); mode != "" {
		config.Backend.Mode = StorageMode(mode)
	}

	// Logging configuration
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}

	// OpenTelemetry configuration
	if endpoint := os.Getenv("OTEL_ENDPOINT"); endpoint != "" {
		config.Observability.OpenTelemetry.Endpoint = endpoint
	}
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		config.Observability.OpenTelemetry.ServiceName = serviceName
	}
	if enabled := os.Getenv("OTEL_ENABLED"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			config.Observability.OpenTelemetry.Enabled = e
		}
	}

	// Metrics configuration
	if enabled := os.Getenv("METRICS_ENABLED"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			config.Observability.Metrics.Enabled = e
		}
	}
	if path := os.Getenv("METRICS_PATH"); path != "" {
		config.Observability.Metrics.Path = path
	}

	// Service configuration
	if pollInterval := os.Getenv("POLL_INTERVAL"); pollInterval != "" {
		if p, err := time.ParseDuration(pollInterval); err == nil {
			config.Service.PollInterval = p
		}
	}
	if batchSize := os.Getenv("BATCH_SIZE"); batchSize != "" {
		if b, err := strconv.ParseInt(batchSize, 10, 32); err == nil {
			config.Service.BatchSize = int32(b)
		}
	}
	if maxRetries := os.Getenv("MAX_RETRIES"); maxRetries != "" {
		if m, err := strconv.Atoi(maxRetries); err == nil {
			config.Service.MaxRetries = m
		}
	}
	if retryDelay := os.Getenv("RETRY_DELAY"); retryDelay != "" {
		if r, err := time.ParseDuration(retryDelay); err == nil {
			config.Service.RetryDelay = r
		}
	}
	if maxChanges := os.Getenv("MAX_CHANGES"); maxChanges != "" {
		if m, err := strconv.Atoi(maxChanges); err == nil {
			config.Service.MaxChanges = m
		}
	}
	if requestTimeout := os.Getenv("REQUEST_TIMEOUT"); requestTimeout != "" {
		if r, err := time.ParseDuration(requestTimeout); err == nil {
			config.Service.RequestTimeout = r
		}
	}
	if maxRetryDelay := os.Getenv("MAX_RETRY_DELAY"); maxRetryDelay != "" {
		if m, err := time.ParseDuration(maxRetryDelay); err == nil {
			config.Service.MaxRetryDelay = m
		}
	}
	if backoffFactor := os.Getenv("BACKOFF_FACTOR"); backoffFactor != "" {
		if b, err := strconv.ParseFloat(backoffFactor, 64); err == nil {
			config.Service.BackoffFactor = b
		}
	}
	if rateLimitDelay := os.Getenv("RATE_LIMIT_DELAY"); rateLimitDelay != "" {
		if r, err := time.ParseDuration(rateLimitDelay); err == nil {
			config.Service.RateLimitDelay = r
		}
	}
	if enableValidation := os.Getenv("ENABLE_VALIDATION"); enableValidation != "" {
		if e, err := strconv.ParseBool(enableValidation); err == nil {
			config.Service.EnableValidation = e
		}
	}

	// Leadership configuration
	if enabled := os.Getenv("LEADERSHIP_ENABLED"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			config.Leadership.Enabled = e
		}
	}
	if namespace := os.Getenv("LEADERSHIP_NAMESPACE"); namespace != "" {
		config.Leadership.Namespace = namespace
	}
	if lockName := os.Getenv("LEADERSHIP_LOCK_NAME"); lockName != "" {
		config.Leadership.LockName = lockName
	}

	return nil
}

// validate validates the configuration
func (c *Config) validate() error {
	var errors []string

	// Validate OpenFGA configuration
	if c.OpenFGA.Endpoint == "" {
		errors = append(errors, "openfga.endpoint is required")
	}
	if c.OpenFGA.StoreID == "" {
		errors = append(errors, "openfga.store_id is required")
	}

	// Validate OpenFGA authentication: either token or OIDC config must be provided
	hasToken := c.OpenFGA.Token != ""
	hasOIDC := c.OpenFGA.OIDC.ClientID != "" && c.OpenFGA.OIDC.ClientSecret != ""

	if !hasToken && !hasOIDC {
		errors = append(errors, "OpenFGA authentication required: either 'token' or OIDC configuration (client_id and client_secret) must be provided")
	}

	if hasToken && hasOIDC {
		errors = append(errors, "OpenFGA authentication conflict: provide either 'token' or OIDC configuration, not both")
	}

	// Validate OIDC configuration if provided
	if hasOIDC {
		if c.OpenFGA.OIDC.Issuer == "" {
			errors = append(errors, "openfga.oidc.issuer is required when using OIDC authentication")
		}
		if c.OpenFGA.OIDC.Audience == "" {
			errors = append(errors, "openfga.oidc.audience is required when using OIDC authentication")
		}
	}

	// Validate backend configuration
	if c.Backend.Type == "" {
		errors = append(errors, "backend.type is required")
	}
	if c.Backend.DSN == "" {
		errors = append(errors, "backend.dsn is required")
	}
	if c.Backend.Mode != StorageModeChangelog && c.Backend.Mode != StorageModeStateful {
		errors = append(errors, "backend.mode must be 'changelog' or 'stateful'")
	}

	// Validate logging configuration
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	if !contains(validLogLevels, c.Logging.Level) {
		errors = append(errors, "logging.level must be one of: debug, info, warn, error, fatal, panic")
	}

	validLogFormats := []string{"text", "json"}
	if !contains(validLogFormats, c.Logging.Format) {
		errors = append(errors, "logging.format must be 'text' or 'json'")
	}

	// Validate service configuration
	if c.Service.PollInterval <= 0 {
		errors = append(errors, "service.poll_interval must be positive")
	}
	if c.Service.BatchSize <= 0 {
		errors = append(errors, "service.batch_size must be positive")
	}
	if c.Service.MaxRetries < 0 {
		errors = append(errors, "service.max_retries must be non-negative")
	}
	if c.Service.RetryDelay < 0 {
		errors = append(errors, "service.retry_delay must be non-negative")
	}
	if c.Service.MaxChanges < 0 {
		errors = append(errors, "service.max_changes must be non-negative")
	}
	if c.Service.RequestTimeout <= 0 {
		errors = append(errors, "service.request_timeout must be positive")
	}
	if c.Service.MaxRetryDelay < 0 {
		errors = append(errors, "service.max_retry_delay must be non-negative")
	}
	if c.Service.BackoffFactor <= 0 {
		errors = append(errors, "service.backoff_factor must be positive")
	}
	if c.Service.RateLimitDelay < 0 {
		errors = append(errors, "service.rate_limit_delay must be non-negative")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, ", "))
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// IsChangelogMode returns true if the storage mode is changelog
func (c *Config) IsChangelogMode() bool {
	return c.Backend.Mode == StorageModeChangelog
}

// IsStatefulMode returns true if the storage mode is stateful
func (c *Config) IsStatefulMode() bool {
	return c.Backend.Mode == StorageModeStateful
}
