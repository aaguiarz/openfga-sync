package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestParseUserTypeAndID(t *testing.T) {
	tests := []struct {
		input      string
		expectType string
		expectID   string
	}{
		{"user:alice", "user", "alice"},
		{"employee:alice", "employee", "alice"},
		{"group:engineering#member", "group", "engineering#member"},
		{"alice", "user", "alice"},
		{"", "user", ""},
		{"namespace:type:id", "namespace", "type:id"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			gotType, gotID := parseUserTypeAndID(test.input)
			if gotType != test.expectType {
				t.Errorf("Expected type %q, got %q", test.expectType, gotType)
			}
			if gotID != test.expectID {
				t.Errorf("Expected ID %q, got %q", test.expectID, gotID)
			}
		})
	}
}

func TestParseObjectTypeAndID(t *testing.T) {
	tests := []struct {
		input      string
		expectType string
		expectID   string
	}{
		{"document:readme.md", "document", "readme.md"},
		{"folder:src", "folder", "src"},
		{"readme.md", "object", "readme.md"},
		{"", "object", ""},
		{"namespace:object:id", "namespace", "object:id"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			gotType, gotID := parseObjectTypeAndID(test.input)
			if gotType != test.expectType {
				t.Errorf("Expected type %q, got %q", test.expectType, gotType)
			}
			if gotID != test.expectID {
				t.Errorf("Expected ID %q, got %q", test.expectID, gotID)
			}
		})
	}
}

func TestDetermineChangeType(t *testing.T) {
	tests := []struct {
		operation string
		expected  string
	}{
		{"WRITE", "tuple_write"},
		{"TUPLE_TO_USERSET_WRITE", "tuple_write"},
		{"DELETE", "tuple_delete"},
		{"TUPLE_TO_USERSET_DELETE", "tuple_delete"},
		{"UNKNOWN", "tuple_change"},
		{"", "tuple_change"},
	}

	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			got := determineChangeType(test.operation)
			if got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}

func TestParseChangeEvent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during testing

	fetcher := &OpenFGAFetcher{
		logger: logger,
	}

	// Mock change event data
	mockChange := map[string]interface{}{
		"operation": "WRITE",
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"tuple_key": map[string]interface{}{
			"user":     "employee:alice",
			"relation": "viewer",
			"object":   "document:readme.md",
		},
	}

	changeEvent, err := fetcher.parseChangeEvent(mockChange)
	if err != nil {
		t.Fatalf("Failed to parse change event: %v", err)
	}

	// Test parsed fields
	if changeEvent.UserType != "employee" {
		t.Errorf("Expected user_type 'employee', got %q", changeEvent.UserType)
	}
	if changeEvent.UserID != "alice" {
		t.Errorf("Expected user_id 'alice', got %q", changeEvent.UserID)
	}
	if changeEvent.ObjectType != "document" {
		t.Errorf("Expected object_type 'document', got %q", changeEvent.ObjectType)
	}
	if changeEvent.ObjectID != "readme.md" {
		t.Errorf("Expected object_id 'readme.md', got %q", changeEvent.ObjectID)
	}
	if changeEvent.Relation != "viewer" {
		t.Errorf("Expected relation 'viewer', got %q", changeEvent.Relation)
	}
	if changeEvent.ChangeType != "tuple_write" {
		t.Errorf("Expected change_type 'tuple_write', got %q", changeEvent.ChangeType)
	}

	// Test that raw JSON is present
	if changeEvent.RawJSON == "" {
		t.Error("Expected raw JSON to be present")
	}

	// Test that raw JSON is valid
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(changeEvent.RawJSON), &rawData); err != nil {
		t.Errorf("Raw JSON is not valid: %v", err)
	}

	// Test legacy compatibility fields
	if changeEvent.TupleKey.User != "employee:alice" {
		t.Errorf("Expected legacy user 'employee:alice', got %q", changeEvent.TupleKey.User)
	}
	if changeEvent.Operation != "WRITE" {
		t.Errorf("Expected legacy operation 'WRITE', got %q", changeEvent.Operation)
	}
}

