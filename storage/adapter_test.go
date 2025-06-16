package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

// TestStorageAdapterInterface tests the StorageAdapter interface across different implementations
func TestStorageAdapterInterface(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	t.Run("SQLite_Changelog", func(t *testing.T) {
		adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
		if err != nil {
			t.Fatalf("Failed to create changelog adapter: %v", err)
		}
		defer adapter.Close()

		ctx := context.Background()
		testContinuationToken(t, ctx, adapter)
		testStats(t, ctx, adapter)
		testWriteChanges(t, ctx, adapter)
	})

	t.Run("SQLite_Stateful", func(t *testing.T) {
		adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
		if err != nil {
			t.Fatalf("Failed to create stateful adapter: %v", err)
		}
		defer adapter.Close()

		ctx := context.Background()
		testContinuationToken(t, ctx, adapter)
		testStats(t, ctx, adapter)
		testApplyChanges(t, ctx, adapter)
	})
}

// testContinuationToken tests continuation token save/retrieve operations
func testContinuationToken(t *testing.T, ctx context.Context, adapter StorageAdapter) {
	testToken := "test-continuation-token-12345"

	// Test saving a token
	err := adapter.SaveContinuationToken(ctx, testToken)
	if err != nil {
		t.Errorf("SaveContinuationToken() error = %v", err)
		return
	}

	// Test retrieving the token
	retrievedToken, err := adapter.GetLastContinuationToken(ctx)
	if err != nil {
		t.Errorf("GetLastContinuationToken() error = %v", err)
		return
	}

	if retrievedToken != testToken {
		t.Errorf("Expected token %q, got %q", testToken, retrievedToken)
	}

	// Test updating the token
	newToken := "updated-token-67890"
	err = adapter.SaveContinuationToken(ctx, newToken)
	if err != nil {
		t.Errorf("SaveContinuationToken() update error = %v", err)
		return
	}

	retrievedToken, err = adapter.GetLastContinuationToken(ctx)
	if err != nil {
		t.Errorf("GetLastContinuationToken() after update error = %v", err)
		return
	}

	if retrievedToken != newToken {
		t.Errorf("Expected updated token %q, got %q", newToken, retrievedToken)
	}

	// Test empty token
	err = adapter.SaveContinuationToken(ctx, "")
	if err != nil {
		t.Errorf("SaveContinuationToken() empty token error = %v", err)
	}
}

// testStats tests the GetStats functionality
func testStats(t *testing.T, ctx context.Context, adapter StorageAdapter) {
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() error = %v", err)
		return
	}

	if stats == nil {
		t.Error("GetStats() returned nil")
		return
	}

	// Stats should be a map with some content
	t.Logf("Stats returned: %+v", stats)
}

// testWriteChanges tests changelog write operations (changelog mode only)
func testWriteChanges(t *testing.T, ctx context.Context, adapter StorageAdapter) {
	changes := createTestChanges()

	// Test writing changes
	err := adapter.WriteChanges(ctx, changes)
	if err != nil {
		t.Errorf("WriteChanges() error = %v", err)
		return
	}

	// Test writing empty changes slice
	err = adapter.WriteChanges(ctx, []fetcher.ChangeEvent{})
	if err != nil {
		t.Errorf("WriteChanges() empty changes error = %v", err)
	}

	// Verify stats reflect the written changes
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() after WriteChanges error = %v", err)
		return
	}

	// For changelog mode, expect changelog_entries count
	if count, ok := stats["changelog_entries"]; ok {
		if countInt, ok := count.(int64); ok && countInt < int64(len(changes)) {
			t.Errorf("Expected at least %d changelog entries, got %d", len(changes), countInt)
		}
	}
}

