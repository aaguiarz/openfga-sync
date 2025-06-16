package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PostgresAdapter implements StorageAdapter for PostgreSQL
type PostgresAdapter struct {
	db     *sql.DB
	logger *logrus.Logger
	mode   config.StorageMode
}

// NewPostgresAdapter creates a new PostgreSQL storage adapter
func NewPostgresAdapter(dsn string, mode config.StorageMode, logger *logrus.Logger) (*PostgresAdapter, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	adapter := &PostgresAdapter{
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
func (p *PostgresAdapter) initSchema() error {
	var queries []string

	// Common sync state table
	queries = append(queries, []string{
		`CREATE TABLE IF NOT EXISTS sync_state (
			id SERIAL PRIMARY KEY,
			continuation_token TEXT,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`INSERT INTO sync_state (continuation_token) 
		 SELECT '' WHERE NOT EXISTS (SELECT 1 FROM sync_state)`,
	}...)

	// Mode-specific tables
	if p.mode == config.StorageModeChangelog {
		// Changelog mode: append-only table with all change events
		queries = append(queries, []string{
			`CREATE TABLE IF NOT EXISTS fga_changelog (
				id BIGSERIAL PRIMARY KEY,
				change_type VARCHAR(20) NOT NULL,
				object_type VARCHAR(100) NOT NULL,
				object_id VARCHAR(255) NOT NULL,
				relation VARCHAR(100) NOT NULL,
				user_type VARCHAR(100) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
				condition JSONB,
				raw_event JSONB,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_timestamp ON fga_changelog(timestamp)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_user_type ON fga_changelog(user_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_object_type ON fga_changelog(object_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_changelog_relation ON fga_changelog(relation)`,
		}...)
	} else {
		// Stateful mode: current state table
		queries = append(queries, []string{
			`CREATE TABLE IF NOT EXISTS fga_tuples (
				object_type VARCHAR(100) NOT NULL,
				object_id VARCHAR(255) NOT NULL,
				relation VARCHAR(100) NOT NULL,
				user_type VARCHAR(100) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				condition JSONB,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				PRIMARY KEY (object_type, object_id, relation, user_type, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_user_type ON fga_tuples(user_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_object_type ON fga_tuples(object_type)`,
			`CREATE INDEX IF NOT EXISTS idx_fga_tuples_relation ON fga_tuples(relation)`,
		}...)
	}

	for _, query := range queries {
		if _, err := p.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// WriteChanges writes a batch of change events to PostgreSQL (changelog mode)
func (p *PostgresAdapter) WriteChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	// Start OpenTelemetry span
	tracer := otel.Tracer("openfga-sync/storage")
	ctx, span := tracer.Start(ctx, "postgres.write_changes",
		trace.WithAttributes(
			attribute.Int("db.changes_count", len(changes)),
			attribute.String("db.storage_mode", string(p.mode)),
			attribute.String("db.system", "postgresql"),
		),
	)
	defer span.End()

	if len(changes) == 0 {
		return nil
	}

	if p.mode != config.StorageModeChangelog {
		err := fmt.Errorf("WriteChanges is only supported in changelog mode")
		span.RecordError(err)
		return err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO fga_changelog (change_type, object_type, object_id, relation, user_type, user_id, timestamp, condition, raw_event)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, change := range changes {
		rawEventJSON, err := json.Marshal(change)
		if err != nil {
			p.logger.WithError(err).Warn("Failed to marshal change event to JSON")
			rawEventJSON = []byte("{}")
		}

		// Handle condition - convert from JSON string to PostgreSQL JSONB
		var conditionJSONB interface{}
		if change.Condition != "" {
			conditionJSONB = change.Condition
		}

		_, err = stmt.ExecContext(ctx,
			change.Operation,
			change.ObjectType,
			change.ObjectID,
			change.Relation,
			change.UserType,
			change.UserID,
			change.Timestamp,
			conditionJSONB,
			string(rawEventJSON),
		)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to insert change: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Add success attributes to span
	span.SetAttributes(
		attribute.Int("db.rows_affected", len(changes)),
		attribute.String("db.operation", "insert"),
	)

	p.logger.WithField("changes_count", len(changes)).Info("Successfully wrote changes to changelog")
	return nil
}

// ApplyChanges applies a batch of changes to state table (stateful mode)
func (p *PostgresAdapter) ApplyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error {
	if len(changes) == 0 {
		return nil
	}

	if p.mode != config.StorageModeStateful {
		return fmt.Errorf("ApplyChanges is only supported in stateful mode")
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO fga_tuples (object_type, object_id, relation, user_type, user_id, condition)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (object_type, object_id, relation, user_type, user_id)
		DO UPDATE SET condition = EXCLUDED.condition, updated_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	deleteStmt, err := tx.PrepareContext(ctx, `
		DELETE FROM fga_tuples 
		WHERE object_type = $1 AND object_id = $2 AND relation = $3 AND user_type = $4 AND user_id = $5
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer deleteStmt.Close()

	var insertCount, deleteCount int
	for _, change := range changes {
		switch strings.ToUpper(change.Operation) {
		case "TUPLE_TO_USERSET_WRITE", "WRITE":
			// Handle condition - convert from JSON string to PostgreSQL JSONB
			var conditionJSONB interface{}
			if change.Condition != "" {
				conditionJSONB = change.Condition
			}

			_, err = insertStmt.ExecContext(ctx,
				change.ObjectType,
				change.ObjectID,
				change.Relation,
				change.UserType,
				change.UserID,
				conditionJSONB,
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
			p.logger.WithField("operation", change.Operation).Warn("Unknown operation type, skipping")
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"inserts": insertCount,
		"deletes": deleteCount,
	}).Info("Successfully applied changes to state table")
	return nil
}

// GetLastContinuationToken retrieves the last processed continuation token
func (p *PostgresAdapter) GetLastContinuationToken(ctx context.Context) (string, error) {
	var token string
	err := p.db.QueryRowContext(ctx, "SELECT continuation_token FROM sync_state ORDER BY id DESC LIMIT 1").Scan(&token)
	if err != nil {
		return "", fmt.Errorf("failed to get continuation token: %w", err)
	}
	return token, nil
}

// SaveContinuationToken saves the continuation token for resuming processing
func (p *PostgresAdapter) SaveContinuationToken(ctx context.Context, token string) error {
	_, err := p.db.ExecContext(ctx, "UPDATE sync_state SET continuation_token = $1, updated_at = NOW()", token)
	if err != nil {
		return fmt.Errorf("failed to save continuation token: %w", err)
	}
	return nil
}

// GetStats returns statistics about the PostgreSQL adapter
func (p *PostgresAdapter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Basic adapter info
	stats["adapter_type"] = "postgres"
	stats["storage_mode"] = string(p.mode)

	// Test database connection
	if err := p.db.PingContext(ctx); err != nil {
		stats["connection_status"] = "error"
		stats["connection_error"] = err.Error()
		return stats, nil
	}
	stats["connection_status"] = "healthy"

	// Get database-specific stats based on mode
	if p.mode == config.StorageModeChangelog {
		var count int64
		err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_changelog").Scan(&count)
		if err != nil {
			stats["query_error"] = err.Error()
		} else {
			stats["changelog_entries"] = count
		}

		// Get count by change type
		rows, err := p.db.QueryContext(ctx, "SELECT change_type, COUNT(*) FROM fga_changelog GROUP BY change_type")
		if err == nil {
			defer rows.Close()
			changeTypeStats := make(map[string]int64)
			for rows.Next() {
				var changeType string
				var count int64
				if err := rows.Scan(&changeType, &count); err == nil {
					changeTypeStats[changeType] = count
				}
			}
			stats["by_change_type"] = changeTypeStats
		}
	} else if p.mode == config.StorageModeStateful {
		var count int64
		err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fga_tuples").Scan(&count)
		if err != nil {
			stats["query_error"] = err.Error()
		} else {
			stats["current_tuples"] = count
		}

		// Get count by object type
		rows, err := p.db.QueryContext(ctx, "SELECT object_type, COUNT(*) FROM fga_tuples GROUP BY object_type")
		if err == nil {
			defer rows.Close()
			objectTypeStats := make(map[string]int64)
			for rows.Next() {
				var objectType string
				var count int64
				if err := rows.Scan(&objectType, &count); err == nil {
					objectTypeStats[objectType] = count
				}
			}
			stats["by_object_type"] = objectTypeStats
		}
	}

	return stats, nil
}

// Close closes the database connection
func (p *PostgresAdapter) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}