func TestValidateChangeEvent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	fetcher := &OpenFGAFetcher{
		logger: logger,
	}

	// Valid change event
	validChange := ChangeEvent{
		ObjectType: "document",
		ObjectID:   "readme.md",
		Relation:   "viewer",
		UserType:   "employee",
		UserID:     "alice",
		ChangeType: "tuple_write",
		Timestamp:  time.Now(),
	}

	if err := fetcher.ValidateChangeEvent(validChange); err != nil {
		t.Errorf("Valid change event should not have validation errors: %v", err)
	}

	// Invalid change event - missing required fields
	invalidChange := ChangeEvent{
		ObjectType: "document",
		// Missing other required fields
	}

	if err := fetcher.ValidateChangeEvent(invalidChange); err == nil {
		t.Error("Invalid change event should have validation errors")
	}
}

// MockOpenFGAFetcher for integration-style testing
type MockOpenFGAFetcher struct {
	*OpenFGAFetcher
	mockChanges []map[string]interface{}
	currentPage int
}

func NewMockOpenFGAFetcher(logger *logrus.Logger) *MockOpenFGAFetcher {
	return &MockOpenFGAFetcher{
		OpenFGAFetcher: &OpenFGAFetcher{
			logger: logger,
		},
		mockChanges: []map[string]interface{}{
			{
				"operation": "WRITE",
				"timestamp": time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
				"tuple_key": map[string]interface{}{
					"user":     "employee:alice",
					"relation": "viewer",
					"object":   "document:readme.md",
				},
			},
			{
				"operation": "DELETE",
				"timestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano),
				"tuple_key": map[string]interface{}{
					"user":     "employee:bob",
					"relation": "editor",
					"object":   "document:spec.md",
				},
			},
			{
				"operation": "WRITE",
				"timestamp": time.Now().Format(time.RFC3339Nano),
				"tuple_key": map[string]interface{}{
					"user":     "group:engineering#member",
					"relation": "viewer",
					"object":   "folder:src",
				},
			},
		},
	}
}

func (m *MockOpenFGAFetcher) FetchChangesWithPaging(ctx context.Context, continuationToken string, pageSize int32) (*FetchResult, error) {
	// Simple mock implementation for testing
	var changes []ChangeEvent

	for _, mockChange := range m.mockChanges {
		changeEvent, err := m.parseChangeEvent(mockChange)
		if err != nil {
			continue
		}
		changes = append(changes, changeEvent)
	}

	return &FetchResult{
		Changes:           changes,
		ContinuationToken: "",
		HasMore:           false,
		TotalFetched:      len(changes),
	}, nil
}

func TestChangeEventStructure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockFetcher := NewMockOpenFGAFetcher(logger)

	result, err := mockFetcher.FetchChangesWithPaging(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("Failed to fetch changes: %v", err)
	}

	if len(result.Changes) == 0 {
		t.Fatal("Expected some changes to be returned")
	}

	// Test that all changes have the required structure
	for i, change := range result.Changes {
		t.Run(fmt.Sprintf("change_%d", i), func(t *testing.T) {
			// Check that all required fields are populated
			if change.ObjectType == "" {
				t.Error("ObjectType should not be empty")
			}
			if change.ObjectID == "" {
				t.Error("ObjectID should not be empty")
			}
			if change.Relation == "" {
				t.Error("Relation should not be empty")
			}
			if change.UserType == "" {
				t.Error("UserType should not be empty")
			}
			if change.UserID == "" {
				t.Error("UserID should not be empty")
			}
			if change.ChangeType == "" {
				t.Error("ChangeType should not be empty")
			}
			if change.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
			if change.RawJSON == "" {
				t.Error("RawJSON should not be empty")
			}

			// Validate that raw JSON can be parsed
			var rawData map[string]interface{}
			if err := json.Unmarshal([]byte(change.RawJSON), &rawData); err != nil {
				t.Errorf("RawJSON should be valid JSON: %v", err)
			}

			// Test validation
			if err := mockFetcher.ValidateChangeEvent(change); err != nil {
				t.Errorf("Change event should be valid: %v", err)
			}
		})
	}
}

func TestRetryConfig(t *testing.T) {
	defaultConfig := DefaultRetryConfig()

	if defaultConfig.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", defaultConfig.MaxRetries)
	}
	if defaultConfig.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay 100ms, got %v", defaultConfig.InitialDelay)
	}
	if defaultConfig.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay 5s, got %v", defaultConfig.MaxDelay)
	}
	if defaultConfig.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor 2.0, got %f", defaultConfig.BackoffFactor)
	}
}

