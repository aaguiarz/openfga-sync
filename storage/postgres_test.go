package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// skipIfNoPostgreSQL skips the test if PostgreSQL is not available
func skipIfNoPostgreSQL(t *testing.T) string {
	// Check if PostgreSQL environment variables are set
	dbHost := os.Getenv("POSTGRES_HOST")
	dbPort := os.Getenv("POSTGRES_PORT")
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")

	// Use defaults if not set
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	if dbUser == "" {
		dbUser = "postgres"
	}
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	if dbName == "" {
		dbName = "openfga_sync_test"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Try to connect to PostgreSQL
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	return dsn
}

func TestNewPostgresAdapter(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name    string
		dsn     string
		mode    config.StorageMode
		wantErr bool
	}{
		{
			name:    "Valid DSN - Changelog mode",
			dsn:     dsn,
			mode:    config.StorageModeChangelog,
			wantErr: false,
		},
		{
			name:    "Valid DSN - Stateful mode",
			dsn:     dsn,
			mode:    config.StorageModeStateful,
			wantErr: false,
		},
		{
			name:    "Invalid DSN",
			dsn:     "invalid_dsn",
			mode:    config.StorageModeChangelog,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewPostgresAdapter(tt.dsn, tt.mode, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPostgresAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if adapter != nil {
				adapter.Close()
			}
		})
	}
}

func TestPostgresAdapter_WriteChanges(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter, err := NewPostgresAdapter(dsn, config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "doc123",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "doc456",
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

	// Verify data was stored
	var count int
	err = adapter.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_changelog").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query changelog: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 records, got %d", count)
	}
}

func TestPostgresAdapter_ApplyChanges(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter, err := NewPostgresAdapter(dsn, config.StorageModeStateful, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	changes := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "doc123",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "doc123",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "doc123",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
	}

	err = adapter.ApplyChanges(ctx, changes)
	if err != nil {
		t.Errorf("ApplyChanges() error = %v", err)
	}

	// Verify final state (should only have bob as editor)
	var count int
	err = adapter.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_tuples").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tuples: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 record, got %d", count)
	}

	// Verify the remaining record is bob as editor
	var userID, relation string
	err = adapter.db.QueryRowContext(ctx, "SELECT user_id, relation FROM fga_tuples").Scan(&userID, &relation)
	if err != nil {
		t.Fatalf("Failed to query tuple details: %v", err)
	}

	if userID != "bob" || relation != "editor" {
		t.Errorf("Expected bob as editor, got %s as %s", userID, relation)
	}
}

