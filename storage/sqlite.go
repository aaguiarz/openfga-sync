package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// SQLiteAdapter implements StorageAdapter for SQLite
type SQLiteAdapter struct {
	db     *sql.DB
	logger *logrus.Logger
	mode   config.StorageMode
}

// NewSQLiteAdapter creates a new SQLite storage adapter
func NewSQLiteAdapter(dsn string, mode config.StorageMode, logger *logrus.Logger) (*SQLiteAdapter, error) {
	// SQLite DSN format: file:path/to/db.sqlite?cache=shared&mode=rwc
	// If no file prefix, add it
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		dsn = "file:" + dsn
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign keys and WAL mode for better performance
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	adapter := &SQLiteAdapter{
		db:     db,
		logger: logger,
		mode:   mode,
	}

	// Initialize database schema
	if err := adapter.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return adapter, nil
}

// initSchema creates the necessary database tables
func (s *SQLiteAdapter) initSchema() error {
	var queries []string

	// Common sync state table
	queries = append(queries, []string{
		`CREATE TABLE IF NOT EXISTS sync_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			continuation_token TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO sync_state (id, continuation_token) VALUES (1, '')`,
	}...)

	// Mode-specific tables
	if s.mode == config.StorageModeChangelog {
		// Changelog mode: append-only table with all change events
		queries = append(queries, []string{
			`CREATE TABLE IF NOT EXISTS fga_changelog (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				change_type TEXT NOT NULL,
				object_type TEXT NOT NULL,
				object_id TEXT NOT NULL,
				relation TEXT NOT NULL,
				user_type TEXT NOT NULL,
				user_id TEXT NOT NULL,
				timestamp DATETIME NOT NULL,
				condition TEXT,
				raw_event TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_timestamp ON fga_changelog(timestamp)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_user_type ON fga_changelog(user_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_object_type ON fga_changelog(object_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_relation ON fga_changelog(relation)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_change_type ON fga_changelog(change_type)`,
		}...)
	} else {
		// Stateful mode: current state table
		queries = append(queries, []string{
			`CREATE TABLE IF NOT EXISTS fga_tuples (
				object_type TEXT NOT NULL,
				object_id TEXT NOT NULL,
				relation TEXT NOT NULL,
				user_type TEXT NOT NULL,
				user_id TEXT NOT NULL,
				condition TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (object_type, object_id, relation, user_type, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_user_type ON fga_tuples(user_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_object_type ON fga_tuples(object_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_relation ON fga_tuples(relation)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_updated_at ON fga_tuples(updated_at)`,
		}...)
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query '%s': %w", query, err)
		}
	}

	return nil
}

