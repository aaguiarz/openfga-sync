package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

func main() {
	// Create a simple logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Demo OpenFGA adapter for replication/backup scenarios
	fmt.Println("🔄 OpenFGA Storage Adapter Demonstration")
	fmt.Println("========================================")
	fmt.Println("")
	fmt.Println("This adapter enables replication from one OpenFGA instance to another,")
	fmt.Println("useful for backup, migration, or multi-region scenarios.")
	fmt.Println("")

	// Note: This demo shows the adapter interface without requiring actual OpenFGA connections
	// In a real scenario, you would have two OpenFGA instances running

	// Test 1: DSN Parsing
	fmt.Println("📝 Testing DSN Parsing:")
	fmt.Println("-----------------------")

	testDSNs := []string{
		"http://localhost:8080/store123",
		"https://api.openfga.example.com/01HXXX-STORE-ID",
		`{"endpoint":"http://localhost:8080","store_id":"store123","token":"secret"}`,
		"invalid-dsn",
	}

	for _, dsn := range testDSNs {
		cfg, err := parseOpenFGADSNTest(dsn)
		if err != nil {
			fmt.Printf("❌ DSN '%s': %v\n", dsn, err)
		} else {
			fmt.Printf("✅ DSN '%s': endpoint=%s, store_id=%s\n", dsn, cfg.Endpoint, cfg.StoreID)
		}
	}

	// Test 2: Adapter Creation (Mock)
	fmt.Println("\n🔧 Testing Adapter Interface:")
	fmt.Println("-----------------------------")

	// Create mock adapters for both modes
	changelogAdapter := createMockAdapter(config.StorageModeChangelog, logger)
	statefulAdapter := createMockAdapter(config.StorageModeStateful, logger)

	// Test 3: Change Event Conversion
	fmt.Println("\n🔄 Testing Change Event Conversion:")
	fmt.Println("-----------------------------------")

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
		{
			Operation:  "DELETE", 
			ObjectType: "folder",
			ObjectID:   "src",
			Relation:   "editor",
			UserType:   "employee",
			UserID:     "bob",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "repo",
			ObjectID:   "openfga-sync",
			Relation:   "admin",
			UserType:   "team",
			UserID:     "engineering",
			Timestamp:  time.Now(),
		},
	}

	for i, change := range changes {
		tupleKey := changelogAdapter.convertToTupleKey(change)
		fmt.Printf("Change %d: %s %s#%s@%s -> %s#%s@%s\n", 
			i+1, change.Operation,
			change.UserType, change.UserID, change.ObjectType,
			tupleKey.User, tupleKey.Relation, tupleKey.Object,
		)
	}

	// Test 4: Mode Validation
	fmt.Println("\n🚦 Testing Mode Validation:")
	fmt.Println("---------------------------")

	ctx := context.Background()

	// Test changelog mode
	err := changelogAdapter.WriteChanges(ctx, changes)
	if err != nil {
		fmt.Printf("❌ Changelog WriteChanges failed (expected): %v\n", err)
	} else {
		fmt.Printf("✅ Changelog WriteChanges would succeed\n")
	}

	err = changelogAdapter.ApplyChanges(ctx, changes)
	if err != nil {
		fmt.Printf("✅ Changelog ApplyChanges correctly rejected: %v\n", err)
	}

	// Test stateful mode
	err = statefulAdapter.ApplyChanges(ctx, changes)
	if err != nil {
		fmt.Printf("❌ Stateful ApplyChanges failed (expected): %v\n", err)
	} else {
		fmt.Printf("✅ Stateful ApplyChanges would succeed\n")
	}

	err = statefulAdapter.WriteChanges(ctx, changes)
	if err != nil {
		fmt.Printf("✅ Stateful WriteChanges correctly rejected: %v\n", err)
	}

	// Test 5: Continuation Token Management
	fmt.Println("\n🔗 Testing Continuation Token Management:")
	fmt.Println("----------------------------------------")

	// Test token persistence (in-memory for OpenFGA adapter)
	testToken := "openfga-token-" + fmt.Sprintf("%d", time.Now().Unix())
	
	err = changelogAdapter.SaveContinuationToken(ctx, testToken)
	if err != nil {
		fmt.Printf("❌ Failed to save token: %v\n", err)
	} else {
		fmt.Printf("✅ Saved continuation token: %s\n", testToken)
	}

	retrievedToken, err := changelogAdapter.GetLastContinuationToken(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve token: %v\n", err)
	} else if retrievedToken == testToken {
		fmt.Printf("✅ Retrieved correct token: %s\n", retrievedToken)
	} else {
		fmt.Printf("❌ Retrieved wrong token: %s (expected %s)\n", retrievedToken, testToken)
	}

	// Test 6: Statistics and Monitoring
	fmt.Println("\n📊 Testing Statistics and Monitoring:")
	fmt.Println("------------------------------------")

	stats, err := changelogAdapter.GetStats(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to get stats: %v\n", err)
	} else {
		fmt.Printf("✅ Adapter Statistics:\n")
		for key, value := range stats {
			fmt.Printf("   %s: %v\n", key, value)
		}
	}

	// Test 7: Batch Processing Logic
	fmt.Println("\n📦 Testing Batch Processing Logic:")
	fmt.Println("----------------------------------")

	// Test batch separation
	var writes, deletes int
	for _, change := range changes {
		switch change.Operation {
		case "WRITE", "TUPLE_TO_USERSET_WRITE":
			writes++
		case "DELETE", "TUPLE_TO_USERSET_DELETE":
			deletes++
		}
	}

	fmt.Printf("✅ Processed %d changes: %d writes, %d deletes\n", len(changes), writes, deletes)
	fmt.Printf("✅ Batch size would be: %d (configurable)\n", changelogAdapter.batchSize)

	fmt.Println("\n🎉 OpenFGA Adapter Demo Complete!")
	fmt.Println("==================================")
	fmt.Println("")
	fmt.Println("The OpenFGA adapter supports:")
	fmt.Println("• Replication to another OpenFGA instance")
	fmt.Println("• Both changelog and stateful modes")
	fmt.Println("• Retry logic with exponential backoff")
	fmt.Println("• Batch processing for performance")
	fmt.Println("• Continuation token management (in-memory)")
	fmt.Println("• Statistics and health monitoring")
	fmt.Println("• Proper tuple key conversion")
	fmt.Println("")
	fmt.Println("Use cases:")
	fmt.Println("• Backup/disaster recovery")
	fmt.Println("• Multi-region replication") 
	fmt.Println("• Development/staging sync")
	fmt.Println("• Data migration between instances")
}

