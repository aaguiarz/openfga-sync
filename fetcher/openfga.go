package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RetryConfig configures retry behavior for OpenFGA API calls
type RetryConfig struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
}

// DefaultRetryConfig provides sensible defaults for retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// FetchOptions provides advanced options for fetching changes
type FetchOptions struct {
	PageSize         int32         `json:"page_size"`
	MaxChanges       int           `json:"max_changes"`
	Timeout          time.Duration `json:"timeout"`
	RetryConfig      RetryConfig   `json:"retry_config"`
	RateLimitDelay   time.Duration `json:"rate_limit_delay"`
	ConcurrentPages  int           `json:"concurrent_pages"`
	EnableValidation bool          `json:"enable_validation"`
}

// DefaultFetchOptions provides sensible defaults
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		PageSize:         100,
		MaxChanges:       0, // No limit
		Timeout:          30 * time.Second,
		RetryConfig:      DefaultRetryConfig(),
		RateLimitDelay:   50 * time.Millisecond,
		ConcurrentPages:  1, // Sequential by default
		EnableValidation: true,
	}
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

// ChangeEvent represents a change event from OpenFGA
type ChangeEvent struct {
	// Parsed fields
	ObjectType string    `json:"object_type"`
	ObjectID   string    `json:"object_id"`
	Relation   string    `json:"relation"`
	UserType   string    `json:"user_type"`
	UserID     string    `json:"user_id"`
	ChangeType string    `json:"change_type"`
	Timestamp  time.Time `json:"timestamp"`
	Condition  string    `json:"condition,omitempty"` // Relationship condition (optional)
	RawJSON    string    `json:"raw_json"`            // Raw JSON from OpenFGA

	// Legacy fields for compatibility
	TupleKey  TupleKey `json:"tuple_key"`
	Operation string   `json:"operation"`
}