func TestFetchOptions(t *testing.T) {
	defaultOptions := DefaultFetchOptions()

	if defaultOptions.PageSize != 100 {
		t.Errorf("Expected PageSize 100, got %d", defaultOptions.PageSize)
	}
	if defaultOptions.MaxChanges != 0 {
		t.Errorf("Expected MaxChanges 0, got %d", defaultOptions.MaxChanges)
	}
	if defaultOptions.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", defaultOptions.Timeout)
	}
	if !defaultOptions.EnableValidation {
		t.Error("Expected EnableValidation to be true")
	}
}

func TestNewOpenFGAFetcherWithOptions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	customOptions := FetchOptions{
		PageSize:         50,
		MaxChanges:       1000,
		Timeout:          60 * time.Second,
		RateLimitDelay:   100 * time.Millisecond,
		EnableValidation: false,
	}

	// This would normally require a real API URL and token, so we'll test the error case
	_, err := NewOpenFGAFetcherWithOptions("http://invalid-url", "invalid-store", "", logger, customOptions)

	// We expect an error because the URL is invalid, but the function should process the options
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}
}

func TestFetcherStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	fetcher := &OpenFGAFetcher{
		logger: logger,
		stats:  FetcherStats{},
	}

	// Test initial stats
	stats := fetcher.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests 0, got %d", stats.TotalRequests)
	}
	if stats.SuccessRequests != 0 {
		t.Errorf("Expected SuccessRequests 0, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 0 {
		t.Errorf("Expected FailedRequests 0, got %d", stats.FailedRequests)
	}

	// Test stats update
	fetcher.updateStats(true, 5, 100*time.Millisecond)

	stats = fetcher.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("Expected TotalRequests 1, got %d", stats.TotalRequests)
	}
	if stats.SuccessRequests != 1 {
		t.Errorf("Expected SuccessRequests 1, got %d", stats.SuccessRequests)
	}
	if stats.TotalChanges != 5 {
		t.Errorf("Expected TotalChanges 5, got %d", stats.TotalChanges)
	}
	if stats.AverageLatency != 100.0 {
		t.Errorf("Expected AverageLatency 100.0, got %f", stats.AverageLatency)
	}
}

func TestAdvancedUserParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectType string
		expectID   string
	}{
		{
			name:       "Standard user",
			input:      "user:alice",
			expectType: "user",
			expectID:   "alice",
		},
		{
			name:       "Employee user",
			input:      "employee:bob",
			expectType: "employee",
			expectID:   "bob",
		},
		{
			name:       "Group with member",
			input:      "group:engineering#member",
			expectType: "group",
			expectID:   "engineering#member",
		},
		{
			name:       "Service account",
			input:      "service_account:ci-bot",
			expectType: "service_account",
			expectID:   "ci-bot",
		},
		{
			name:       "Nested namespace",
			input:      "org:acme:department:engineering",
			expectType: "org",
			expectID:   "acme:department:engineering",
		},
		{
			name:       "Email format",
			input:      "user:alice@example.com",
			expectType: "user",
			expectID:   "alice@example.com",
		},
		{
			name:       "Empty string",
			input:      "",
			expectType: "user",
			expectID:   "",
		},
		{
			name:       "Just ID",
			input:      "alice",
			expectType: "user",
			expectID:   "alice",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotID := parseUserTypeAndID(test.input)
			if gotType != test.expectType {
				t.Errorf("Expected type %q, got %q", test.expectType, gotType)
			}
			if gotID != test.expectID {
				t.Errorf("Expected ID %q, got %q", test.expectID, gotID)
			}
		})
	}
}

func TestAdvancedObjectParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectType string
		expectID   string
	}{
		{
			name:       "Document",
			input:      "document:readme.md",
			expectType: "document",
			expectID:   "readme.md",
		},
		{
			name:       "Folder with path",
			input:      "folder:src/main/java",
			expectType: "folder",
			expectID:   "src/main/java",
		},
		{
			name:       "Repository",
			input:      "repository:github.com/org/repo",
			expectType: "repository",
			expectID:   "github.com/org/repo",
		},
		{
			name:       "Database table",
			input:      "table:users",
			expectType: "table",
			expectID:   "users",
		},
		{
			name:       "Nested resource",
			input:      "resource:project:api:endpoint",
			expectType: "resource",
			expectID:   "project:api:endpoint",
		},
		{
			name:       "UUID",
			input:      "entity:550e8400-e29b-41d4-a716-446655440000",
			expectType: "entity",
			expectID:   "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:       "Empty string",
			input:      "",
			expectType: "object",
			expectID:   "",
		},
		{
			name:       "Just ID",
			input:      "readme.md",
			expectType: "object",
			expectID:   "readme.md",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotID := parseObjectTypeAndID(test.input)
			if gotType != test.expectType {
				t.Errorf("Expected type %q, got %q", test.expectType, gotType)
			}
			if gotID != test.expectID {
				t.Errorf("Expected ID %q, got %q", test.expectID, gotID)
			}
		})
	}
}

func TestMockFetcherWithPaging(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockFetcher := NewMockOpenFGAFetcher(logger)

	// Test basic fetch
	result, err := mockFetcher.FetchChangesWithPaging(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("Failed to fetch changes: %v", err)
	}

	if len(result.Changes) == 0 {
		t.Error("Expected some changes to be returned")
	}

	// Verify that each change has proper structure
	for i, change := range result.Changes {
		t.Run(fmt.Sprintf("change_%d_structure", i), func(t *testing.T) {
			if change.ObjectType == "" {
				t.Error("ObjectType should not be empty")
			}
			if change.ObjectID == "" {
				t.Error("ObjectID should not be empty")
			}
			if change.UserType == "" {
				t.Error("UserType should not be empty")
			}
			if change.UserID == "" {
				t.Error("UserID should not be empty")
			}
			if change.Relation == "" {
				t.Error("Relation should not be empty")
			}
			if change.ChangeType == "" {
				t.Error("ChangeType should not be empty")
			}
			if change.RawJSON == "" {
				t.Error("RawJSON should not be empty")
			}
			if change.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

func TestValidationEdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	fetcher := &OpenFGAFetcher{
		logger: logger,
	}

	tests := []struct {
		name        string
		change      ChangeEvent
		expectValid bool
	}{
		{
			name: "Valid complete change",
			change: ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme.md",
				Relation:   "viewer",
				UserType:   "employee",
				UserID:     "alice",
				ChangeType: "tuple_write",
				Timestamp:  time.Now(),
			},
			expectValid: true,
		},
		{
			name: "Missing object type",
			change: ChangeEvent{
				ObjectID:   "readme.md",
				Relation:   "viewer",
				UserType:   "employee",
				UserID:     "alice",
				ChangeType: "tuple_write",
				Timestamp:  time.Now(),
			},
			expectValid: false,
		},
		{
			name: "Missing user info",
			change: ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme.md",
				Relation:   "viewer",
				ChangeType: "tuple_write",
				Timestamp:  time.Now(),
			},
			expectValid: false,
		},
		{
			name: "Zero timestamp",
			change: ChangeEvent{
				ObjectType: "document",
				ObjectID:   "readme.md",
				Relation:   "viewer",
				UserType:   "employee",
				UserID:     "alice",
				ChangeType: "tuple_write",
			},
			expectValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := fetcher.ValidateChangeEvent(test.change)
			if test.expectValid && err != nil {
				t.Errorf("Expected valid change, got error: %v", err)
			}
			if !test.expectValid && err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

// BenchmarkParseUserTypeAndID benchmarks the user parsing function
func BenchmarkParseUserTypeAndID(b *testing.B) {
	testCases := []string{
		"user:alice",
		"employee:bob",
		"group:engineering#member",
		"service_account:ci-bot",
		"alice",
		"",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			parseUserTypeAndID(testCase)
		}
	}
}

// BenchmarkParseObjectTypeAndID benchmarks the object parsing function
func BenchmarkParseObjectTypeAndID(b *testing.B) {
	testCases := []string{
		"document:readme.md",
		"folder:src/main/java",
		"repository:github.com/org/repo",
		"readme.md",
		"",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			parseObjectTypeAndID(testCase)
		}
	}
}
