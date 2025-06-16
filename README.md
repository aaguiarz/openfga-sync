# OpenFGA Sync Service

A comprehensive Go service that consumes OpenFGA `/changes` API and synchronizes authorization data to databases for auditing, analytics, replication, and compliance purposes.

## Features

### Core Functionality
- **OpenFGA Integration**: Consumes changes API with advanced pagination and continuation token support
- **Multi-Storage Support**: PostgreSQL and SQLite with planned MySQL support
- **Dual Storage Modes**: 
  - **Changelog mode**: Append-only audit trail of all changes
  - **Stateful mode**: Current state representation for efficient queries
- **Configuration Management**: YAML files with environment variable overrides
- **Production Ready**: Graceful shutdown, automatic schema initialization, comprehensive error handling

### Enhanced Fetcher Capabilities
- **Intelligent Parsing**: Automatic user/object type extraction (e.g., "employee:alice" → type="employee", id="alice")
- **Retry Logic**: Exponential backoff with configurable parameters
- **Rate Limiting**: Built-in request throttling to respect API limits
- **Statistics Tracking**: Real-time metrics on requests, latency, and success rates
- **Raw JSON Preservation**: Complete audit trail with original OpenFGA responses
- **Change Validation**: Comprehensive validation of change events
- **Paging Support**: Advanced pagination with configurable page sizes and limits

### Observability & Operations
- **Structured Logging**: JSON/text formats with configurable levels
- **Health Endpoints**: Ready for Kubernetes health checks
- **Metrics**: Prometheus-compatible metrics endpoint
- **OpenTelemetry**: Distributed tracing support (planned)
- **Leader Election**: Kubernetes-native HA support (planned)

## Architecture

The service follows a clean architecture pattern with clear separation of concerns:

### Core Components
- **`main.go`**: Service orchestration, signal handling, and startup logic
- **`config/`**: Configuration management with YAML/environment variable support
- **`fetcher/`**: Enhanced OpenFGA client with retry logic, parsing, and statistics
- **`storage/`**: Database adapters with dual storage modes

### Storage Modes

#### Changelog Mode (`changelog`)
- **Table**: `fga_changelog` 
- **Purpose**: Complete audit trail of all authorization changes
- **Schema**: Stores every change event with timestamps, change types, and raw JSON
- **Use Cases**: Compliance, audit trails, change analysis, debugging

#### Stateful Mode (`stateful`)
- **Table**: `fga_tuples`
- **Purpose**: Current state representation for efficient queries
- **Schema**: Maintains only the current authorization relationships
- **Use Cases**: Authorization queries, data replication, performance optimization

### Change Event Structure

Each change event contains both structured and raw data:

```go
type ChangeEvent struct {
    // Structured fields (parsed from OpenFGA)
    ObjectType string    `json:"object_type"`  // e.g., "document"
    ObjectID   string    `json:"object_id"`    // e.g., "readme.md"
    Relation   string    `json:"relation"`     // e.g., "viewer"
    UserType   string    `json:"user_type"`    // e.g., "employee"
    UserID     string    `json:"user_id"`      // e.g., "alice"
    ChangeType string    `json:"change_type"`  // "tuple_write" or "tuple_delete"
    Timestamp  time.Time `json:"timestamp"`    // When the change occurred
    RawJSON    string    `json:"raw_json"`     // Original OpenFGA response
}
```

### Storage Backends

#### PostgreSQL
- **Production Ready**: Fully tested with transaction support
- **Features**: JSONB storage, advanced indexing, concurrent connections
- **DSN Format**: `postgres://user:password@host:port/database?sslmode=disable`
- **Best For**: Production deployments, high-volume scenarios

```yaml
backend:
  type: "postgres"
  dsn: "postgres://user:password@localhost:5432/openfga_sync?sslmode=disable"
  mode: "changelog"  # or "stateful"
```

#### SQLite
- **Lightweight**: Single-file database, no server required
- **Features**: WAL mode, in-memory support, ACID transactions
- **DSN Format**: File path or `:memory:` for in-memory database
- **Best For**: Development, testing, single-instance deployments