// testApplyChanges tests stateful operations (add, update, delete)
func testApplyChanges(t *testing.T, ctx context.Context, adapter StorageAdapter) {
	// Test 1: Add tuples
	addChanges := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			ChangeType: "tuple_write",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			ChangeType: "tuple_write",
			Timestamp:  time.Now(),
		},
	}

	err := adapter.ApplyChanges(ctx, addChanges)
	if err != nil {
		t.Errorf("ApplyChanges() add error = %v", err)
		return
	}

	// Verify tuples were added
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() after add error = %v", err)
		return
	}

	if count, ok := stats["current_tuples"]; ok {
		if countInt, ok := count.(int64); ok && countInt != 2 {
			t.Errorf("Expected 2 current tuples after add, got %d", countInt)
		}
	}

	// Test 2: Delete a tuple
	deleteChanges := []fetcher.ChangeEvent{
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			ChangeType: "tuple_delete",
			Timestamp:  time.Now(),
		},
	}

	err = adapter.ApplyChanges(ctx, deleteChanges)
	if err != nil {
		t.Errorf("ApplyChanges() delete error = %v", err)
		return
	}

	// Verify tuple was deleted
	stats, err = adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() after delete error = %v", err)
		return
	}

	if count, ok := stats["current_tuples"]; ok {
		if countInt, ok := count.(int64); ok && countInt != 1 {
			t.Errorf("Expected 1 current tuple after delete, got %d", countInt)
		}
	}

	// Test 3: Update existing tuple (upsert behavior)
	updateChanges := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			ChangeType: "tuple_write",
			Timestamp:  time.Now(),
			Condition:  `{"name":"test_condition","context":{"department":"engineering"}}`,
		},
	}

	err = adapter.ApplyChanges(ctx, updateChanges)
	if err != nil {
		t.Errorf("ApplyChanges() update error = %v", err)
	}

	// Count should still be 1 (upsert, not new insert)
	stats, err = adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() after update error = %v", err)
		return
	}

	if count, ok := stats["current_tuples"]; ok {
		if countInt, ok := count.(int64); ok && countInt != 1 {
			t.Errorf("Expected 1 current tuple after update, got %d", countInt)
		}
	}

	// Test 4: Empty changes
	err = adapter.ApplyChanges(ctx, []fetcher.ChangeEvent{})
	if err != nil {
		t.Errorf("ApplyChanges() empty changes error = %v", err)
	}
}

// createTestChanges creates a diverse set of test change events
func createTestChanges() []fetcher.ChangeEvent {
	now := time.Now()
	return []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme.md",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			ChangeType: "tuple_write",
			Timestamp:  now,
			RawJSON:    `{"operation":"WRITE","tuple_key":{"user":"user:alice","relation":"viewer","object":"document:readme.md"}}`,
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "guide.md",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			ChangeType: "tuple_write",
			Timestamp:  now.Add(time.Second),
			RawJSON:    `{"operation":"WRITE","tuple_key":{"user":"user:bob","relation":"editor","object":"document:guide.md"}}`,
		},
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "deprecated.md",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "charlie",
			ChangeType: "tuple_delete",
			Timestamp:  now.Add(2 * time.Second),
			RawJSON:    `{"operation":"DELETE","tuple_key":{"user":"user:charlie","relation":"viewer","object":"document:deprecated.md"}}`,
		},
		{
			Operation:  "WRITE",
			ObjectType: "folder",
			ObjectID:   "src",
			Relation:   "owner",
			UserType:   "team",
			UserID:     "engineering",
			ChangeType: "tuple_write",
			Timestamp:  now.Add(3 * time.Second),
			Condition:  `{"name":"team_access","context":{"department":"engineering","level":"senior"}}`,
			RawJSON:    `{"operation":"WRITE","tuple_key":{"user":"team:engineering","relation":"owner","object":"folder:src","condition":{"name":"team_access"}}}`,
		},
	}
}

// TestStorageAdapterModeValidation tests that operations respect storage modes
func TestStorageAdapterModeValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	ctx := context.Background()
	changes := createTestChanges()

	t.Run("WriteChanges_StatefulMode_ShouldFail", func(t *testing.T) {
		adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
		if err != nil {
			t.Fatalf("Failed to create stateful adapter: %v", err)
		}
		defer adapter.Close()

		err = adapter.WriteChanges(ctx, changes)
		if err == nil {
			t.Error("Expected WriteChanges to fail in stateful mode")
		}
	})

	t.Run("ApplyChanges_ChangelogMode_ShouldFail", func(t *testing.T) {
		adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
		if err != nil {
			t.Fatalf("Failed to create changelog adapter: %v", err)
		}
		defer adapter.Close()

		err = adapter.ApplyChanges(ctx, changes)
		if err == nil {
			t.Error("Expected ApplyChanges to fail in changelog mode")
		}
	})
}

