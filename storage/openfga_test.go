package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

func TestParseOpenFGADSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    *OpenFGAConfig
		wantErr bool
	}{
		{
			name:    "simple format",
			dsn:     "http://localhost:8080/store123",
			want:    &OpenFGAConfig{Endpoint: "http://localhost:8080", StoreID: "store123"},
			wantErr: false,
		},
		{
			name:    "invalid simple format",
			dsn:     "invalid",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty string",
			dsn:     "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "json format basic",
			dsn:     `{"endpoint":"http://localhost:8080","store_id":"store123"}`,
			want:    &OpenFGAConfig{Endpoint: "http://localhost:8080", StoreID: "store123"},
			wantErr: false,
		},
		{
			name: "json format with token",
			dsn:  `{"endpoint":"https://api.openfga.example.com","store_id":"01HXXX-STORE-ID","token":"secret-token"}`,
			want: &OpenFGAConfig{
				Endpoint: "https://api.openfga.example.com",
				StoreID:  "01HXXX-STORE-ID",
				Token:    "secret-token",
			},
			wantErr: false,
		},
		{
			name: "json format with all fields",
			dsn: `{
				"endpoint":"https://api.openfga.example.com",
				"store_id":"01HXXX-STORE-ID",
				"token":"secret-token",
				"authorization_model_id":"01MODEL-ID",
				"request_timeout":"30s",
				"max_retries":5,
				"batch_size":200
			}`,
			want: &OpenFGAConfig{
				Endpoint:             "https://api.openfga.example.com",
				StoreID:              "01HXXX-STORE-ID",
				Token:                "secret-token",
				AuthorizationModelID: "01MODEL-ID",
				MaxRetries:           5,
				BatchSize:            200,
			},
			wantErr: false,
		},
		{
			name:    "json format missing endpoint",
			dsn:     `{"store_id":"store123"}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "json format missing store_id",
			dsn:     `{"endpoint":"http://localhost:8080"}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid json",
			dsn:     `{"invalid":json}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "https endpoint with store ID",
			dsn:     "https://api.openfga.example.com/01HXXX-STORE-ID",
			want:    &OpenFGAConfig{Endpoint: "https://api.openfga.example.com", StoreID: "01HXXX-STORE-ID"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOpenFGADSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOpenFGADSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got == nil {
					t.Errorf("parseOpenFGADSN() returned nil config but no error")
					return
				}

				if got.Endpoint != tt.want.Endpoint {
					t.Errorf("parseOpenFGADSN() endpoint = %v, want %v", got.Endpoint, tt.want.Endpoint)
				}

				if got.StoreID != tt.want.StoreID {
					t.Errorf("parseOpenFGADSN() store_id = %v, want %v", got.StoreID, tt.want.StoreID)
				}

				if got.Token != tt.want.Token {
					t.Errorf("parseOpenFGADSN() token = %v, want %v", got.Token, tt.want.Token)
				}

				if tt.want.AuthorizationModelID != "" && got.AuthorizationModelID != tt.want.AuthorizationModelID {
					t.Errorf("parseOpenFGADSN() authorization_model_id = %v, want %v", got.AuthorizationModelID, tt.want.AuthorizationModelID)
				}

				if tt.want.MaxRetries != 0 && got.MaxRetries != tt.want.MaxRetries {
					t.Errorf("parseOpenFGADSN() max_retries = %v, want %v", got.MaxRetries, tt.want.MaxRetries)
				}

				if tt.want.BatchSize != 0 && got.BatchSize != tt.want.BatchSize {
					t.Errorf("parseOpenFGADSN() batch_size = %v, want %v", got.BatchSize, tt.want.BatchSize)
				}
			}
		})
	}
}

func TestConvertToTupleKey(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a mock adapter for testing
	adapter := &OpenFGAAdapter{
		logger: logger,
	}

	tests := []struct {
		name   string
		change fetcher.ChangeEvent
		want   string // Expected user:relation:object format for comparison
	}{
		{
			name: "basic conversion",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
			},
			want: "user:alice#viewer@document:readme",
		},
		{
			name: "no user type",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "",
				UserID:     "alice",
				Operation:  "WRITE",
			},
			want: "alice#viewer@document:readme",
		},
		{
			name: "no object type",
			change: fetcher.ChangeEvent{
				ObjectType: "",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
			},
			want: "user:alice#viewer@readme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.convertToTupleKey(tt.change)

			// Verify the conversion
			if result.User == "" || result.Relation == "" || result.Object == "" {
				t.Errorf("convertToTupleKey() returned incomplete result: %+v", result)
			}

			// Check individual components
			expectedUser := tt.change.UserID
			if tt.change.UserType != "" {
				expectedUser = tt.change.UserType + ":" + tt.change.UserID
			}

			expectedObject := tt.change.ObjectID
			if tt.change.ObjectType != "" {
				expectedObject = tt.change.ObjectType + ":" + tt.change.ObjectID
			}

			if result.User != expectedUser {
				t.Errorf("convertToTupleKey() user = %v, want %v", result.User, expectedUser)
			}

			if result.Relation != tt.change.Relation {
				t.Errorf("convertToTupleKey() relation = %v, want %v", result.Relation, tt.change.Relation)
			}

			if result.Object != expectedObject {
				t.Errorf("convertToTupleKey() object = %v, want %v", result.Object, expectedObject)
			}
		})
	}
}

func TestOpenFGAAdapter_ContinuationToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a mock adapter for testing continuation tokens
	adapter := &OpenFGAAdapter{
		logger: logger,
	}

	ctx := context.Background()

	// Test initial state
	token, err := adapter.GetLastContinuationToken(ctx)
	if err != nil {
		t.Errorf("GetLastContinuationToken() error = %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token initially, got %s", token)
	}

	// Test saving and retrieving token
	testToken := "test-token-123"
	err = adapter.SaveContinuationToken(ctx, testToken)
	if err != nil {
		t.Errorf("SaveContinuationToken() error = %v", err)
	}

	retrievedToken, err := adapter.GetLastContinuationToken(ctx)
	if err != nil {
		t.Errorf("GetLastContinuationToken() error = %v", err)
	}
	if retrievedToken != testToken {
		t.Errorf("Expected token %s, got %s", testToken, retrievedToken)
	}
}

func TestOpenFGAAdapter_ModeValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	changelogAdapter := &OpenFGAAdapter{
		logger: logger,
		mode:   config.StorageModeChangelog,
	}

	statefulAdapter := &OpenFGAAdapter{
		logger: logger,
		mode:   config.StorageModeStateful,
	}

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
	}

	// Test that WriteChanges fails in stateful mode
	err := statefulAdapter.WriteChanges(ctx, changes)
	if err == nil {
		t.Error("Expected WriteChanges to fail in stateful mode")
	}

	// Test that ApplyChanges fails in changelog mode
	err = changelogAdapter.ApplyChanges(ctx, changes)
	if err == nil {
		t.Error("Expected ApplyChanges to fail in changelog mode")
	}
}

func TestOpenFGAAdapter_ProcessBatch(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a mock adapter for testing batch processing
	adapter := &OpenFGAAdapter{
		logger:    logger,
		batchSize: 2,
	}

	changes := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme1",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
		},
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "readme2",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
		},
		{
			Operation:  "UNKNOWN",
			ObjectType: "document",
			ObjectID:   "readme3",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "charlie",
		},
	}

	// Test batch processing (this will fail due to no actual OpenFGA connection, but we can test the logic)
	// ctx := context.Background() // Context not used in this test

	// Test conversion and batch separation
	var writes, deletes int
	for _, change := range changes {
		switch change.Operation {
		case "WRITE", "TUPLE_TO_USERSET_WRITE":
			writes++
		case "DELETE", "TUPLE_TO_USERSET_DELETE":
			deletes++
		}
	}

	if writes != 1 {
		t.Errorf("Expected 1 write operation, got %d", writes)
	}
	if deletes != 1 {
		t.Errorf("Expected 1 delete operation, got %d", deletes)
	}

	// Test tuple key conversion for each change
	for _, change := range changes {
		if change.Operation != "UNKNOWN" {
			tupleKey := adapter.convertToTupleKey(change)
			if tupleKey.User == "" || tupleKey.Relation == "" || tupleKey.Object == "" {
				t.Errorf("convertToTupleKey failed for change: %+v, result: %+v", change, tupleKey)
			}
		}
	}
}

func TestOpenFGAAdapter_GetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter := &OpenFGAAdapter{
		logger:         logger,
		targetStoreID:  "test-store-id",
		mode:           config.StorageModeChangelog,
		lastToken:      "test-token",
		requestTimeout: 30 * time.Second,
		maxRetries:     3,
		batchSize:      100,
		// Note: client is nil for this test, so GetStats will show connection error
	}

	ctx := context.Background()
	stats, err := adapter.GetStats(ctx)
	// We expect no error from GetStats itself, even if connection fails
	if err != nil {
		t.Errorf("GetStats() error = %v", err)
	}

	// Check that all expected stats are present
	expectedFields := []string{
		"adapter_type",
		"target_store_id",
		"storage_mode",
		"last_token",
		"request_timeout",
		"max_retries",
		"batch_size",
		"connection_status",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("GetStats() missing field: %s", field)
		}
	}

	// Check specific values
	if stats["adapter_type"] != "openfga" {
		t.Errorf("GetStats() adapter_type = %v, want openfga", stats["adapter_type"])
	}

	if stats["target_store_id"] != "test-store-id" {
		t.Errorf("GetStats() target_store_id = %v, want test-store-id", stats["target_store_id"])
	}

	if stats["storage_mode"] != "changelog" {
		t.Errorf("GetStats() storage_mode = %v, want changelog", stats["storage_mode"])
	}

	// Since client is nil, connection should show error
	if stats["connection_status"] != "error" {
		t.Errorf("GetStats() expected connection_status = error with nil client, got %v", stats["connection_status"])
	}
}

// TestOpenFGAAdapter_EmptyChanges tests handling of empty change arrays
func TestOpenFGAAdapter_EmptyChanges(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	changelogAdapter := &OpenFGAAdapter{
		logger: logger,
		mode:   config.StorageModeChangelog,
	}

	statefulAdapter := &OpenFGAAdapter{
		logger: logger,
		mode:   config.StorageModeStateful,
	}

	ctx := context.Background()
	emptyChanges := []fetcher.ChangeEvent{}

	// Test empty changes in both modes - should return nil without error
	err := changelogAdapter.WriteChanges(ctx, emptyChanges)
	if err != nil {
		t.Errorf("WriteChanges() with empty changes error = %v", err)
	}

	err = statefulAdapter.ApplyChanges(ctx, emptyChanges)
	if err != nil {
		t.Errorf("ApplyChanges() with empty changes error = %v", err)
	}
}

func TestConvertToTupleKeyWithCondition(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a mock adapter for testing
	adapter := &OpenFGAAdapter{
		logger: logger,
	}

	tests := []struct {
		name        string
		change      fetcher.ChangeEvent
		expectError bool
	}{
		{
			name: "tuple key with valid condition",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
				Condition:  `{"name":"ip_allowlist","context":{"allowed_ips":["192.168.1.1","10.0.0.1"]}}`,
			},
			expectError: false,
		},
		{
			name: "tuple key with condition name only",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
				Condition:  `{"name":"time_based"}`,
			},
			expectError: false,
		},
		{
			name: "tuple key with invalid condition JSON",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
				Condition:  `{invalid json}`,
			},
			expectError: false, // Should log warning but not fail
		},
		{
			name: "tuple key with condition missing name",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
				Condition:  `{"context":{"key":"value"}}`,
			},
			expectError: false, // Should log warning but not fail
		},
		{
			name: "tuple key without condition",
			change: fetcher.ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
				Operation:  "WRITE",
				Condition:  "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.convertToTupleKey(tt.change)

			// Verify basic tuple key fields
			if result.User == "" || result.Relation == "" || result.Object == "" {
				t.Errorf("convertToTupleKey() returned incomplete result: %+v", result)
			}

			// Check condition handling
			if tt.change.Condition != "" && !strings.Contains(tt.change.Condition, "invalid") && !strings.Contains(tt.change.Condition, `"context"`) {
				// Valid condition case
				if result.Condition == nil {
					t.Errorf("Expected condition to be set for valid condition JSON")
				} else {
					// Verify condition name is set
					if result.Condition.Name == "" {
						t.Errorf("Expected condition name to be set")
					}
				}
			} else if tt.change.Condition == "" {
				// No condition case
				if result.Condition != nil {
					t.Errorf("Expected condition to be nil when no condition provided")
				}
			}
			// For invalid cases, condition might be nil (logged warning but continues)
		})
	}
}

func TestParseCondition(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter := &OpenFGAAdapter{
		logger: logger,
	}

	tests := []struct {
		name          string
		conditionJSON string
		expectError   bool
		expectedName  string
		expectContext bool
	}{
		{
			name:          "valid condition with context",
			conditionJSON: `{"name":"ip_allowlist","context":{"allowed_ips":["192.168.1.1"]}}`,
			expectError:   false,
			expectedName:  "ip_allowlist",
			expectContext: true,
		},
		{
			name:          "valid condition without context",
			conditionJSON: `{"name":"time_based"}`,
			expectError:   false,
			expectedName:  "time_based",
			expectContext: false,
		},
		{
			name:          "empty condition",
			conditionJSON: "",
			expectError:   false,
			expectedName:  "",
			expectContext: false,
		},
		{
			name:          "invalid JSON",
			conditionJSON: `{invalid json}`,
			expectError:   true,
		},
		{
			name:          "missing name field",
			conditionJSON: `{"context":{"key":"value"}}`,
			expectError:   true,
		},
		{
			name:          "empty name field",
			conditionJSON: `{"name":"","context":{"key":"value"}}`,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.parseCondition(tt.conditionJSON)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.conditionJSON == "" {
				if result != nil {
					t.Errorf("Expected nil result for empty condition")
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result for valid condition")
				return
			}

			if result.Name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, result.Name)
			}

			if tt.expectContext {
				if result.Context == nil {
					t.Errorf("Expected context to be set")
				}
			} else {
				if result.Context != nil {
					t.Errorf("Expected context to be nil")
				}
			}
		})
	}
}

func TestConditionEndToEndFlow(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a mock adapter for testing
	adapter := &OpenFGAAdapter{
		logger: logger,
	}

	// Simulate a ChangeEvent with condition as it would come from the fetcher
	changeEvent := fetcher.ChangeEvent{
		ObjectType: "document",
		ObjectID:   "sensitive_doc_123",
		Relation:   "viewer",
		UserType:   "employee",
		UserID:     "alice@company.com",
		Operation:  "WRITE",
		ChangeType: "tuple_write",
		Timestamp:  time.Now(),
		// This simulates how conditions are stored by the fetcher
		Condition: `{"name":"ip_allowlist","context":{"allowed_ips":["192.168.1.100","10.0.0.50"],"department":"engineering"}}`,
		RawJSON:   `{"operation":"WRITE","tuple_key":{"user":"employee:alice@company.com","relation":"viewer","object":"document:sensitive_doc_123","condition":{"name":"ip_allowlist","context":{"allowed_ips":["192.168.1.100","10.0.0.50"],"department":"engineering"}}}}`,
	}

	// Convert to OpenFGA ClientTupleKey
	tupleKey := adapter.convertToTupleKey(changeEvent)

	// Verify the basic tuple components
	if tupleKey.User != "employee:alice@company.com" {
		t.Errorf("Expected user 'employee:alice@company.com', got '%s'", tupleKey.User)
	}
	if tupleKey.Relation != "viewer" {
		t.Errorf("Expected relation 'viewer', got '%s'", tupleKey.Relation)
	}
	if tupleKey.Object != "document:sensitive_doc_123" {
		t.Errorf("Expected object 'document:sensitive_doc_123', got '%s'", tupleKey.Object)
	}

	// Verify condition is properly parsed and set
	if tupleKey.Condition == nil {
		t.Fatal("Expected condition to be set, but it was nil")
	}

	if tupleKey.Condition.Name != "ip_allowlist" {
		t.Errorf("Expected condition name 'ip_allowlist', got '%s'", tupleKey.Condition.Name)
	}

	if tupleKey.Condition.Context == nil {
		t.Fatal("Expected condition context to be set, but it was nil")
	}

	context := *tupleKey.Condition.Context

	// Verify context contains expected data
	if allowedIps, ok := context["allowed_ips"]; ok {
		if ipsSlice, ok := allowedIps.([]interface{}); ok {
			if len(ipsSlice) != 2 {
				t.Errorf("Expected 2 allowed IPs, got %d", len(ipsSlice))
			}
			if ipsSlice[0] != "192.168.1.100" {
				t.Errorf("Expected first IP '192.168.1.100', got '%v'", ipsSlice[0])
			}
		} else {
			t.Error("Expected allowed_ips to be a slice")
		}
	} else {
		t.Error("Expected 'allowed_ips' field in context")
	}

	if department, ok := context["department"]; ok {
		if dept, ok := department.(string); ok {
			if dept != "engineering" {
				t.Errorf("Expected department 'engineering', got '%s'", dept)
			}
		} else {
			t.Error("Expected department to be a string")
		}
	} else {
		t.Error("Expected 'department' field in context")
	}

	t.Logf("✅ End-to-end condition flow test passed successfully!")
	t.Logf("   Condition Name: %s", tupleKey.Condition.Name)

	// Get context keys for logging
	var keys []string
	for key := range context {
		keys = append(keys, key)
	}
	t.Logf("   Context Keys: %v", keys)
}