```yaml
backend:
  type: "sqlite"
  dsn: "/var/lib/openfga-sync/data.db"  # or ":memory:"
  mode: "stateful"  # or "changelog"
```

#### MySQL (Coming Soon)
- **Enterprise**: Planned support for MySQL 5.7+ and MariaDB
- **Features**: Replication support, clustering capabilities
- **DSN Format**: `user:password@tcp(host:port)/database?parseTime=true`

#### OpenFGA Replication
- **Use Case**: Replicate changes to another OpenFGA instance
- **Features**: Backup, disaster recovery, multi-region sync
- **DSN Formats**: 
  - Simple: `http://target-endpoint/target-store-id`
  - JSON (advanced): `{"endpoint":"...","store_id":"...","token":"...","batch_size":200}`
- **Best For**: Backup scenarios, development/staging sync, migration

```yaml
backend:
  type: "openfga"
  # Simple format
  dsn: "http://backup-openfga:8080/01BACKUP-STORE-ID"
  mode: "stateful"  # or "changelog"

  # OR JSON format for advanced configuration
  dsn: |
    {
      "endpoint": "https://target-openfga.example.com",
      "store_id": "01HTARGET-STORE-ID",
      "token": "target-api-token",
      "authorization_model_id": "01HMODEL-ID",
      "request_timeout": "45s",
      "max_retries": 5,
      "batch_size": 250
    }
```

## Configuration

The service supports comprehensive configuration through YAML files with environment variable overrides. See [`config.example.yaml`](config.example.yaml) for a complete example.

### Quick Start Configuration

Create a `config.yaml` file:

```yaml
# OpenFGA connection
openfga:
  endpoint: "https://api.openfga.example.com"
  store_id: "01HXXX-YOUR-STORE-ID"
  token: "your-api-token"  # Optional

# Database connection  
backend:
  type: "postgres"
  dsn: "postgres://user:password@localhost:5432/openfga_sync?sslmode=disable"
  mode: "changelog"  # or "stateful"

# Service behavior
service:
  poll_interval: "5s"
  batch_size: 100
  max_retries: 3
  retry_delay: "1s"
  enable_validation: true

# Logging
logging:
  level: "info"
  format: "json"
```

### Advanced Configuration

For production deployments, additional options are available:

```yaml
service:
  # Fetching behavior
  max_changes: 0                    # Limit changes per poll (0 = no limit)
  request_timeout: "30s"            # Timeout for OpenFGA requests
  max_retry_delay: "5s"             # Maximum delay between retries
  backoff_factor: 2.0               # Exponential backoff multiplier
  rate_limit_delay: "50ms"          # Delay between requests

# Observability
observability:
  opentelemetry:
    endpoint: "http://otel-collector:4317"
    service_name: "openfga-sync"
    enabled: true
  metrics:
    enabled: true
    path: "/metrics"

# High availability (Kubernetes)
leadership:
  enabled: true
  namespace: "openfga-system"
  lock_name: "openfga-sync-leader"
```

### Environment Variable Support

All configuration options can be overridden with environment variables:

```bash
export OPENFGA_ENDPOINT="https://api.openfga.example.com"
export OPENFGA_STORE_ID="01HXXX-YOUR-STORE-ID"
export BACKEND_DSN="postgres://user:password@localhost:5432/openfga_sync"
export BACKEND_MODE="changelog"
export LOG_LEVEL="debug"
```

## Usage

### Installation

```bash
# Clone the repository
git clone https://github.com/aaguiarz/openfga-sync.git
cd openfga-sync

# Install dependencies
go mod tidy

# Build the service
go build -o openfga-sync
```

### Running the Service

```bash
# Run with default config (config.yaml)
./openfga-sync

# Run with custom config file
./openfga-sync -config /path/to/config.yaml

# Run with environment variables only
OPENFGA_ENDPOINT="https://api.openfga.example.com" \
OPENFGA_STORE_ID="01HXXX-STORE-ID" \
BACKEND_DSN="postgres://user:pass@localhost/db" \
./openfga-sync
```

