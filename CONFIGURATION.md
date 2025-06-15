# OpenFGA Sync Service - Configuration Guide

## Overview

This service supports comprehensive configuration through both YAML files and environment variables, following the RFC specification for the OpenFGA Change Streamer Service.

## Configuration Features

### ✅ **Dual Configuration Sources**
- **YAML Configuration**: File-based configuration with defaults
- **Environment Variables**: Override any YAML setting with env vars
- **Precedence**: Environment variables override YAML settings

### ✅ **RFC-Compliant Structure**
- **OpenFGA Configuration**: Endpoint, store ID, authentication token
- **Backend Storage**: Type (postgres/mysql/sqlite), DSN, storage mode
- **Storage Modes**: 
  - `changelog`: Append-only event log
  - `stateful`: Current state representation
- **Service Behavior**: Polling intervals, batch sizes, retry logic
- **Observability**: OpenTelemetry, metrics, structured logging
- **High Availability**: Kubernetes leader election support

### ✅ **Comprehensive Validation**
- Required field validation
- Type validation for enums and durations
- Sensible defaults for all optional settings

## Configuration Examples

### Basic YAML Configuration

```yaml
openfga:
  endpoint: "https://api.openfga.example.com"
  store_id: "01HXXX-EXAMPLE-STORE-ID"
  token: "fga_secret_token_here"

backend:
  type: "postgres"
  dsn: "postgres://user:password@localhost:5432/openfga_sync"
  mode: "changelog"

service:
  poll_interval: "5s"
  batch_size: 100
```

### Environment Variable Configuration

```bash
# OpenFGA Configuration
export OPENFGA_ENDPOINT="https://api.openfga.example.com"
export OPENFGA_STORE_ID="01HXXX-EXAMPLE-STORE-ID"
export OPENFGA_TOKEN="fga_secret_token_here"

# Backend Configuration
export BACKEND_TYPE="postgres"
export BACKEND_DSN="postgres://user:password@localhost:5432/openfga_sync"
export BACKEND_MODE="stateful"

# Service Configuration
export POLL_INTERVAL="10s"
export BATCH_SIZE="50"

# Observability
export OTEL_ENDPOINT="http://otel-collector:4317"
export OTEL_ENABLED="true"
export LOG_LEVEL="debug"
export LOG_FORMAT="json"
```

## Database Schema Support

### Changelog Mode (`backend.mode: "changelog"`)
Creates an append-only table `fga_changelog`:
- Stores all change events with full context
- Includes parsed `user_type`/`user_id` and `object_type`/`object_id`
- Maintains raw event JSON for audit trails

### Stateful Mode (`backend.mode: "stateful"`)
Creates a current state table `fga_tuples`:
- Represents current authorization state
- Supports upsert for WRITE operations
- Supports delete for DELETE operations
- Uses composite primary key for tuple uniqueness

## User/Object Parsing

The service automatically parses OpenFGA user and object strings:
- `user:123` → `user_type: "user"`, `user_id: "123"`
- `document:abc` → `object_type: "document"`, `object_id: "abc"`
- Falls back to defaults if no type prefix found

## Testing

Comprehensive test suite covers:
- ✅ YAML configuration parsing
- ✅ Environment variable overrides
- ✅ Configuration validation
- ✅ Storage mode helper methods

Run tests: `go test ./config -v`

## Usage Examples

### Run with YAML config only
```bash
./openfga-sync -config config.yaml
```

### Run with environment overrides
```bash
BACKEND_MODE=stateful POLL_INTERVAL=30s ./openfga-sync -config config.yaml
```

### Run with environment variables only
```bash
OPENFGA_ENDPOINT=https://api.example.com \
OPENFGA_STORE_ID=store-123 \
BACKEND_TYPE=postgres \
BACKEND_DSN=postgres://user:pass@localhost/db \
BACKEND_MODE=changelog \
./openfga-sync
```

## Configuration Reference

See `config.example.yaml` for a complete configuration reference with all available options and their descriptions.

## Validation

The service validates all configuration at startup:
- **Required fields**: `openfga.endpoint`, `openfga.store_id`, `backend.dsn`
- **Enum validation**: `backend.mode`, `backend.type`, `logging.level`, `logging.format`
- **Duration parsing**: `service.poll_interval`, `service.retry_delay`
- **Numeric validation**: Port numbers, batch sizes, retry counts

Validation errors are reported clearly with specific field names and expected values.