// TupleKey represents a tuple key from OpenFGA with parsed user (legacy compatibility)
type TupleKey struct {
	User       string `json:"user"`
	UserType   string `json:"user_type"`
	UserID     string `json:"user_id"`
	Relation   string `json:"relation"`
	Object     string `json:"object"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

// FetchResult represents the result of a fetch operation
type FetchResult struct {
	Changes           []ChangeEvent `json:"changes"`
	ContinuationToken string        `json:"continuation_token"`
	HasMore           bool          `json:"has_more"`
	TotalFetched      int           `json:"total_fetched"`
}

// OpenFGAFetcher handles fetching changes from OpenFGA
type OpenFGAFetcher struct {
	client      *client.OpenFgaClient
	storeID     string
	logger      *logrus.Logger
	options     FetchOptions
	rateLimiter *time.Ticker
	mutex       sync.RWMutex
	stats       FetcherStats
}

// FetcherStats tracks statistics about fetch operations
type FetcherStats struct {
	TotalRequests   int64     `json:"total_requests"`
	SuccessRequests int64     `json:"success_requests"`
	FailedRequests  int64     `json:"failed_requests"`
	TotalChanges    int64     `json:"total_changes"`
	LastFetchTime   time.Time `json:"last_fetch_time"`
	AverageLatency  float64   `json:"average_latency_ms"`
}

// NewOpenFGAFetcher creates a new OpenFGA fetcher
func NewOpenFGAFetcher(apiURL, storeID, apiToken string, logger *logrus.Logger) (*OpenFGAFetcher, error) {
	return NewOpenFGAFetcherWithOptions(apiURL, storeID, apiToken, logger, DefaultFetchOptions())
}

// NewOpenFGAFetcherWithOIDC creates a new OpenFGA fetcher with OIDC authentication
func NewOpenFGAFetcherWithOIDC(apiURL, storeID string, oidcConfig OIDCConfig, logger *logrus.Logger) (*OpenFGAFetcher, error) {
	return NewOpenFGAFetcherWithOIDCAndOptions(apiURL, storeID, oidcConfig, logger, DefaultFetchOptions())
}

// NewOpenFGAFetcherWithOIDCAndOptions creates a new OpenFGA fetcher with OIDC authentication and custom options
func NewOpenFGAFetcherWithOIDCAndOptions(apiURL, storeID string, oidcConfig OIDCConfig, logger *logrus.Logger, options FetchOptions) (*OpenFGAFetcher, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl:  apiURL,
		StoreId: storeID,
	}

	// Set up OIDC credentials
	credentialsConfig := &credentials.Config{
		ClientCredentialsClientId:       oidcConfig.ClientID,
		ClientCredentialsClientSecret:   oidcConfig.ClientSecret,
		ClientCredentialsApiTokenIssuer: oidcConfig.TokenIssuer,
		ClientCredentialsApiAudience:    oidcConfig.Audience,
	}

	// Add scopes if provided
	if len(oidcConfig.Scopes) > 0 {
		credentialsConfig.ClientCredentialsScopes = strings.Join(oidcConfig.Scopes, " ")
	}

	creds, err := credentials.NewCredentials(credentials.Credentials{
		Method: credentials.CredentialsMethodClientCredentials,
		Config: credentialsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC credentials: %w", err)
	}
	configuration.Credentials = creds

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenFGA client with OIDC: %w", err)
	}

	var rateLimiter *time.Ticker
	if options.RateLimitDelay > 0 {
		rateLimiter = time.NewTicker(options.RateLimitDelay)
	}

	return &OpenFGAFetcher{
		client:      fgaClient,
		storeID:     storeID,
		logger:      logger,
		options:     options,
		rateLimiter: rateLimiter,
		stats:       FetcherStats{},
	}, nil
}

// NewOpenFGAFetcherWithOptions creates a new OpenFGA fetcher with custom options
func NewOpenFGAFetcherWithOptions(apiURL, storeID, apiToken string, logger *logrus.Logger, options FetchOptions) (*OpenFGAFetcher, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl:  apiURL,
		StoreId: storeID,
	}

	if apiToken != "" {
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: apiToken,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create credentials: %w", err)
		}
		configuration.Credentials = creds
	}

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenFGA client: %w", err)
	}

	var rateLimiter *time.Ticker
	if options.RateLimitDelay > 0 {
		rateLimiter = time.NewTicker(options.RateLimitDelay)
	}

	return &OpenFGAFetcher{
		client:      fgaClient,
		storeID:     storeID,
		logger:      logger,
		options:     options,
		rateLimiter: rateLimiter,
		stats:       FetcherStats{},
	}, nil
}

// GetStats returns current fetcher statistics
func (f *OpenFGAFetcher) GetStats() FetcherStats {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.stats
}

// Close cleans up resources like rate limiter
func (f *OpenFGAFetcher) Close() {
	if f.rateLimiter != nil {
		f.rateLimiter.Stop()
	}
}

// UpdateOptions updates the fetcher options
func (f *OpenFGAFetcher) UpdateOptions(options FetchOptions) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.options = options

	// Update rate limiter if needed
	if f.rateLimiter != nil {
		f.rateLimiter.Stop()
	}

	if options.RateLimitDelay > 0 {
		f.rateLimiter = time.NewTicker(options.RateLimitDelay)
	}
}

// FetchChanges fetches changes from OpenFGA starting from a continuation token
func (f *OpenFGAFetcher) FetchChanges(ctx context.Context, continuationToken string) ([]ChangeEvent, string, error) {
	result, err := f.FetchChangesWithPaging(ctx, continuationToken, 0)
	if err != nil {
		return nil, "", err
	}
	return result.Changes, result.ContinuationToken, nil
}

// FetchChangesWithPaging fetches changes with enhanced paging support
func (f *OpenFGAFetcher) FetchChangesWithPaging(ctx context.Context, continuationToken string, pageSize int32) (*FetchResult, error) {
	// Start OpenTelemetry span
	tracer := otel.Tracer("openfga-sync/fetcher")
	ctx, span := tracer.Start(ctx, "openfga.fetch_changes",
		trace.WithAttributes(
			attribute.String("openfga.store_id", f.storeID),
			attribute.String("openfga.continuation_token", continuationToken),
			attribute.Int64("openfga.page_size", int64(pageSize)),
		),
	)
	defer span.End()

	f.logger.WithFields(logrus.Fields{
		"continuation_token": continuationToken,
		"page_size":          pageSize,
	}).Debug("Fetching changes from OpenFGA with paging")

	options := client.ClientReadChangesOptions{}
	if continuationToken != "" {
		options.ContinuationToken = &continuationToken
	}
	if pageSize > 0 {
		options.PageSize = &pageSize
	}

	response, err := f.client.ReadChanges(ctx).Options(options).Execute()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.message", err.Error()))
		return nil, fmt.Errorf("failed to fetch changes: %w", err)
	}

	var changes []ChangeEvent
	for _, change := range response.Changes {
		changeEvent, err := f.parseChangeEvent(change)
		if err != nil {
			f.logger.WithError(err).Warn("Failed to parse change event, skipping")
			continue
		}
		changes = append(changes, changeEvent)
	}

	nextToken := ""
	hasMore := false
	if response.ContinuationToken != nil {
		nextToken = *response.ContinuationToken
		hasMore = nextToken != ""
	}

	result := &FetchResult{
		Changes:           changes,
		ContinuationToken: nextToken,
		HasMore:           hasMore,
		TotalFetched:      len(changes),
	}

	// Add span attributes for the result
	span.SetAttributes(
		attribute.Int("openfga.changes_count", len(changes)),
		attribute.String("openfga.next_token", nextToken),
		attribute.Bool("openfga.has_more", hasMore),
	)

	f.logger.WithFields(logrus.Fields{
		"changes_count": len(changes),
		"next_token":    nextToken,
		"has_more":      hasMore,
	}).Info("Successfully fetched changes from OpenFGA")

	return result, nil
}

// FetchAllChanges fetches all available changes by automatically handling pagination
func (f *OpenFGAFetcher) FetchAllChanges(ctx context.Context, startToken string, maxChanges int) (*FetchResult, error) {
	f.logger.WithFields(logrus.Fields{
		"start_token": startToken,
		"max_changes": maxChanges,
	}).Info("Starting to fetch all changes with automatic pagination")

	var allChanges []ChangeEvent
	currentToken := startToken
	totalFetched := 0

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check if we've reached the maximum changes limit
		if maxChanges > 0 && totalFetched >= maxChanges {
			f.logger.WithField("total_fetched", totalFetched).Info("Reached maximum changes limit")
			break
		}

		// Fetch the next batch
		result, err := f.FetchChangesWithPaging(ctx, currentToken, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch changes batch: %w", err)
		}

		// Add changes to our collection
		allChanges = append(allChanges, result.Changes...)
		totalFetched += len(result.Changes)

		// Check if we have more changes
		if !result.HasMore || result.ContinuationToken == "" {
			f.logger.WithField("total_fetched", totalFetched).Info("No more changes available")
			break
		}

		// Update token for next iteration
		currentToken = result.ContinuationToken

		f.logger.WithFields(logrus.Fields{
			"batch_size":    len(result.Changes),
			"total_fetched": totalFetched,
			"next_token":    currentToken,
		}).Debug("Processed batch, continuing pagination")
	}

	return &FetchResult{
		Changes:           allChanges,
		ContinuationToken: currentToken,
		HasMore:           false, // We've fetched all available
		TotalFetched:      totalFetched,
	}, nil
}

// parseChangeEvent converts an OpenFGA change to our ChangeEvent struct
func (f *OpenFGAFetcher) parseChangeEvent(change interface{}) (ChangeEvent, error) {
	// First, serialize the entire change to JSON for raw storage
	rawJSON, err := json.Marshal(change)
	if err != nil {
		return ChangeEvent{}, fmt.Errorf("failed to marshal change to JSON: %w", err)
	}

	// Handle the SDK's actual response structure
	// The OpenFGA SDK returns a structured response, not a map
	var user, relation, object, operation string
	var timestamp time.Time

	// Try to extract fields using reflection or type assertions
	// This handles the actual OpenFGA SDK response structure
	changeBytes, err := json.Marshal(change)
	if err != nil {
		return ChangeEvent{}, fmt.Errorf("failed to marshal change for parsing: %w", err)
	}

	// Parse into a generic map to extract fields
	var changeMap map[string]interface{}
	if err := json.Unmarshal(changeBytes, &changeMap); err != nil {
		return ChangeEvent{}, fmt.Errorf("failed to unmarshal change: %w", err)
	}

	// Extract operation
	if op, ok := changeMap["operation"]; ok {
		operation = fmt.Sprintf("%v", op)
	}

	// Extract timestamp
	if ts, ok := changeMap["timestamp"]; ok {
		if tsStr, ok := ts.(string); ok {
			if parsed, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				timestamp = parsed
			}
		} else if tsTime, ok := ts.(time.Time); ok {
			timestamp = tsTime
		}
	}

	// Extract tuple key information
	var condition string
	if tupleKeyRaw, ok := changeMap["tuple_key"]; ok {
		if tupleKey, ok := tupleKeyRaw.(map[string]interface{}); ok {
			if u, ok := tupleKey["user"]; ok {
				user = fmt.Sprintf("%v", u)
			}
			if r, ok := tupleKey["relation"]; ok {
				relation = fmt.Sprintf("%v", r)
			}
			if o, ok := tupleKey["object"]; ok {
				object = fmt.Sprintf("%v", o)
			}
			// Extract condition if present
			if c, ok := tupleKey["condition"]; ok && c != nil {
				if conditionMap, ok := c.(map[string]interface{}); ok {
					// Convert condition to JSON string for storage
					if conditionBytes, err := json.Marshal(conditionMap); err == nil {
						condition = string(conditionBytes)
					}
				}
			}
		}
	}

	// If timestamp is zero, use current time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Parse user and object into type/ID components
	userType, userID := parseUserTypeAndID(user)
	objectType, objectID := parseObjectTypeAndID(object)

	// Create the change event with both new and legacy fields
	changeEvent := ChangeEvent{
		// New structured fields
		ObjectType: objectType,
		ObjectID:   objectID,
		Relation:   relation,
		UserType:   userType,
		UserID:     userID,
		ChangeType: determineChangeType(operation),
		Timestamp:  timestamp,
		Condition:  condition,
		RawJSON:    string(rawJSON),

		// Legacy fields for backward compatibility
		TupleKey: TupleKey{
			User:       user,
			UserType:   userType,
			UserID:     userID,
			Relation:   relation,
			Object:     object,
			ObjectType: objectType,
			ObjectID:   objectID,
		},
		Operation: operation,
	}

	return changeEvent, nil
}

// determineChangeType maps OpenFGA operations to change types
func determineChangeType(operation string) string {
	switch strings.ToUpper(operation) {
	case "TUPLE_TO_USERSET_WRITE", "WRITE":
		return "tuple_write"
	case "TUPLE_TO_USERSET_DELETE", "DELETE":
		return "tuple_delete"
	default:
		return "tuple_change"
	}
}

// parseTupleKey parses a tuple key and splits user and object into type and ID components (legacy method)
func (f *OpenFGAFetcher) parseTupleKey(user, relation, object string) TupleKey {
	userType, userID := parseUserTypeAndID(user)
	objectType, objectID := parseObjectTypeAndID(object)

	return TupleKey{
		User:       user,
		UserType:   userType,
		UserID:     userID,
		Relation:   relation,
		Object:     object,
		ObjectType: objectType,
		ObjectID:   objectID,
	}
}

// parseUserTypeAndID parses a user string into type and ID
// Expected formats:
// - "user_type:user_id" -> type="user_type", id="user_id"
// - "user_id" -> type="user", id="user_id"
// - "type:namespace:id" -> type="type", id="namespace:id"
func parseUserTypeAndID(user string) (string, string) {
	if user == "" {
		return "user", ""
	}

	// Handle special cases like user sets: "group:engineering#member"
	if strings.Contains(user, "#") {
		parts := strings.SplitN(user, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "user", user
	}

	// Standard format: "type:id"
	if strings.Contains(user, ":") {
		parts := strings.SplitN(user, ":", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
	}

	// If no type prefix, assume it's just an ID
	return "user", user
}

// parseObjectTypeAndID parses an object string into type and ID
// Expected formats:
// - "object_type:object_id" -> type="object_type", id="object_id"
// - "object_id" -> type="object", id="object_id"
func parseObjectTypeAndID(object string) (string, string) {
	if object == "" {
		return "object", ""
	}

	// Standard format: "type:id"
	if strings.Contains(object, ":") {
		parts := strings.SplitN(object, ":", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
	}

	// If no type prefix, assume it's just an ID
	return "object", object
}

// GetChangesSince fetches all changes since a given timestamp
func (f *OpenFGAFetcher) GetChangesSince(ctx context.Context, since time.Time, maxChanges int) (*FetchResult, error) {
	f.logger.WithFields(logrus.Fields{
		"since":       since,
		"max_changes": maxChanges,
	}).Info("Fetching changes since timestamp")

	// Start from the beginning and filter by timestamp
	result, err := f.FetchAllChanges(ctx, "", maxChanges)
	if err != nil {
		return nil, err
	}

	// Filter changes by timestamp
	var filteredChanges []ChangeEvent
	for _, change := range result.Changes {
		if change.Timestamp.After(since) || change.Timestamp.Equal(since) {
			filteredChanges = append(filteredChanges, change)
		}
	}

	result.Changes = filteredChanges
	result.TotalFetched = len(filteredChanges)

	f.logger.WithFields(logrus.Fields{
		"total_changes":    len(result.Changes),
		"filtered_changes": len(filteredChanges),
		"since":            since,
	}).Info("Filtered changes by timestamp")

	return result, nil
}

// ValidateChangeEvent validates that a change event has all required fields
func (f *OpenFGAFetcher) ValidateChangeEvent(change ChangeEvent) error {
	var errors []string

	if change.ObjectType == "" {
		errors = append(errors, "object_type is required")
	}
	if change.ObjectID == "" {
		errors = append(errors, "object_id is required")
	}
	if change.Relation == "" {
		errors = append(errors, "relation is required")
	}
	if change.UserType == "" {
		errors = append(errors, "user_type is required")
	}
	if change.UserID == "" {
		errors = append(errors, "user_id is required")
	}
	if change.ChangeType == "" {
		errors = append(errors, "change_type is required")
	}
	if change.Timestamp.IsZero() {
		errors = append(errors, "timestamp is required")
	}

	if len(errors) > 0 {
		return fmt.Errorf("change event validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

// updateStats updates internal statistics
func (f *OpenFGAFetcher) updateStats(success bool, changesCount int, latency time.Duration) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	atomic.AddInt64(&f.stats.TotalRequests, 1)
	if success {
		atomic.AddInt64(&f.stats.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&f.stats.FailedRequests, 1)
	}

	atomic.AddInt64(&f.stats.TotalChanges, int64(changesCount))
	f.stats.LastFetchTime = time.Now()

	// Update average latency (simple moving average)
	totalRequests := atomic.LoadInt64(&f.stats.TotalRequests)
	if totalRequests > 0 {
		f.stats.AverageLatency = (f.stats.AverageLatency*float64(totalRequests-1) + float64(latency.Milliseconds())) / float64(totalRequests)
	}
}

// retryWithBackoff executes a function with exponential backoff retry logic
func (f *OpenFGAFetcher) retryWithBackoff(ctx context.Context, operation func() error) error {
	config := f.options.RetryConfig
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check context before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		err := operation()
		if err == nil {
			return nil
		}

		f.logger.WithFields(logrus.Fields{
			"attempt":     attempt + 1,
			"max_retries": config.MaxRetries,
			"delay":       delay,
			"error":       err.Error(),
		}).Warn("Operation failed, retrying")

		// Don't retry on the last attempt
		if attempt == config.MaxRetries {
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries", config.MaxRetries)
}

// FetchChangesWithRetry fetches changes with retry logic and enhanced error handling
func (f *OpenFGAFetcher) FetchChangesWithRetry(ctx context.Context, continuationToken string, pageSize int32) (*FetchResult, error) {
	startTime := time.Now()
	var result *FetchResult

	// Apply rate limiting
	if f.rateLimiter != nil {
		select {
		case <-f.rateLimiter.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Execute with retry logic
	err := f.retryWithBackoff(ctx, func() error {
		var err error
		result, err = f.FetchChangesWithPaging(ctx, continuationToken, pageSize)
		return err
	})

	latency := time.Since(startTime)
	changesCount := 0
	if result != nil {
		changesCount = len(result.Changes)
	}

	// Update statistics
	f.updateStats(err == nil, changesCount, latency)

	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"continuation_token": continuationToken,
			"page_size":          pageSize,
			"latency_ms":         latency.Milliseconds(),
			"error":              err.Error(),
		}).Error("Failed to fetch changes after retries")
		return nil, err
	}

	// Validate changes if enabled
	if f.options.EnableValidation && result != nil {
		for i, change := range result.Changes {
			if validationErr := f.ValidateChangeEvent(change); validationErr != nil {
				f.logger.WithFields(logrus.Fields{
					"change_index":     i,
					"validation_error": validationErr.Error(),
				}).Warn("Change event validation failed")
			}
		}
	}

	f.logger.WithFields(logrus.Fields{
		"changes_count": changesCount,
		"latency_ms":    latency.Milliseconds(),
		"next_token":    result.ContinuationToken,
		"has_more":      result.HasMore,
	}).Debug("Successfully fetched changes with retry")

	return result, nil
}

// FetchAllChangesWithOptions fetches all changes with advanced options
func (f *OpenFGAFetcher) FetchAllChangesWithOptions(ctx context.Context, startToken string, options FetchOptions) (*FetchResult, error) {
	f.logger.WithFields(logrus.Fields{
		"start_token":      startToken,
		"max_changes":      options.MaxChanges,
		"page_size":        options.PageSize,
		"concurrent_pages": options.ConcurrentPages,
	}).Info("Starting to fetch all changes with advanced options")

	// Create context with timeout if specified
	var ctxWithTimeout context.Context
	var cancel context.CancelFunc
	if options.Timeout > 0 {
		ctxWithTimeout, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	} else {
		ctxWithTimeout = ctx
	}

	var allChanges []ChangeEvent
	currentToken := startToken
	totalFetched := 0

	for {
		// Check context cancellation
		select {
		case <-ctxWithTimeout.Done():
			return nil, ctxWithTimeout.Err()
		default:
		}

		// Check if we've reached the maximum changes limit
		if options.MaxChanges > 0 && totalFetched >= options.MaxChanges {
			f.logger.WithField("total_fetched", totalFetched).Info("Reached maximum changes limit")
			break
		}

		// Fetch the next batch with retry logic
		result, err := f.FetchChangesWithRetry(ctxWithTimeout, currentToken, options.PageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch changes batch: %w", err)
		}

		// Add changes to our collection
		allChanges = append(allChanges, result.Changes...)
		totalFetched += len(result.Changes)

		// Check if we have more changes
		if !result.HasMore || result.ContinuationToken == "" {
			f.logger.WithField("total_fetched", totalFetched).Info("No more changes available")
			break
		}

		// Update token for next iteration
		currentToken = result.ContinuationToken

		f.logger.WithFields(logrus.Fields{
			"batch_size":    len(result.Changes),
			"total_fetched": totalFetched,
			"next_token":    currentToken,
		}).Debug("Processed batch, continuing pagination")
	}

	return &FetchResult{
		Changes:           allChanges,
		ContinuationToken: currentToken,
		HasMore:           false, // We've fetched all available
		TotalFetched:      totalFetched,
	}, nil
}

// GetChangesSinceWithOptions fetches changes since a timestamp with advanced options
func (f *OpenFGAFetcher) GetChangesSinceWithOptions(ctx context.Context, since time.Time, options FetchOptions) (*FetchResult, error) {
	f.logger.WithFields(logrus.Fields{
		"since":       since,
		"max_changes": options.MaxChanges,
	}).Info("Fetching changes since timestamp with options")

	// Start from the beginning and filter by timestamp
	result, err := f.FetchAllChangesWithOptions(ctx, "", options)
	if err != nil {
		return nil, err
	}

	// Filter changes by timestamp
	var filteredChanges []ChangeEvent
	for _, change := range result.Changes {
		if change.Timestamp.After(since) || change.Timestamp.Equal(since) {
			filteredChanges = append(filteredChanges, change)
		}
	}

	result.Changes = filteredChanges
	result.TotalFetched = len(filteredChanges)

	f.logger.WithFields(logrus.Fields{
		"total_changes":    len(result.Changes),
		"filtered_changes": len(filteredChanges),
		"since":            since,
	}).Info("Filtered changes by timestamp")

	return result, nil
}
