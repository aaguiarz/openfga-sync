# OpenFGA Replication Quick Start Guide

This guide will help you set up OpenFGA-to-OpenFGA replication in minutes.

## Prerequisites

- Two OpenFGA instances (source and target)
- Go 1.21+ installed
- Basic understanding of OpenFGA concepts

## Step 1: Clone and Build

```bash
git clone https://github.com/aaguiarz/openfga-sync.git
cd openfga-sync
go mod tidy
go build -o openfga-sync
```

## Step 2: Basic Configuration

Create a `config.yaml` file:

```yaml
# Source OpenFGA instance (what we're reading FROM)
openfga:
  endpoint: "http://source-openfga:8080"
  store_id: "01HSOURCE-STORE-ID"
  token: "source-api-token"  # Optional

# Target OpenFGA instance (what we're writing TO)
backend:
  type: "openfga"
  dsn: "http://target-openfga:8080/01HTARGET-STORE-ID"
  mode: "stateful"  # Use "changelog" for audit trail

# Basic settings
service:
  poll_interval: "10s"
  batch_size: 100

logging:
  level: "info"
  format: "text"
```

## Step 3: Run the Service

```bash
./openfga-sync -config config.yaml
```

You should see output like:
```
INFO Successfully created OpenFGA storage adapter target_store_id=01HTARGET-STORE-ID
INFO Starting OpenFGA sync service
INFO Polling for changes...
```

## Advanced Configuration (JSON DSN)

For more control, use JSON DSN format:

```yaml
backend:
  type: "openfga"
  dsn: |
    {
      "endpoint": "https://target-openfga.example.com",
      "store_id": "01HTARGET-STORE-ID",
      "token": "target-api-token",
      "authorization_model_id": "01HMODEL-ID",
      "request_timeout": "30s",
      "max_retries": 5,
      "batch_size": 200
    }
  mode: "stateful"
```

## Common Use Cases

### Backup Setup
```yaml
# Replicate to backup instance
backend:
  type: "openfga"
  dsn: "https://backup.openfga.example.com/01BACKUP-STORE-ID"
  mode: "stateful"
```

### Development Sync
```yaml
# Sync production to development
backend:
  type: "openfga"
  dsn: "http://dev-openfga:8080/01DEV-STORE-ID"
  mode: "changelog"  # Keep audit trail
```

## Monitoring

Check the service status:
```bash
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

## Troubleshooting

### Connection Issues
- Verify OpenFGA endpoints are accessible
- Check API tokens have appropriate permissions
- Ensure store IDs exist on both instances

### Performance Tuning
- Increase `batch_size` for higher throughput
- Decrease `poll_interval` for lower latency
- Adjust `max_retries` for unreliable networks

### Debug Mode
```yaml
logging:
  level: "debug"  # Enable detailed logs
```

## Examples

Run the interactive demo to test functionality:
```bash
go run examples/openfga_demo/main.go
```

See `config.openfga-advanced.yaml` for complex scenarios.

## Support

- Read the full documentation in `README.md`
- Check `OPENFGA_IMPLEMENTATION.md` for technical details
- Report issues on GitHub