// WriteChanges writes a batch of change events to SQLite (changelog mode)
func (s *SQLiteAdapter) WriteChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if len(changes) == 0 {
		return nil
	}

	if s.mode != config.StorageModeChangelog {
		return fmt.Errorf("WriteChanges is only supported in changelog mode")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO fga_changelog (change_type, object_type, object_id, relation, user_type, user_id, timestamp, condition, raw_event)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, change := range changes {
		rawEventJSON, err := json.Marshal(change)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to marshal change event to JSON")
			rawEventJSON = []byte("{}")
		}

		// Handle condition - store as JSON string in TEXT field
		var conditionText interface{}
		if change.Condition != "" {
			conditionText = change.Condition
		}

		_, err = stmt.ExecContext(ctx,
			change.Operation,
			change.ObjectType,
			change.ObjectID,
			change.Relation,
			change.UserType,
			change.UserID,
			change.Timestamp.Format("2006-01-02 15:04:05.000"),
			conditionText,
			string(rawEventJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert change: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.WithField("changes_count", len(changes)).Info("Successfully wrote changes to changelog")
	return nil
}

// ApplyChanges applies a batch of changes to state table (stateful mode)
func (s *SQLiteAdapter) ApplyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if len(changes) == 0 {
		return nil
	}

	if s.mode != config.StorageModeStateful {
		return fmt.Errorf("ApplyChanges is only supported in stateful mode")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// SQLite uses INSERT OR REPLACE for upsert functionality
	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO fga_tuples (object_type, object_id, relation, user_type, user_id, condition, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 
			COALESCE((SELECT created_at FROM fga_tuples WHERE object_type = ? AND object_id = ? AND relation = ? AND user_type = ? AND user_id = ?), CURRENT_TIMESTAMP),
			CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	deleteStmt, err := tx.PrepareContext(ctx, `
		DELETE FROM fga_tuples 
		WHERE object_type = ? AND object_id = ? AND relation = ? AND user_type = ? AND user_id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer deleteStmt.Close()

	var insertCount, deleteCount int
	for _, change := range changes {
		switch strings.ToUpper(change.Operation) {
		case "TUPLE_TO_USERSET_WRITE", "WRITE":
			// Handle condition - store as JSON string in TEXT field
			var conditionText interface{}
			if change.Condition != "" {
				conditionText = change.Condition
			}

			_, err = insertStmt.ExecContext(ctx,
				change.ObjectType,
				change.ObjectID,
				change.Relation,
				change.UserType,
				change.UserID,
				conditionText,
				// Parameters for the COALESCE subquery
				change.ObjectType,
				change.ObjectID,
				change.Relation,
				change.UserType,
				change.UserID,
			)
			if err != nil {
				return fmt.Errorf("failed to insert/update tuple: %w", err)
			}
			insertCount++
		case "TUPLE_TO_USERSET_DELETE", "DELETE":
			_, err = deleteStmt.ExecContext(ctx,
				change.ObjectType,
				change.ObjectID,
				change.Relation,
				change.UserType,
				change.UserID,
			)
			if err != nil {
				return fmt.Errorf("failed to delete tuple: %w", err)
			}
			deleteCount++
		default:
			s.logger.WithField("operation", change.Operation).Warn("Unknown operation type, skipping")
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"inserts": insertCount,
		"deletes": deleteCount,
	}).Info("Successfully applied changes to state table")
	return nil
}

// GetLastContinuationToken retrieves the last processed continuation token
func (s *SQLiteAdapter) GetLastContinuationToken(ctx context.Context) (string, error) {
	var token string
	err := s.db.QueryRowContext(ctx, "SELECT continuation_token FROM sync_state WHERE id = 1").Scan(&token)
	if err != nil {
		return "", fmt.Errorf("failed to get continuation token: %w", err)
	}
	return token, nil
}

// SaveContinuationToken saves the continuation token for resuming processing
func (s *SQLiteAdapter) SaveContinuationToken(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE sync_state SET continuation_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", token)
	if err != nil {
		return fmt.Errorf("failed to save continuation token: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *SQLiteAdapter) Close() error {
	return s.db.Close()
}

// GetStats returns statistics about the SQLite database
func (s *SQLiteAdapter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	if s.mode == config.StorageModeChangelog {
		var count int64
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_changelog").Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to get changelog count: %w", err)
		}
		stats["changelog_entries"] = count

		// Get count by change type
		rows, err := s.db.QueryContext(ctx, "SELECT change_type, COUNT(*) FROM fga_changelog GROUP BY change_type")
		if err != nil {
			return nil, fmt.Errorf("failed to get changelog stats by type: %w", err)
		}
		defer rows.Close()

		changeTypeStats := make(map[string]int64)
		for rows.Next() {
			var changeType string
			var count int64
			if err := rows.Scan(&changeType, &count); err != nil {
				return nil, fmt.Errorf("failed to scan changelog stats: %w", err)
			}
			changeTypeStats[changeType] = count
		}
		stats["by_change_type"] = changeTypeStats
	} else {
		var count int64
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_tuples").Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to get tuples count: %w", err)
		}
		stats["current_tuples"] = count

		// Get count by object type
		rows, err := s.db.QueryContext(ctx, "SELECT object_type, COUNT(*) FROM fga_tuples GROUP BY object_type")
		if err != nil {
			return nil, fmt.Errorf("failed to get tuples stats by object type: %w", err)
		}
		defer rows.Close()

		objectTypeStats := make(map[string]int64)
		for rows.Next() {
			var objectType string
			var count int64
			if err := rows.Scan(&objectType, &count); err != nil {
				return nil, fmt.Errorf("failed to scan tuples stats: %w", err)
			}
			objectTypeStats[objectType] = count
		}
		stats["by_object_type"] = objectTypeStats
	}

	return stats, nil
}
