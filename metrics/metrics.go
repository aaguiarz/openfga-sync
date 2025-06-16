package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all the Prometheus metrics for the OpenFGA sync service
type Metrics struct {
	// Change processing metrics
	ChangesProcessedTotal prometheus.Counter
	ChangesErrorsTotal    prometheus.Counter
	ChangesLagSeconds     prometheus.Gauge

	// Sync processing metrics
	SyncDurationSeconds prometheus.Histogram
	SyncLastTimestamp   prometheus.Gauge

	// OpenFGA API metrics
	OpenFGARequestsTotal       prometheus.CounterVec
	OpenFGARequestDuration     prometheus.HistogramVec
	OpenFGALastSuccessfulFetch prometheus.Gauge

	// Storage adapter metrics
	StorageOperationsTotal   prometheus.CounterVec
	StorageOperationDuration prometheus.HistogramVec
	StorageConnectionStatus  prometheus.Gauge

	// Service health metrics
	ServiceUptime         prometheus.Counter
	ServiceStartTimestamp prometheus.Gauge

	mu sync.RWMutex
}

// New creates a new Metrics instance with all prometheus metrics registered
func New() *Metrics {
	return &Metrics{
		// Change processing metrics
		ChangesProcessedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "openfga_sync_changes_processed_total",
			Help: "Total number of changes processed successfully",
		}),
		ChangesErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "openfga_sync_changes_errors_total",
			Help: "Total number of change processing errors",
		}),
		ChangesLagSeconds: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "openfga_sync_changes_lag_seconds",
			Help: "Lag in seconds between the last change timestamp and current time",
		}),

		// Sync processing metrics
		SyncDurationSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "openfga_sync_duration_seconds",
			Help:    "Duration of sync operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		SyncLastTimestamp: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "openfga_sync_last_timestamp",
			Help: "Unix timestamp of the last successful sync",
		}),

		// OpenFGA API metrics
		OpenFGARequestsTotal: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "openfga_sync_openfga_requests_total",
			Help: "Total number of OpenFGA API requests by status",
		}, []string{"status"}),
		OpenFGARequestDuration: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "openfga_sync_openfga_request_duration_seconds",
			Help:    "Duration of OpenFGA API requests in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint"}),
		OpenFGALastSuccessfulFetch: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "openfga_sync_openfga_last_successful_fetch",
			Help: "Unix timestamp of the last successful OpenFGA fetch",
		}),

		// Storage adapter metrics
		StorageOperationsTotal: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "openfga_sync_storage_operations_total",
			Help: "Total number of storage operations by type and status",
		}, []string{"operation", "status"}),
		StorageOperationDuration: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "openfga_sync_storage_operation_duration_seconds",
			Help:    "Duration of storage operations in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"}),
		StorageConnectionStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "openfga_sync_storage_connection_status",
			Help: "Storage connection status (1 = connected, 0 = disconnected)",
		}),

		// Service health metrics
		ServiceUptime: promauto.NewCounter(prometheus.CounterOpts{
			Name: "openfga_sync_service_uptime_seconds_total",
			Help: "Total service uptime in seconds",
		}),
		ServiceStartTimestamp: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "openfga_sync_service_start_timestamp",
			Help: "Unix timestamp when the service started",
		}),
	}
}

// RecordChangesProcessed increments the changes processed counter
func (m *Metrics) RecordChangesProcessed(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChangesProcessedTotal.Add(float64(count))
}

// RecordChangesError increments the changes error counter
func (m *Metrics) RecordChangesError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChangesErrorsTotal.Inc()
}

// UpdateChangesLag updates the changes lag gauge
func (m *Metrics) UpdateChangesLag(lagSeconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChangesLagSeconds.Set(lagSeconds)
}

// RecordSyncDuration records the duration of a sync operation
func (m *Metrics) RecordSyncDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SyncDurationSeconds.Observe(duration.Seconds())
	m.SyncLastTimestamp.Set(float64(time.Now().Unix()))
}

// RecordOpenFGARequest records OpenFGA API request metrics
func (m *Metrics) RecordOpenFGARequest(status string, duration time.Duration, endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OpenFGARequestsTotal.WithLabelValues(status).Inc()
	m.OpenFGARequestDuration.WithLabelValues(endpoint).Observe(duration.Seconds())

	if status == "success" {
		m.OpenFGALastSuccessfulFetch.Set(float64(time.Now().Unix()))
	}
}

// RecordStorageOperation records storage operation metrics
func (m *Metrics) RecordStorageOperation(operation, status string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StorageOperationsTotal.WithLabelValues(operation, status).Inc()
	m.StorageOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// UpdateStorageConnectionStatus updates the storage connection status
func (m *Metrics) UpdateStorageConnectionStatus(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if connected {
		m.StorageConnectionStatus.Set(1)
	} else {
		m.StorageConnectionStatus.Set(0)
	}
}

// RecordServiceStart records when the service started
func (m *Metrics) RecordServiceStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ServiceStartTimestamp.Set(float64(time.Now().Unix()))
}

// IncrementUptime increments the service uptime counter
func (m *Metrics) IncrementUptime() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ServiceUptime.Inc()
}
