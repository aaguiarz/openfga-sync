# Quick Start: HTTP Endpoints & Metrics

## 🚀 Test the New Features

### 1. Start the Service
```bash
./openfga-sync -config config.test.yaml
```

### 2. Test Health Endpoints
```bash
# Health check
curl http://localhost:8080/healthz

# Readiness check  
curl http://localhost:8080/readyz
```

### 3. View Metrics
```bash
# All metrics
curl http://localhost:8080/metrics

# Just OpenFGA sync metrics
curl http://localhost:8080/metrics | grep openfga_sync
```

## 📊 Key Metrics to Monitor

- **`openfga_sync_changes_processed_total`** - Changes successfully processed
- **`openfga_sync_changes_errors_total`** - Processing errors
- **`openfga_sync_changes_lag_seconds`** - Data freshness lag
- **`openfga_sync_storage_connection_status`** - Storage health (1=healthy, 0=down)
- **`openfga_sync_service_uptime_seconds_total`** - Service uptime

## 🔧 Configuration

Add to your `config.yaml`:
```yaml
server:
  port: 8080

observability:
  metrics:
    enabled: true
    path: "/metrics"
```

## 📋 Files Added/Modified

### New Files:
- `metrics/metrics.go` - Prometheus metrics collection
- `server/server.go` - HTTP endpoints server
- `HTTP_METRICS_IMPLEMENTATION.md` - Complete documentation
- `test_comprehensive.sh` - Endpoint validation script

### Modified Files:
- `main.go` - HTTP server integration & metrics tracking
- `storage/adapter.go` - Added GetStats() interface method
- `storage/postgres.go` - Implemented GetStats() method
- `go.mod` - Added Prometheus client dependency
- `README.md` - Added endpoints & metrics documentation
- `CHANGELOG.md` - Version 1.2.0 release notes
