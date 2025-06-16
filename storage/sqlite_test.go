package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

func TestNewSQLiteAdapter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name    string
		dsn     string
		mode    config.StorageMode
		wantErr bool
	}{
		{
			name:    "memory database changelog mode",
			dsn:     ":memory:",
			mode:    config.StorageModeChangelog,
			wantErr: false,
		},
		{
			name:    "memory database stateful mode",
			dsn:     ":memory:",
			mode:    config.StorageModeStateful,
			wantErr: false,
		},
		{
			name:    "file database changelog mode",
			dsn:     "/tmp/test_changelog.db",
			mode:    config.StorageModeChangelog,
			wantErr: false,
		},
		{
			name:    "file database stateful mode",
			dsn:     "/tmp/test_stateful.db",
			mode:    config.StorageModeStateful,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing file
			if tt.dsn != ":memory:" {
				os.Remove(tt.dsn)
				defer os.Remove(tt.dsn)
			}

			adapter, err := NewSQLiteAdapter(tt.dsn, tt.mode, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSQLiteAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if adapter != nil {
				defer adapter.Close()

				// Test basic functionality
				ctx := context.Background()

				// Test continuation token operations
				token, err := adapter.GetLastContinuationToken(ctx)
				if err != nil {
					t.Errorf("GetLastContinuationToken() error = %v", err)
				}
				if token != "" {
					t.Errorf("Expected empty token initially, got %s", token)
				}

				// Save a token
				testToken := "test-token-123"
				err = adapter.SaveContinuationToken(ctx, testToken)
				if err != nil {
					t.Errorf("SaveContinuationToken() error = %v", err)
				}

				// Retrieve the token
				retrievedToken, err := adapter.GetLastContinuationToken(ctx)
				if err != nil {
					t.Errorf("GetLastContinuationToken() error = %v", err)
				}
				if retrievedToken != testToken {
					t.Errorf("Expected token %s, got %s", testToken, retrievedToken)
				}
			}
		})
	}
}

func TestSQLiteAdapter_WriteChanges(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
		{
			Operation: "DELETE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "editor",
				UserType:   "user",
				UserID:     "bob",
			},
			Timestamp: time.Now(),
		},
	}

	err = adapter.WriteChanges(ctx, changes)
	if err != nil {
		t.Errorf("WriteChanges() error = %v", err)
	}

	// Test stats
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() error = %v", err)
	}

	if count, ok := stats["changelog_entries"].(int64); !ok || count != 2 {
		t.Errorf("Expected 2 changelog entries, got %v", stats["changelog_entries"])
	}
}

func TestSQLiteAdapter_ApplyChanges(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter, err := NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
		{
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "editor",
				UserType:   "user",
				UserID:     "bob",
			},
			Timestamp: time.Now(),
		},
	}

	err = adapter.ApplyChanges(ctx, changes)
	if err != nil {
		t.Errorf("ApplyChanges() error = %v", err)
	}

	// Apply a delete change
	deleteChanges := []fetcher.ChangeEvent{
		{
			Operation: "DELETE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
	}

	err = adapter.ApplyChanges(ctx, deleteChanges)
	if err != nil {
		t.Errorf("ApplyChanges() delete error = %v", err)
	}

	// Test stats
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() error = %v", err)
	}

	if count, ok := stats["current_tuples"].(int64); !ok || count != 1 {
		t.Errorf("Expected 1 current tuple, got %v", stats["current_tuples"])
	}
}

func TestSQLiteAdapter_ModeValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	changelogAdapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create changelog adapter: %v", err)
	}
	defer changelogAdapter.Close()

	statefulAdapter, err := NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
	if err != nil {
		t.Fatalf("Failed to create stateful adapter: %v", err)
	}
	defer statefulAdapter.Close()

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
	}

	// Test that WriteChanges fails in stateful mode
	err = statefulAdapter.WriteChanges(ctx, changes)
	if err == nil {
		t.Error("Expected WriteChanges to fail in stateful mode")
	}

	// Test that ApplyChanges fails in changelog mode
	err = changelogAdapter.ApplyChanges(ctx, changes)
	if err == nil {
		t.Error("Expected ApplyChanges to fail in changelog mode")
	}
}
