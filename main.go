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
	"github.com/aaguiarz/openfga-sync/metrics"
	"github.com/aaguiarz/openfga-sync/server"
	"github.com/aaguiarz/openfga-sync/storage"
	"github.com/aaguiarz/openfga-sync/telemetry"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
		"version":          "1.0.0",
		"openfga_endpoint": cfg.OpenFGA.Endpoint,
		"openfga_store":    cfg.OpenFGA.StoreID,
		"backend_type":     cfg.Backend.Type,
		"storage_mode":     cfg.Backend.Mode,
		"poll_interval":    cfg.Service.PollInterval,
		"server_port":      cfg.Server.Port,
		"metrics_enabled":  cfg.Observability.Metrics.Enabled,
	}).Info("Starting OpenFGA sync service")

	// Initialize OpenTelemetry
	telemetryProvider, err := telemetry.InitOpenTelemetry(context.Background(), cfg)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize OpenTelemetry")
	}

	if cfg.Observability.OpenTelemetry.Enabled {
		logger.WithField("otel_endpoint", cfg.Observability.OpenTelemetry.Endpoint).Info("OpenTelemetry initialized")
	}

	// Initialize metrics
	metricsCollector := metrics.New()

	// Initialize HTTP server
	httpServer := server.New(cfg, logger, metricsCollector)

	// Initialize storage adapter
	storageAdapter, err := storage.NewStorageAdapter(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize storage adapter")
	}

	// Initialize OpenFGA fetcher with enhanced options
	fetchOptions := fetcher.FetchOptions{
		PageSize:   cfg.Service.BatchSize,
		MaxChanges: cfg.Service.MaxChanges,
		Timeout:    cfg.Service.RequestTimeout,
		RetryConfig: fetcher.RetryConfig{
			MaxRetries:    cfg.Service.MaxRetries,
			InitialDelay:  cfg.Service.RetryDelay,
			MaxDelay:      cfg.Service.MaxRetryDelay,
			BackoffFactor: cfg.Service.BackoffFactor,
		},
		RateLimitDelay:   cfg.Service.RateLimitDelay,
		EnableValidation: cfg.Service.EnableValidation,
	}

	fgaFetcher, err := fetcher.NewOpenFGAFetcherWithOptions(
		cfg.OpenFGA.Endpoint,
		cfg.OpenFGA.StoreID,
		cfg.OpenFGA.Token,
		logger,
		fetchOptions,
	)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize OpenFGA fetcher")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server
	if err := httpServer.Start(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to start HTTP server")
	}

	// Setup enhanced signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Enhanced shutdown handler
	go func() {
		sig := <-sigChan
		logger.WithField("signal", sig.String()).Info("Received shutdown signal, initiating graceful shutdown...")
		
		// Start shutdown process
		cancel()
		
		// Set a hard timeout for complete shutdown
		shutdownTimer := time.NewTimer(30 * time.Second)
		defer shutdownTimer.Stop()
		
		// Wait for second signal to force immediate shutdown
		go func() {
			select {
			case sig2 := <-sigChan:
				logger.WithField("signal", sig2.String()).Warn("Received second shutdown signal, forcing immediate exit")
				os.Exit(1)
			case <-shutdownTimer.C:
				logger.Error("Shutdown timeout exceeded, forcing exit")
				os.Exit(1)
			case <-ctx.Done():
				// Normal shutdown completed
				return
			}
		}()
	}()

	// Mark service as ready after initialization
	httpServer.SetReady(true)

	// Start background goroutine to monitor storage connection status
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Check storage connection status
				if stats, err := storageAdapter.GetStats(ctx); err == nil {
					if status, ok := stats["connection_status"].(string); ok {
						metricsCollector.UpdateStorageConnectionStatus(status == "healthy" || status == "connected")
					}
				} else {
					metricsCollector.UpdateStorageConnectionStatus(false)
				}
			}
		}
	}()

	// Start the sync process
	logger.Info("OpenFGA sync service started successfully")
	
	// Run the sync loop until shutdown
	syncErr := runSyncLoop(ctx, fgaFetcher, storageAdapter, cfg, logger, metricsCollector)
	
	// Begin graceful shutdown
	logger.Info("Beginning graceful shutdown...")
	
	// Mark service as not ready
	httpServer.SetReady(false)
	logger.Debug("Service marked as not ready")
	
	// Stop HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := httpServer.Stop(shutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to stop HTTP server gracefully")
	} else {
		logger.Debug("HTTP server stopped gracefully")
	}
	shutdownCancel()
	
	// Close storage adapter
	if err := storageAdapter.Close(); err != nil {
		logger.WithError(err).Error("Failed to close storage adapter gracefully")
	} else {
		logger.Debug("Storage adapter closed gracefully")
	}
	
	// Close OpenFGA fetcher
	fgaFetcher.Close()
	logger.Debug("OpenFGA fetcher closed gracefully")
	
	// Shutdown OpenTelemetry
	telemetryShutdownCtx, telemetryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := telemetryProvider.Shutdown(telemetryShutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to shutdown OpenTelemetry gracefully")
	} else {
		logger.Debug("OpenTelemetry shutdown gracefully")
	}
	telemetryCancel()
	
	// Log final sync error if any
	if syncErr != nil {
		logger.WithError(syncErr).Error("Sync loop terminated with error")
	}
	
	logger.Info("OpenFGA sync service stopped gracefully")
}

// runSyncLoop runs the main synchronization loop
func runSyncLoop(ctx context.Context, fgaFetcher *fetcher.OpenFGAFetcher, storageAdapter storage.StorageAdapter, cfg *config.Config, logger *logrus.Logger, metrics *metrics.Metrics) error {
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
			if err := syncChanges(ctx, fgaFetcher, storageAdapter, cfg, &continuationToken, logger, metrics); err != nil {
				logger.WithError(err).Error("Failed to sync changes")
				metrics.RecordChangesError()
				// Continue running despite errors
			}
		}
	}
}