### Docker Usage

```bash
# Build Docker image
docker build -t openfga-sync .

# Run with config file
docker run -v $(pwd)/config.yaml:/app/config.yaml openfga-sync

# Run with environment variables
docker run -e OPENFGA_ENDPOINT="https://api.openfga.example.com" \
           -e BACKEND_DSN="postgres://user:pass@db:5432/openfga_sync" \
           openfga-sync
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openfga-sync
spec:
  replicas: 1
  selector:
    matchLabels:
      app: openfga-sync
  template:
    metadata:
      labels:
        app: openfga-sync
    spec:
      containers:
      - name: openfga-sync
        image: openfga-sync:latest
        env:
        - name: OPENFGA_ENDPOINT
          value: "https://api.openfga.example.com"
        - name: OPENFGA_STORE_ID
          value: "01HXXX-STORE-ID"
        - name: BACKEND_DSN
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: dsn
        - name: LEADERSHIP_ENABLED
          value: "true"
```

### HTTP Endpoints & Monitoring

The service exposes HTTP endpoints for health checks and metrics on the configured server port (default: 8080).

#### Health Check Endpoints

**Health Endpoint: `/healthz`**
```bash
curl http://localhost:8080/healthz
```

Response:
```json
{
  "status": "UP",
  "service": "openfga-sync",
  "version": "1.0.0",
  "uptime": "2m30s",
  "details": {
    "backend_type": "postgres",
    "storage_mode": "changelog",
    "poll_interval": "5s"
  }
}
```

**Readiness Endpoint: `/readyz`**
```bash
curl http://localhost:8080/readyz
```

Response:
```json
{
  "status": "READY",
  "service": "openfga-sync",
  "dependencies": {
    "service_ready": "OK"
  }
}
```

#### Prometheus Metrics

**Metrics Endpoint: `/metrics`**
```bash
curl http://localhost:8080/metrics
```

The service exposes comprehensive Prometheus metrics:

**Change Processing Metrics:**
- `openfga_sync_changes_processed_total` - Total changes processed successfully
- `openfga_sync_changes_errors_total` - Total change processing errors  
- `openfga_sync_changes_lag_seconds` - Lag between latest change timestamp and current time

**Sync Operation Metrics:**
- `openfga_sync_duration_seconds` - Histogram of sync operation durations
- `openfga_sync_last_timestamp` - Unix timestamp of last successful sync

**OpenFGA API Metrics:**
- `openfga_sync_openfga_requests_total{status="success|error"}` - API request counts by status
- `openfga_sync_openfga_request_duration_seconds{endpoint="changes"}` - API request duration histogram
- `openfga_sync_openfga_last_successful_fetch` - Unix timestamp of last successful fetch

**Storage Metrics:**
- `openfga_sync_storage_operations_total{operation,status}` - Storage operation counts
- `openfga_sync_storage_operation_duration_seconds{operation}` - Storage operation durations
- `openfga_sync_storage_connection_status` - Storage connection status (1=connected, 0=disconnected)

**Service Health Metrics:**
- `openfga_sync_service_uptime_seconds_total` - Total service uptime
- `openfga_sync_service_start_timestamp` - Service start timestamp

#### Configuration

Enable metrics in your configuration:
```yaml
server:
  port: 8080  # HTTP server port

observability:
  metrics:
    enabled: true     # Enable Prometheus metrics
    path: "/metrics"  # Metrics endpoint path (default: /metrics)
```

#### Monitoring Integration