func TestPostgresAdapter_ConditionSupport(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Test changelog mode with conditions
	changelogAdapter, err := NewPostgresAdapter(dsn, config.StorageModeChangelog, logger)
	if err != nil {
		t.Fatalf("Failed to create changelog adapter: %v", err)
	}
	defer changelogAdapter.Close()

	// Test stateful mode with conditions
	statefulAdapter, err := NewPostgresAdapter(dsn, config.StorageModeStateful, logger)
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
			Condition:  `{"name":"time_based","context":{"start_time":"09:00","end_time":"17:00"}}`,
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
			var objectID string
			var condition sql.NullString
			if err := rows.Scan(&objectID, &condition); err != nil {
				t.Fatalf("Failed to scan row: %v", err)
			}

			if condition.Valid {
				conditionCount++
				conditionStr := condition.String
				if objectID == "sensitive_doc" {
					if !strings.Contains(conditionStr, "ip_allowlist") {
						t.Errorf("Expected condition with ip_allowlist for sensitive_doc, got: %s", conditionStr)
					}
				} else if objectID == "financial_reports" {
					if !strings.Contains(conditionStr, "time_based") {
						t.Errorf("Expected condition with time_based for financial_reports, got: %s", conditionStr)
					}
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
			var objectID string
			var condition sql.NullString
			if err := rows.Scan(&objectID, &condition); err != nil {
				t.Fatalf("Failed to scan row: %v", err)
			}

			if condition.Valid {
				conditionCount++
				conditionStr := condition.String
				if objectID == "financial_reports" {
					if !strings.Contains(conditionStr, "time_based") {
						t.Errorf("Expected condition with time_based for financial_reports, got: %s", conditionStr)
					}
				} else if objectID == "sensitive_doc" {
					if !strings.Contains(conditionStr, "ip_allowlist") {
						t.Errorf("Expected condition with ip_allowlist for sensitive_doc, got: %s", conditionStr)
					}
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
		var condition sql.NullString
		err = statefulAdapter.db.QueryRowContext(ctx,
			"SELECT condition FROM fga_tuples WHERE object_type = $1 AND object_id = $2 AND relation = $3 AND user_type = $4 AND user_id = $5",
			"document", "sensitive_doc", "viewer", "employee", "alice").Scan(&condition)
		if err != nil {
			t.Fatalf("Failed to query updated condition: %v", err)
		}

		if !condition.Valid || !strings.Contains(condition.String, "geo_restriction") {
			t.Errorf("Expected updated condition with geo_restriction, got: %s", condition.String)
		}
	})

	// Test complex JSON conditions
	t.Run("complex_json_conditions", func(t *testing.T) {
		complexChanges := []fetcher.ChangeEvent{
			{
				Operation:  "WRITE",
				ObjectType: "file",
				ObjectID:   "confidential_file",
				Relation:   "viewer",
				UserType:   "employee",
				UserID:     "david",
				Condition:  `{"name":"complex_condition","context":{"departments":["finance","hr"],"security_level":5,"valid_until":"2024-12-31T23:59:59Z"}}`,
				Timestamp:  time.Now(),
			},
		}

		err := statefulAdapter.ApplyChanges(ctx, complexChanges)
		if err != nil {
			t.Errorf("ApplyChanges() complex condition error = %v", err)
		}

		// Verify complex JSON was stored correctly
		var condition sql.NullString
		err = statefulAdapter.db.QueryRowContext(ctx,
			"SELECT condition FROM fga_tuples WHERE object_type = $1 AND object_id = $2 AND user_id = $3",
			"file", "confidential_file", "david").Scan(&condition)
		if err != nil {
			t.Fatalf("Failed to query complex condition: %v", err)
		}

		if !condition.Valid {
			t.Error("Expected complex condition to be stored")
		} else {
			conditionStr := condition.String
			if !strings.Contains(conditionStr, "complex_condition") ||
				!strings.Contains(conditionStr, "finance") ||
				!strings.Contains(conditionStr, "security_level") {
				t.Errorf("Complex condition not stored correctly, got: %s", conditionStr)
			}
		}
	})

	// Test invalid JSON conditions (should be handled gracefully)
	t.Run("invalid_json_conditions", func(t *testing.T) {
		invalidChanges := []fetcher.ChangeEvent{
			{
				Operation:  "WRITE",
				ObjectType: "file",
				ObjectID:   "test_file",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "eve",
				Condition:  `{invalid_json: missing_quotes}`, // Invalid JSON
				Timestamp:  time.Now(),
			},
		}

		// This should not fail - invalid JSON should be stored as-is
		err := statefulAdapter.ApplyChanges(ctx, invalidChanges)
		if err != nil {
			t.Errorf("ApplyChanges() should handle invalid JSON gracefully, error = %v", err)
		}

		// Verify the invalid JSON was still stored
		var condition sql.NullString
		err = statefulAdapter.db.QueryRowContext(ctx,
			"SELECT condition FROM fga_tuples WHERE object_type = $1 AND object_id = $2 AND user_id = $3",
			"file", "test_file", "eve").Scan(&condition)
		if err != nil {
			t.Fatalf("Failed to query invalid condition: %v", err)
		}

		if !condition.Valid {
			t.Error("Expected invalid condition to be stored as-is")
		}
	})
}