// syncChanges fetches and stores changes from OpenFGA
func syncChanges(ctx context.Context, fgaFetcher *fetcher.OpenFGAFetcher, storageAdapter storage.StorageAdapter, cfg *config.Config, continuationToken *string, logger *logrus.Logger, metrics *metrics.Metrics) error {
	// Start OpenTelemetry span for the entire sync operation
	tracer := otel.Tracer("openfga-sync/main")
	ctx, span := tracer.Start(ctx, "sync.changes",
		trace.WithAttributes(
			attribute.String("sync.continuation_token", *continuationToken),
			attribute.String("sync.storage_mode", string(cfg.Backend.Mode)),
			attribute.String("sync.storage_type", cfg.Backend.Type),
			attribute.Int64("sync.batch_size", int64(cfg.Service.BatchSize)),
		),
	)
	defer span.End()

	syncStart := time.Now()
	defer func() {
		metrics.RecordSyncDuration(time.Since(syncStart))
	}()

	// Use enhanced fetch with retry logic
	fetchStart := time.Now()
	result, err := fgaFetcher.FetchChangesWithRetry(ctx, *continuationToken, cfg.Service.BatchSize)
	fetchDuration := time.Since(fetchStart)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.type", "fetch_error"))
		metrics.RecordOpenFGARequest("error", fetchDuration, "changes")
		return fmt.Errorf("failed to fetch changes: %w", err)
	}

	metrics.RecordOpenFGARequest("success", fetchDuration, "changes")

	if len(result.Changes) == 0 {
		span.SetAttributes(attribute.Int("sync.changes_found", 0))
		logger.Debug("No new changes found")
		return nil
	}

	// Add span attributes for the fetched data
	span.SetAttributes(
		attribute.Int("sync.changes_found", len(result.Changes)),
		attribute.String("sync.next_token", result.ContinuationToken),
		attribute.Bool("sync.has_more", result.HasMore),
	)

	// Log fetcher statistics
	stats := fgaFetcher.GetStats()
	logger.WithFields(logrus.Fields{
		"total_requests":   stats.TotalRequests,
		"success_requests": stats.SuccessRequests,
		"failed_requests":  stats.FailedRequests,
		"average_latency":  fmt.Sprintf("%.2fms", stats.AverageLatency),
	}).Debug("Fetcher statistics")

	// Apply changes based on storage mode
	storageStart := time.Now()
	var storageErr error

	if cfg.IsChangelogMode() {
		storageErr = storageAdapter.WriteChanges(ctx, result.Changes)
		if storageErr != nil {
			span.RecordError(storageErr)
			span.SetAttributes(attribute.String("error.type", "storage_write_error"))
			metrics.RecordStorageOperation("write", "error", time.Since(storageStart))
			return fmt.Errorf("failed to write changes: %w", storageErr)
		}
		metrics.RecordStorageOperation("write", "success", time.Since(storageStart))
		span.SetAttributes(attribute.String("sync.storage_operation", "write"))
	} else if cfg.IsStatefulMode() {
		storageErr = storageAdapter.ApplyChanges(ctx, result.Changes)
		if storageErr != nil {
			span.RecordError(storageErr)
			span.SetAttributes(attribute.String("error.type", "storage_apply_error"))
			metrics.RecordStorageOperation("apply", "error", time.Since(storageStart))
			return fmt.Errorf("failed to apply changes: %w", storageErr)
		}
		metrics.RecordStorageOperation("apply", "success", time.Since(storageStart))
		span.SetAttributes(attribute.String("sync.storage_operation", "apply"))
	} else {
		err := fmt.Errorf("unsupported storage mode: %s", cfg.Backend.Mode)
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.type", "invalid_storage_mode"))
		return err
	}

	// Record successful change processing
	metrics.RecordChangesProcessed(len(result.Changes))

	if result.ContinuationToken != "" {
		tokenStart := time.Now()
		if err := storageAdapter.SaveContinuationToken(ctx, result.ContinuationToken); err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.type", "token_save_error"))
			metrics.RecordStorageOperation("save_token", "error", time.Since(tokenStart))
			return fmt.Errorf("failed to save continuation token: %w", err)
		}
		metrics.RecordStorageOperation("save_token", "success", time.Since(tokenStart))
		*continuationToken = result.ContinuationToken
	}

	// Calculate and record lag if we have changes with timestamps
	if len(result.Changes) > 0 {
		// Get the timestamp of the most recent change
		var mostRecentChange time.Time
		for _, change := range result.Changes {
			if change.Timestamp.After(mostRecentChange) {
				mostRecentChange = change.Timestamp
			}
		}

		if !mostRecentChange.IsZero() {
			lagSeconds := time.Since(mostRecentChange).Seconds()
			metrics.UpdateChangesLag(lagSeconds)
			span.SetAttributes(attribute.Float64("sync.lag_seconds", lagSeconds))
		}
	}

	// Add final success attributes
	span.SetAttributes(
		attribute.Int("sync.changes_processed", len(result.Changes)),
		attribute.Int64("sync.duration_ms", time.Since(syncStart).Milliseconds()),
	)

	logger.WithFields(logrus.Fields{
		"changes_processed": len(result.Changes),
		"next_token":        result.ContinuationToken,
		"storage_mode":      cfg.Backend.Mode,
		"has_more":          result.HasMore,
		"total_fetched":     result.TotalFetched,
		"sync_duration_ms":  time.Since(syncStart).Milliseconds(),
	}).Info("Successfully processed changes batch")

	return nil
}
