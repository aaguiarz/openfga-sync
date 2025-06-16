package storage

import (
	"context"
	"fmt"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

// StorageAdapter defines the interface for storage adapters
type StorageAdapter interface {
	// WriteChanges writes a batch of change events to storage in changelog mode
	WriteChanges(ctx context.Context, changes []fetcher.ChangeEvent) error

	// ApplyChanges applies a batch of changes to state table (stateful mode)
	ApplyChanges(ctx context.Context, changes []fetcher.ChangeEvent) error

	// GetLastContinuationToken retrieves the last processed continuation token
	GetLastContinuationToken(ctx context.Context) (string, error)

	// SaveContinuationToken saves the continuation token for resuming processing
	SaveContinuationToken(ctx context.Context, token string) error

	// GetStats returns statistics about the storage adapter
	GetStats(ctx context.Context) (map[string]interface{}, error)

	// Close closes the storage connection
	Close() error
}

// StorageMode represents the storage operation mode
type StorageMode string

const (
	StorageModeChangelog StorageMode = "changelog"
	StorageModeStateful  StorageMode = "stateful"
)

// NewStorageAdapter creates a storage adapter based on configuration
func NewStorageAdapter(cfg *config.Config, logger interface{}) (StorageAdapter, error) {
	switch cfg.Backend.Type {
	case "postgres":
		// Convert logger to the expected type
		if l, ok := logger.(*logrus.Logger); ok {
			return NewPostgresAdapter(cfg.Backend.DSN, cfg.Backend.Mode, l)
		}
		return nil, fmt.Errorf("invalid logger type for postgres adapter")
	case "sqlite":
		// Convert logger to the expected type
		if l, ok := logger.(*logrus.Logger); ok {
			return NewSQLiteAdapter(cfg.Backend.DSN, cfg.Backend.Mode, l)
		}
		return nil, fmt.Errorf("invalid logger type for sqlite adapter")
	case "openfga":
		// Convert logger to the expected type
		if l, ok := logger.(*logrus.Logger); ok {
			return NewOpenFGAAdapter(cfg.Backend.DSN, cfg.Backend.Mode, l)
		}
		return nil, fmt.Errorf("invalid logger type for openfga adapter")
	// TODO: Add other adapters as needed
	// case "mysql":
	//     return NewMySQLAdapter(cfg.Backend.DSN, cfg.Backend.Mode, logger)
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", cfg.Backend.Type)
	}
}