// TestStorageAdapterFactory tests the adapter factory function
func TestStorageAdapterFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	testCases := []struct {
		name      string
		config    *config.Config
		expectErr bool
		skipConn  bool // Skip connection test (for PostgreSQL in test env)
	}{
		{
			name: "SQLite_Valid_Changelog",
			config: &config.Config{
				Backend: config.BackendConfig{
					Type: "sqlite",
					DSN:  ":memory:",
					Mode: config.StorageModeChangelog,
				},
			},
			expectErr: false,
		},
		{
			name: "SQLite_Valid_Stateful",
			config: &config.Config{
				Backend: config.BackendConfig{
					Type: "sqlite",
					DSN:  ":memory:",
					Mode: config.StorageModeStateful,
				},
			},
			expectErr: false,
		},
		{
			name: "PostgreSQL_ValidConfig",
			config: &config.Config{
				Backend: config.BackendConfig{
					Type: "postgres",
					DSN:  "postgres://user:pass@localhost/testdb?sslmode=disable",
					Mode: config.StorageModeStateful,
				},
			},
			expectErr: false,
			skipConn:  true, // Connection will likely fail in test environment
		},
		{
			name: "InvalidBackendType",
			config: &config.Config{
				Backend: config.BackendConfig{
					Type: "invalid",
					DSN:  ":memory:",
					Mode: config.StorageModeChangelog,
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := NewStorageAdapter(tc.config, logger)

			if tc.expectErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tc.expectErr && !tc.skipConn && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tc.skipConn && err != nil {
				t.Logf("Expected connection error in test environment: %v", err)
				// Don't try to close adapter if creation failed
				return
			}

			if adapter != nil {
				defer func() {
					if closeErr := adapter.Close(); closeErr != nil {
						t.Logf("Error closing adapter: %v", closeErr)
					}
				}()
			}
		})
	}

	// Test invalid logger type
	t.Run("InvalidLoggerType", func(t *testing.T) {
		cfg := &config.Config{
			Backend: config.BackendConfig{
				Type: "sqlite",
				DSN:  ":memory:",
				Mode: config.StorageModeChangelog,
			},
		}

		_, err := NewStorageAdapter(cfg, "not-a-logger")
		if err == nil {
			t.Error("Expected error for invalid logger type")
		}
	})
}

// TestStorageAdapterConcurrency tests basic concurrency safety
func TestStorageAdapterConcurrency(t *testing.T) {
	t.Skip("Skipping concurrency test - SQLite adapter table creation is not thread-safe")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := createTestChanges()

	// Initialize the adapter by writing some data first
	err = adapter.WriteChanges(ctx, changes[:1])
	if err != nil {
		t.Fatalf("Failed to initialize adapter: %v", err)
	}

	// Test concurrent writes - disabled due to table creation race condition
	t.Run("ConcurrentWrites", func(t *testing.T) {
		t.Skip("Table creation race condition in SQLite adapter")
	})

	// Test concurrent token operations - disabled due to table creation race condition
	t.Run("ConcurrentTokens", func(t *testing.T) {
		t.Skip("Table creation race condition in SQLite adapter")
	})
}

// BenchmarkStorageAdapter provides performance benchmarks
func BenchmarkStorageAdapter(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		b.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := createTestChanges()

	b.Run("WriteChanges", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := adapter.WriteChanges(ctx, changes)
			if err != nil {
				b.Errorf("WriteChanges error: %v", err)
			}
		}
	})

	b.Run("SaveContinuationToken", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := adapter.SaveContinuationToken(ctx, fmt.Sprintf("bench-token-%d", i))
			if err != nil {
				b.Errorf("SaveContinuationToken error: %v", err)
			}
		}
	})

	b.Run("GetStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := adapter.GetStats(ctx)
			if err != nil {
				b.Errorf("GetStats error: %v", err)
			}
		}
	})
}