**Prometheus Configuration:**
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'openfga-sync'
    static_configs:
      - targets: ['openfga-sync:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

**Kubernetes ServiceMonitor:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: openfga-sync
spec:
  selector:
    matchLabels:
      app: openfga-sync
  endpoints:
  - port: http
    path: /metrics
```

### Database Setup

The service automatically creates and manages database schemas based on the storage mode:

#### Changelog Mode Tables
- **`fga_changelog`**: Complete audit trail of authorization changes
  - `id`: Primary key (auto-increment)
  - `object_type`, `object_id`: Parsed object information
  - `relation`: The relationship type
  - `user_type`, `user_id`: Parsed user information  
  - `change_type`: Type of change (tuple_write, tuple_delete)
  - `timestamp`: When the change occurred
  - `raw_json`: Original OpenFGA response for compliance
  - `created_at`: When the record was stored

#### Stateful Mode Tables
- **`fga_tuples`**: Current state representation
  - `object_type`, `object_id`: Object identification
  - `relation`: Relationship type
  - `user_type`, `user_id`: User identification
  - `created_at`, `updated_at`: Timestamps
  - Primary key: Composite of object, relation, and user

#### Common Tables
- **`sync_state`**: Synchronization metadata
  - `id`: Primary key
  - `continuation_token`: Last processed token
  - `last_sync_time`: Timestamp of last successful sync

### Development

#### Prerequisites
- Go 1.21+ 
- Storage backend: PostgreSQL 12+ or SQLite 3.x
- OpenFGA server instance

#### Local Development

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run with verbose testing
go test -v ./...

# Run specific test suites
go test ./config ./fetcher ./storage

# Run demo examples
go run examples/changes_demo.go
go run examples/enhanced_demo/main.go
```

#### Testing

The project includes comprehensive test suites:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./fetcher

# Test configuration parsing
go test ./config -v

# Test fetcher functionality
go test ./fetcher -v
```

### OpenFGA Replication

The OpenFGA storage adapter enables replication from one OpenFGA instance to another, supporting various scenarios such as backup, disaster recovery, and multi-region deployments.

#### Configuration Formats

**Simple DSN Format:**
```yaml
backend:
  type: "openfga"
  dsn: "http://target-openfga:8080/target-store-id"
  mode: "stateful"
```

**JSON DSN Format (Advanced):**
```yaml
backend:
  type: "openfga"
  dsn: |
    {
      "endpoint": "https://target-openfga.example.com",
      "store_id": "01HTARGET-STORE-ID",
      "token": "target-api-token",
      "authorization_model_id": "01HMODEL-ID",
      "request_timeout": "45s",
      "max_retries": 5,
      "retry_delay": "2s",
      "batch_size": 250
    }
  mode: "stateful"
```

#### JSON DSN Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `endpoint` | string | Required | Target OpenFGA API endpoint |
| `store_id` | string | Required | Target store ID |
| `token` | string | Optional | API token for authentication |
| `authorization_model_id` | string | Optional | Specific model ID to use |
| `request_timeout` | string | "30s" | Timeout for API requests |
| `max_retries` | int | 3 | Maximum retry attempts |
| `retry_delay` | string | "1s" | Delay between retries |
| `batch_size` | int | 100 | Number of changes per batch |

#### Use Cases

**1. Backup and Disaster Recovery**
```yaml
# Primary → Backup replication
backend:
  type: "openfga"
  dsn: "https://backup-region.openfga.example.com/01BACKUP-STORE-ID"
  mode: "stateful"  # Maintain current state for quick failover
```

**2. Development/Staging Sync**
```yaml
# Production → Development replication
backend:
  type: "openfga"
  dsn: "http://dev-openfga:8080/01DEV-STORE-ID"
  mode: "changelog"  # Keep full audit trail for testing
```

**3. Cross-Cloud Migration**
```yaml
# Migration with custom settings
backend:
  type: "openfga"
  dsn: |
    {
      "endpoint": "https://new-cloud.openfga.example.com",
      "store_id": "01HNEW-STORE-ID",
      "token": "migration-token",
      "request_timeout": "120s",
      "batch_size": 50
    }
  mode: "changelog"
```

**4. Multi-Region Active Replication**
```yaml
# High-performance replication
backend:
  type: "openfga"
  dsn: |
    {
      "endpoint": "https://us-west.openfga.example.com",
      "store_id": "01HUS-WEST-STORE-ID",
      "batch_size": 500,
      "max_retries": 10
    }
  mode: "stateful"
```

#### Features

- **Dual Mode Support**: Both changelog and stateful modes
- **Retry Logic**: Exponential backoff with configurable parameters
- **Batch Processing**: Configurable batch sizes for optimal performance
- **Authentication**: API token support for secure connections
- **Health Monitoring**: Connection status and statistics
- **Tuple Conversion**: Smart reconstruction of OpenFGA tuple keys
- **Error Handling**: Graceful handling of network and API errors

## Database Schema

### openfga_changes table

- `id`: Primary key
- `user_key`: The user from the tuple
- `relation`: The relation from the tuple
- `object_key`: The object from the tuple
- `operation`: The operation (WRITE, DELETE)
- `timestamp`: When the change occurred
- `change_type`: Type of change (always "tuple_change")
- `created_at`: When the record was inserted

### sync_state table

- `id`: Primary key
- `continuation_token`: Last processed continuation token
- `updated_at`: When the token was last updated

## Performance & Best Practices

### Recommended Settings

#### For High-Volume Environments
```yaml
service:
  batch_size: 500
  poll_interval: "1s"
  max_retries: 5
  rate_limit_delay: "10ms"
  request_timeout: "60s"
  max_retry_delay: "10s"
```

#### For Low-Latency Requirements
```yaml
service:
  batch_size: 50
  poll_interval: "100ms"
  max_retries: 2
  rate_limit_delay: "5ms"
  request_timeout: "5s"
```

### Storage Mode Selection

- **Use Changelog Mode** for:
  - Compliance and audit requirements
  - Change analysis and debugging
  - Complete historical tracking
  - Forensic analysis

- **Use Stateful Mode** for:
  - Performance-critical authorization queries
  - Data replication to other systems
  - Current state analysis
  - Reduced storage requirements

### Database Optimization

#### PostgreSQL Settings
```sql
-- Optimize for changelog mode
CREATE INDEX idx_changelog_timestamp ON fga_changelog(timestamp DESC);
CREATE INDEX idx_changelog_user ON fga_changelog(user_type, user_id);
CREATE INDEX idx_changelog_object ON fga_changelog(object_type, object_id);

-- Optimize for stateful mode
CREATE INDEX idx_tuples_user ON fga_tuples(user_type, user_id);
CREATE INDEX idx_tuples_object ON fga_tuples(object_type, object_id);
```

## Troubleshooting

### Common Issues

#### Connection Problems
```bash
# Test OpenFGA connectivity
curl https://api.openfga.example.com/stores/01HXXX-STORE-ID/changes

# Test database connectivity
psql "postgres://user:pass@localhost:5432/openfga_sync" -c "SELECT 1;"
```

#### High Memory Usage
- Reduce `batch_size` in configuration
- Enable `rate_limit_delay` to slow down processing
- Monitor with `go tool pprof`

#### Missing Changes
- Check continuation token in `sync_state` table
- Verify OpenFGA store ID is correct
- Review logs for parsing errors

### Debug Mode

Enable debug logging for detailed information:

```yaml
logging:
  level: "debug"
  format: "json"
```

## Roadmap

### Planned Features

- **Additional Storage Backends**
  - MySQL support
  - MongoDB support
  - OpenFGA write-back mode

- **High Availability**
  - Kubernetes leader election
  - Multi-region support
  - Auto-failover

- **Enhanced Observability**
  - OpenTelemetry tracing
  - Custom dashboards
  - Alerting rules

- **Performance**
  - Concurrent processing
  - Change deduplication
  - Compression support

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes with tests
4. Run the test suite: `go test ./...`
5. Submit a pull request

### Code Standards

- Follow Go conventions and best practices
- Add tests for new functionality
- Update documentation for user-facing changes
- Use structured logging with appropriate levels

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: See [CONFIGURATION.md](CONFIGURATION.md) for detailed configuration options
- **Examples**: Check the `examples/` directory for usage examples
- **Issues**: Report bugs and feature requests on GitHub Issues
