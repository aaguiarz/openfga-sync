package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/aaguiarz/openfga-sync/storage"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		logger.Warn("Invalid log level, defaulting to info")
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	logger.WithFields(logrus.Fields{
		"version":         "1.0.0",
		"openfga_endpoint": cfg.OpenFGA.Endpoint,
		"openfga_store":   cfg.OpenFGA.StoreID,
		"backend_type":    cfg.Backend.Type,
		"storage_mode":    cfg.Backend.Mode,
		"poll_interval":   cfg.Service.PollInterval,
	}).Info("Starting OpenFGA sync service")

	// Initialize storage adapter
	storageAdapter, err := storage.NewStorageAdapter(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize storage adapter")
	}
	defer storageAdapter.Close()

	// Initialize OpenFGA fetcher
	fgaFetcher, err := fetcher.NewOpenFGAFetcher(
		cfg.OpenFGA.Endpoint,
		cfg.OpenFGA.StoreID,
		cfg.OpenFGA.Token,
		logger,
	)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize OpenFGA fetcher")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, gracefully stopping...")
		cancel()
	}()

	// Start the sync process
	logger.Info("OpenFGA sync service started successfully")
	if err := runSyncLoop(ctx, fgaFetcher, storageAdapter, cfg, logger); err != nil {
		logger.WithError(err).Error("Sync loop failed")
	}

	logger.Info("OpenFGA sync service stopped")
}

// runSyncLoop runs the main synchronization loop
func runSyncLoop(ctx context.Context, fgaFetcher *fetcher.OpenFGAFetcher, storageAdapter storage.StorageAdapter, cfg *config.Config, logger *logrus.Logger) error {
	// Get the last continuation token
	continuationToken, err := storageAdapter.GetLastContinuationToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last continuation token: %w", err)
	}

	logger.WithField("continuation_token", continuationToken).Info("Starting sync from continuation token")

	ticker := time.NewTicker(cfg.Service.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := syncChanges(ctx, fgaFetcher, storageAdapter, cfg, &continuationToken, logger); err != nil {
				logger.WithError(err).Error("Failed to sync changes")
				// Continue running despite errors
			}
		}
	}
}

// syncChanges fetches and stores changes from OpenFGA
func syncChanges(ctx context.Context, fgaFetcher *fetcher.OpenFGAFetcher, storageAdapter storage.StorageAdapter, cfg *config.Config, continuationToken *string, logger *logrus.Logger) error {
	changes, nextToken, err := fgaFetcher.FetchChanges(ctx, *continuationToken)
	if err != nil {
		return fmt.Errorf("failed to fetch changes: %w", err)
	}

	if len(changes) == 0 {
		logger.Debug("No new changes found")
		return nil
	}

	// Apply changes based on storage mode
	if cfg.IsChangelogMode() {
		if err := storageAdapter.WriteChanges(ctx, changes); err != nil {
			return fmt.Errorf("failed to write changes: %w", err)
		}
	} else if cfg.IsStatefulMode() {
		if err := storageAdapter.ApplyChanges(ctx, changes); err != nil {
			return fmt.Errorf("failed to apply changes: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported storage mode: %s", cfg.Backend.Mode)
	}

	if nextToken != "" {
		if err := storageAdapter.SaveContinuationToken(ctx, nextToken); err != nil {
			return fmt.Errorf("failed to save continuation token: %w", err)
		}
		*continuationToken = nextToken
	}

	logger.WithFields(logrus.Fields{
		"changes_processed": len(changes),
		"next_token":        nextToken,
		"storage_mode":      cfg.Backend.Mode,
	}).Info("Successfully processed changes batch")

	return nil
}
