package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	"github.com/sirupsen/logrus"
)

// OpenFGAAdapter implements StorageAdapter for writing to another OpenFGA instance
type OpenFGAAdapter struct {
	client         *client.OpenFgaClient
	targetStoreID  string
	logger         *logrus.Logger
	mode           config.StorageMode
	enableStateBak bool
	lastToken      string
	requestTimeout time.Duration
	maxRetries     int
	retryDelay     time.Duration
	batchSize      int
}

// OpenFGAConfig represents the configuration for OpenFGA adapter
type OpenFGAConfig struct {
	Endpoint             string     `json:"endpoint"`
	StoreID              string     `json:"store_id"`
	Token                string     `json:"token"`
	AuthorizationModelID string     `json:"authorization_model_id,omitempty"`
	RequestTimeout       string     `json:"request_timeout,omitempty"` // String format like "30s"
	MaxRetries           int        `json:"max_retries,omitempty"`
	RetryDelay           string     `json:"retry_delay,omitempty"` // String format like "1s"
	BatchSize            int        `json:"batch_size,omitempty"`
	OIDC                 OIDCConfig `json:"oidc,omitempty"`
}

// OIDCConfig contains OIDC authentication configuration
type OIDCConfig struct {
	Issuer       string   `json:"issuer"`
	Audience     string   `json:"audience"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	TokenIssuer  string   `json:"token_issuer"`
}

// NewOpenFGAAdapter creates a new OpenFGA storage adapter
func NewOpenFGAAdapter(dsn string, mode config.StorageMode, logger *logrus.Logger) (*OpenFGAAdapter, error) {
	// Parse DSN which should be in format: "openfga://endpoint/store_id?token=xxx&model_id=yyy"
	// For simplicity, we'll expect a JSON DSN format
	cfg, err := parseOpenFGADSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenFGA DSN: %w", err)
	}

	// Create OpenFGA client configuration
	configuration := &client.ClientConfiguration{
		ApiUrl:  cfg.Endpoint,
		StoreId: cfg.StoreID,
	}

	// Set up authentication - either token or OIDC
	if cfg.Token != "" {
		// Use API token authentication
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.Token,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create token credentials: %w", err)
		}
		configuration.Credentials = creds
	} else if cfg.OIDC.ClientID != "" && cfg.OIDC.ClientSecret != "" {
		// Use OIDC client credentials authentication
		credentialsConfig := &credentials.Config{
			ClientCredentialsClientId:       cfg.OIDC.ClientID,
			ClientCredentialsClientSecret:   cfg.OIDC.ClientSecret,
			ClientCredentialsApiTokenIssuer: cfg.OIDC.TokenIssuer,
			ClientCredentialsApiAudience:    cfg.OIDC.Audience,
		}

		// Add scopes if provided
		if len(cfg.OIDC.Scopes) > 0 {
			credentialsConfig.ClientCredentialsScopes = strings.Join(cfg.OIDC.Scopes, " ")
		}

		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodClientCredentials,
			Config: credentialsConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC credentials: %w", err)
		}
		configuration.Credentials = creds
	}

	// Set authorization model ID if provided
	if cfg.AuthorizationModelID != "" {
		configuration.AuthorizationModelId = cfg.AuthorizationModelID
	}

	// Create the OpenFGA client
	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenFGA client: %w", err)
	}

	// Set default values and parse durations
	requestTimeout := 30 * time.Second
	if cfg.RequestTimeout != "" {
		if parsed, err := time.ParseDuration(cfg.RequestTimeout); err == nil {
			requestTimeout = parsed
		} else {
			return nil, fmt.Errorf("invalid request_timeout format: %w", err)
		}
	}

	retryDelay := time.Second
	if cfg.RetryDelay != "" {
		if parsed, err := time.ParseDuration(cfg.RetryDelay); err == nil {
			retryDelay = parsed
		} else {
			return nil, fmt.Errorf("invalid retry_delay format: %w", err)
		}
	}

	maxRetries := 3
	if cfg.MaxRetries > 0 {
		maxRetries = cfg.MaxRetries
	}

	batchSize := 100
	if cfg.BatchSize > 0 {
		batchSize = cfg.BatchSize
	}

	adapter := &OpenFGAAdapter{
		client:         fgaClient,
		targetStoreID:  cfg.StoreID,
		logger:         logger,
		mode:           mode,
		requestTimeout: requestTimeout,
		maxRetries:     maxRetries,
		retryDelay:     retryDelay,
		batchSize:      batchSize,
	}

	// Test connection
	if err := adapter.testConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to target OpenFGA instance: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"target_store_id": cfg.StoreID,
		"target_endpoint": cfg.Endpoint,
		"storage_mode":    mode,
		"batch_size":      batchSize,
	}).Info("Successfully created OpenFGA storage adapter")

	return adapter, nil
}

// parseOpenFGADSN parses the OpenFGA DSN configuration string
// Supports two formats:
// 1. Simple: "endpoint/store_id" (e.g., "http://localhost:8080/store123")
// 2. JSON: {"endpoint":"http://localhost:8080","store_id":"store123","token":"token123"}
// 3. JSON with OIDC: {"endpoint":"...","store_id":"...","oidc":{"issuer":"...","audience":"...","client_id":"...","client_secret":"..."}}
func parseOpenFGADSN(dsn string) (*OpenFGAConfig, error) {
	// If DSN starts with {, treat it as JSON format
	if strings.HasPrefix(dsn, "{") {
		var cfg OpenFGAConfig
		if err := json.Unmarshal([]byte(dsn), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON DSN: %w", err)
		}

		// Validate required fields
		if cfg.Endpoint == "" || cfg.StoreID == "" {
			return nil, fmt.Errorf("JSON DSN must contain non-empty 'endpoint' and 'store_id' fields")
		}

		return &cfg, nil
	}

	// Simple format: endpoint/store_id
	// Find the last occurrence of '/' to properly split endpoint and store_id
	lastSlashIndex := strings.LastIndex(dsn, "/")
	if lastSlashIndex == -1 || lastSlashIndex == len(dsn)-1 {
		return nil, fmt.Errorf("invalid DSN format, expected endpoint/store_id")
	}

	endpoint := dsn[:lastSlashIndex]
	storeID := dsn[lastSlashIndex+1:]

	// Basic validation
	if endpoint == "" || storeID == "" {
		return nil, fmt.Errorf("invalid DSN format, both endpoint and store_id must be non-empty")
	}

	return &OpenFGAConfig{
		Endpoint: endpoint,
		StoreID:  storeID,
	}, nil
}

// testConnection tests the connection to the target OpenFGA instance
func (o *OpenFGAAdapter) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), o.requestTimeout)
	defer cancel()

	// Try to read from the store to test connectivity
	request := o.client.Read(ctx).Body(client.ClientReadRequest{})

	_, err := o.client.ReadExecute(request)
	if err != nil {
		return fmt.Errorf("failed to test connection: %w", err)
	}

	return nil
}

// WriteChanges writes a batch of change events to the target OpenFGA instance (changelog mode)
func (o *OpenFGAAdapter) WriteChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if len(changes) == 0 {
		return nil
	}

	if o.mode != config.StorageModeChangelog {
		return fmt.Errorf("WriteChanges is only supported in changelog mode")
	}

	// In changelog mode, we apply all changes as they happened historically
	return o.applyChangesWithRetry(ctx, changes)
}

// ApplyChanges applies a batch of changes to the target OpenFGA instance (stateful mode)
func (o *OpenFGAAdapter) ApplyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if len(changes) == 0 {
		return nil
	}

	if o.mode != config.StorageModeStateful {
		return fmt.Errorf("ApplyChanges is only supported in stateful mode")
	}

	// In stateful mode, we apply changes to maintain current state
	return o.applyChangesWithRetry(ctx, changes)
}

// applyChangesWithRetry applies changes with retry logic
func (o *OpenFGAAdapter) applyChangesWithRetry(ctx context.Context, changes []fetcher.ChangeEvent) error {
	var lastErr error

	for attempt := 0; attempt <= o.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(o.retryDelay * time.Duration(attempt)):
				// Continue with retry
			}
		}

		err := o.applyChanges(ctx, changes)
		if err == nil {
			if attempt > 0 {
				o.logger.WithFields(logrus.Fields{
					"attempt":       attempt + 1,
					"changes_count": len(changes),
				}).Info("Successfully applied changes after retry")
			}
			return nil
		}

		lastErr = err
		o.logger.WithFields(logrus.Fields{
			"attempt":       attempt + 1,
			"max_retries":   o.maxRetries,
			"changes_count": len(changes),
			"error":         err,
		}).Warn("Failed to apply changes, will retry")
	}

	return fmt.Errorf("failed to apply changes after %d attempts: %w", o.maxRetries+1, lastErr)
}

// applyChanges applies changes to the target OpenFGA instance
func (o *OpenFGAAdapter) applyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	// Process changes in batches
	for i := 0; i < len(changes); i += o.batchSize {
		end := i + o.batchSize
		if end > len(changes) {
			end = len(changes)
		}

		batch := changes[i:end]
		if err := o.processBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}
	}

	o.logger.WithField("changes_count", len(changes)).Info("Successfully applied all changes to target OpenFGA instance")
	return nil
}

// processBatch processes a batch of changes
func (o *OpenFGAAdapter) processBatch(ctx context.Context, changes []fetcher.ChangeEvent) error {
	// Separate writes and deletes
	var writes []client.ClientTupleKey
	var deletes []client.ClientTupleKeyWithoutCondition

	for _, change := range changes {
		tupleKey := o.convertToTupleKey(change)

		switch strings.ToUpper(change.Operation) {
		case "TUPLE_TO_USERSET_WRITE", "WRITE":
			writes = append(writes, tupleKey)
		case "TUPLE_TO_USERSET_DELETE", "DELETE":
			// Convert to delete format (without condition)
			deleteKey := client.ClientTupleKeyWithoutCondition{
				User:     tupleKey.User,
				Relation: tupleKey.Relation,
				Object:   tupleKey.Object,
			}
			deletes = append(deletes, deleteKey)
		default:
			o.logger.WithField("operation", change.Operation).Warn("Unknown operation type, skipping")
		}
	}

	// Apply writes and deletes
	if len(writes) > 0 || len(deletes) > 0 {
		return o.executeWrite(ctx, writes, deletes)
	}

	return nil
}

// convertToTupleKey converts a ChangeEvent to OpenFGA ClientTupleKey
func (o *OpenFGAAdapter) convertToTupleKey(change fetcher.ChangeEvent) client.ClientTupleKey {
	// Reconstruct the tuple from parsed components
	user := change.UserID
	if change.UserType != "" {
		user = change.UserType + ":" + change.UserID
	}

	object := change.ObjectID
	if change.ObjectType != "" {
		object = change.ObjectType + ":" + change.ObjectID
	}

	tupleKey := client.ClientTupleKey{
		User:     user,
		Relation: change.Relation,
		Object:   object,
	}

	// Handle condition if present
	if change.Condition != "" {
		condition, err := o.parseCondition(change.Condition)
		if err != nil {
			o.logger.WithFields(logrus.Fields{
				"error":     err.Error(),
				"condition": change.Condition,
			}).Warn("Failed to parse condition, proceeding without condition")
		} else if condition != nil {
			tupleKey.Condition = condition
		}
	}

	return tupleKey
}

// parseCondition converts a JSON string condition to RelationshipCondition
func (o *OpenFGAAdapter) parseCondition(conditionJSON string) (*openfga.RelationshipCondition, error) {
	if conditionJSON == "" {
		return nil, nil
	}

	// Parse the JSON string to extract condition data
	var conditionData map[string]interface{}
	if err := json.Unmarshal([]byte(conditionJSON), &conditionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal condition JSON: %w", err)
	}

	// Extract condition name (required)
	name, ok := conditionData["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("condition name is required and must be a string")
	}

	// Create RelationshipCondition
	condition := openfga.RelationshipCondition{
		Name: name,
	}

	// Extract context if present (optional)
	if contextData, ok := conditionData["context"]; ok && contextData != nil {
		if contextMap, ok := contextData.(map[string]interface{}); ok && len(contextMap) > 0 {
			condition.Context = &contextMap
		}
	}

	return &condition, nil
}

// executeWrite executes a write operation to OpenFGA
func (o *OpenFGAAdapter) executeWrite(ctx context.Context, writes []client.ClientTupleKey, deletes []client.ClientTupleKeyWithoutCondition) error {
	// Create the write request
	body := client.ClientWriteRequest{}

	if len(writes) > 0 {
		body.Writes = writes
	}

	if len(deletes) > 0 {
		body.Deletes = deletes
	}

	// Execute the write
	request := o.client.Write(ctx).Body(body)
	response, err := o.client.WriteExecute(request)
	if err != nil {
		return fmt.Errorf("failed to execute write: %w", err)
	}

	// Log the response
	o.logger.WithFields(logrus.Fields{
		"writes_count":  len(writes),
		"deletes_count": len(deletes),
		"response":      response,
	}).Debug("Successfully executed write operation")

	return nil
}

// GetLastContinuationToken retrieves the last processed continuation token
// Note: For OpenFGA adapter, we store this in memory (not persistent across restarts)
func (o *OpenFGAAdapter) GetLastContinuationToken(ctx context.Context) (string, error) {
	return o.lastToken, nil
}

// SaveContinuationToken saves the continuation token for resuming processing
// Note: For OpenFGA adapter, we store this in memory (not persistent across restarts)
func (o *OpenFGAAdapter) SaveContinuationToken(ctx context.Context, token string) error {
	o.lastToken = token
	o.logger.WithField("token", token).Debug("Saved continuation token")
	return nil
}

// Close closes the OpenFGA adapter (no-op for HTTP client)
func (o *OpenFGAAdapter) Close() error {
	o.logger.Info("Closing OpenFGA adapter")
	return nil
}

// GetStats returns statistics about the OpenFGA adapter
func (o *OpenFGAAdapter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"adapter_type":    "openfga",
		"target_store_id": o.targetStoreID,
		"storage_mode":    string(o.mode),
		"last_token":      o.lastToken,
		"request_timeout": o.requestTimeout.String(),
		"max_retries":     o.maxRetries,
		"batch_size":      o.batchSize,
	}

	// Try to get some basic stats from the target store if client is available
	if o.client != nil {
		testCtx, cancel := context.WithTimeout(ctx, o.requestTimeout)
		defer cancel()

		request := o.client.Read(testCtx).Body(client.ClientReadRequest{})

		_, err := o.client.ReadExecute(request)
		if err != nil {
			stats["connection_status"] = "error"
			stats["connection_error"] = err.Error()
		} else {
			stats["connection_status"] = "healthy"
		}
	} else {
		stats["connection_status"] = "error"
		stats["connection_error"] = "client not initialized"
	}

	return stats, nil
}