// Mock function to test DSN parsing without importing internal functions
func parseOpenFGADSNTest(dsn string) (*OpenFGAConfig, error) {
	// Simplified version for demo
	if dsn == "invalid-dsn" {
		return nil, fmt.Errorf("invalid DSN format")
	}

	// If DSN starts with {, treat it as JSON format (demo version)
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
	
	// Find the last occurrence of '/' to properly split endpoint and store_id
	lastSlashIndex := strings.LastIndex(dsn, "/")
	if lastSlashIndex == -1 || lastSlashIndex == len(dsn)-1 {
		return nil, fmt.Errorf("invalid DSN format, expected endpoint/store_id")
	}

	endpoint := dsn[:lastSlashIndex]
	storeID := dsn[lastSlashIndex+1:]

	return &OpenFGAConfig{
		Endpoint: endpoint,
		StoreID:  storeID,
	}, nil
}

// OpenFGAConfig represents the configuration for OpenFGA adapter (demo copy)
type OpenFGAConfig struct {
	Endpoint string `json:"endpoint"`
	StoreID  string `json:"store_id"`
}

// Mock adapter for demonstration
type MockOpenFGAAdapter struct {
	logger        *logrus.Logger
	mode          config.StorageMode
	lastToken     string
	batchSize     int
	targetStoreID string
}

func createMockAdapter(mode config.StorageMode, logger *logrus.Logger) *MockOpenFGAAdapter {
	return &MockOpenFGAAdapter{
		logger:        logger,
		mode:          mode,
		batchSize:     100,
		targetStoreID: "mock-target-store",
	}
}

func (m *MockOpenFGAAdapter) WriteChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if m.mode != config.StorageModeChangelog {
		return fmt.Errorf("WriteChanges is only supported in changelog mode")
	}
	// In real implementation, this would call applyChangesWithRetry
	return fmt.Errorf("mock adapter - no actual OpenFGA connection")
}

func (m *MockOpenFGAAdapter) ApplyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if m.mode != config.StorageModeStateful {
		return fmt.Errorf("ApplyChanges is only supported in stateful mode")
	}
	// In real implementation, this would call applyChangesWithRetry
	return fmt.Errorf("mock adapter - no actual OpenFGA connection")
}

func (m *MockOpenFGAAdapter) GetLastContinuationToken(ctx context.Context) (string, error) {
	return m.lastToken, nil
}

func (m *MockOpenFGAAdapter) SaveContinuationToken(ctx context.Context, token string) error {
	m.lastToken = token
	return nil
}

func (m *MockOpenFGAAdapter) Close() error {
	return nil
}

func (m *MockOpenFGAAdapter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"adapter_type":       "openfga",
		"target_store_id":    m.targetStoreID,
		"storage_mode":       string(m.mode),
		"last_token":         m.lastToken,
		"batch_size":         m.batchSize,
		"connection_status":  "error",
		"connection_error":   "mock adapter - no actual connection",
	}, nil
}

func (m *MockOpenFGAAdapter) convertToTupleKey(change fetcher.ChangeEvent) TupleKey {
	user := change.UserID
	if change.UserType != "" {
		user = change.UserType + ":" + change.UserID
	}

	object := change.ObjectID
	if change.ObjectType != "" {
		object = change.ObjectType + ":" + change.ObjectID
	}

	return TupleKey{
		User:     user,
		Relation: change.Relation,
		Object:   object,
	}
}

// TupleKey for demo
type TupleKey struct {
	User     string
	Relation string
	Object   string
}
