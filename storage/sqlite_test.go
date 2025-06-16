package storage

import (
	"context"
	"os"
	"strings"
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
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			Timestamp:  time.Now(),
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
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			Timestamp:  time.Now(),
		},
	}

	err = adapter.ApplyChanges(ctx, changes)
	if err != nil {
		t.Errorf("ApplyChanges() error = %v", err)
	}

	// Apply a delete change
	deleteChanges := []fetcher.ChangeEvent{
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
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

func TestSQLiteAdapter_ConditionSupport(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Test changelog mode with conditions
	changelogAdapter, err := NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create changelog adapter: %v", err)
	}
	defer changelogAdapter.Close()

	// Test stateful mode with conditions
	statefulAdapter, err := NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
	if err != nil {
		t.Fatalf("Failed to create stateful adapter: %v", err)
	}
	defer statefulAdapter.Close()

	ctx := context.Background()

	// Test changes with conditions
	changesWithConditions := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "sensitive_doc",
			Relation:   "viewer",
			UserType:   "employee",
			UserID:     "alice",
			Condition:  `{"name":"ip_allowlist","context":{"allowed_ips":["192.168.1.1"]}}`,
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "folder",
			ObjectID:   "financial_reports",
			Relation:   "editor",
			UserType:   "employee",
			UserID:     "bob",
			Condition:  `{"name":"time_based"}`,
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "public_doc",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "charlie",
			Condition:  "", // No condition
			Timestamp:  time.Now(),
		},
	}

	// Test changelog mode with conditions
	t.Run("changelog_mode_with_conditions", func(t *testing.T) {
		err := changelogAdapter.WriteChanges(ctx, changesWithConditions)
		if err != nil {
			t.Errorf("WriteChanges() error = %v", err)
		}

		// Verify data was stored with conditions
		rows, err := changelogAdapter.db.QueryContext(ctx, "SELECT object_id, condition FROM fga_changelog WHERE condition IS NOT NULL ORDER BY object_id")
		if err != nil {
			t.Fatalf("Failed to query changelog: %v", err)
		}
		defer rows.Close()

		conditionCount := 0
		for rows.Next() {
			var objectID, condition string
			if err := rows.Scan(&objectID, &condition); err != nil {
				t.Fatalf("Failed to scan row: %v", err)
			}

			conditionCount++
			if objectID == "sensitive_doc" {
				if !strings.Contains(condition, "ip_allowlist") {
					t.Errorf("Expected condition with ip_allowlist for sensitive_doc, got: %s", condition)
				}
			} else if objectID == "financial_reports" {
				if !strings.Contains(condition, "time_based") {
					t.Errorf("Expected condition with time_based for financial_reports, got: %s", condition)
				}
			}
		}

		if conditionCount != 2 {
			t.Errorf("Expected 2 records with conditions, got %d", conditionCount)
		}
	})

	// Test stateful mode with conditions
	t.Run("stateful_mode_with_conditions", func(t *testing.T) {
		err := statefulAdapter.ApplyChanges(ctx, changesWithConditions)
		if err != nil {
			t.Errorf("ApplyChanges() error = %v", err)
		}

		// Verify data was stored with conditions
		rows, err := statefulAdapter.db.QueryContext(ctx, "SELECT object_id, condition FROM fga_tuples WHERE condition IS NOT NULL ORDER BY object_id")
		if err != nil {
			t.Fatalf("Failed to query tuples: %v", err)
		}
		defer rows.Close()

		conditionCount := 0
		for rows.Next() {
			var objectID, condition string
			if err := rows.Scan(&objectID, &condition); err != nil {
				t.Fatalf("Failed to scan row: %v", err)
			}

			conditionCount++
			if objectID == "financial_reports" {
				if !strings.Contains(condition, "time_based") {
					t.Errorf("Expected condition with time_based for financial_reports, got: %s", condition)
				}
			} else if objectID == "sensitive_doc" {
				if !strings.Contains(condition, "ip_allowlist") {
					t.Errorf("Expected condition with ip_allowlist for sensitive_doc, got: %s", condition)
				}
			}
		}

		if conditionCount != 2 {
			t.Errorf("Expected 2 records with conditions, got %d", conditionCount)
		}

		// Test that records without conditions also exist
		var totalCount int
		err = statefulAdapter.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_tuples").Scan(&totalCount)
		if err != nil {
			t.Fatalf("Failed to count total tuples: %v", err)
		}

		if totalCount != 3 {
			t.Errorf("Expected 3 total tuples, got %d", totalCount)
		}
	})

	// Test condition updates in stateful mode
	t.Run("condition_updates_stateful", func(t *testing.T) {
		// Update the condition for an existing tuple
		updateChanges := []fetcher.ChangeEvent{
			{
				Operation:  "WRITE",
				ObjectType: "document",
				ObjectID:   "sensitive_doc",
				Relation:   "viewer",
				UserType:   "employee",
				UserID:     "alice",
				Condition:  `{"name":"geo_restriction","context":{"allowed_countries":["US","CA"]}}`,
				Timestamp:  time.Now(),
			},
		}

		err := statefulAdapter.ApplyChanges(ctx, updateChanges)
		if err != nil {
			t.Errorf("ApplyChanges() update error = %v", err)
		}

		// Verify the condition was updated
		var condition string
		err = statefulAdapter.db.QueryRowContext(ctx,
			"SELECT condition FROM fga_tuples WHERE object_type = ? AND object_id = ? AND relation = ? AND user_type = ? AND user_id = ?",
			"document", "sensitive_doc", "viewer", "employee", "alice").Scan(&condition)
		if err != nil {
			t.Fatalf("Failed to query updated condition: %v", err)
		}

		if !strings.Contains(condition, "geo_restriction") {
			t.Errorf("Expected updated condition with geo_restriction, got: %s", condition)
		}
	})
}
