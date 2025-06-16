# OpenFGA Sync Service

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](Dockerfile)

A production-ready Go service that synchronizes OpenFGA authorization data to various storage backends for auditing, analytics, replication, and compliance purposes.

## 🚀 Quick Start

```bash
# Clone and build
git clone https://github.com/aaguiarz/openfga-sync.git
cd openfga-sync
go build -o openfga-sync

# Run with environment variables
OPENFGA_ENDPOINT="https://api.fga.example.com" \
OPENFGA_STORE_ID="01HXXX-STORE-ID" \
BACKEND_DSN="postgres://user:pass@localhost/db" \
./openfga-sync
```

## 📋 Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [Storage Backends](#storage-backends)
- [Usage Examples](#usage-examples)
- [Monitoring & Observability](#monitoring--observability)
- [Deployment](#deployment)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

## ✨ Features

### Core Capabilities
- **🔄 Real-time Sync**: Consumes OpenFGA `/changes` API with intelligent pagination
- **📊 Multi-Storage**: PostgreSQL, SQLite, and OpenFGA replication support
- **🎯 Dual Modes**: Changelog (audit trail) and Stateful (current state) storage
- **🔧 Configuration**: YAML files with comprehensive environment variable overrides
- **🛡️ Production Ready**: Graceful shutdown, health checks, comprehensive error handling

### Advanced Fetcher
- **🧠 Smart Parsing**: Automatic user/object type extraction (`employee:alice` → `type=employee, id=alice`)
- **🔄 Retry Logic**: Exponential backoff with configurable parameters
- **⚡ Rate Limiting**: Built-in throttling to respect API limits
- **📈 Statistics**: Real-time metrics on requests, latency, and success rates
- **📝 Audit Trail**: Complete preservation of original OpenFGA responses
- **✅ Validation**: Comprehensive change event validation

### Observability & Operations
- **📊 Metrics**: Prometheus-compatible metrics with 20+ indicators
- **🔍 Tracing**: OpenTelemetry distributed tracing with OTLP export
- **📋 Logging**: Structured JSON/text logging with configurable levels
- **💚 Health Checks**: Kubernetes-ready health and readiness endpoints
- **⚖️ Load Balancing**: Leader election support for HA deployments

## 🏗️ Architecture

The service follows a clean, modular architecture with clear separation of concerns:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   OpenFGA API   │───▷│  Fetcher Module  │───▷│ Storage Adapter │
│   /changes      │    │  - Parsing       │    │  - PostgreSQL   │
│   /stores       │    │  - Retry Logic   │    │  - SQLite       │
└─────────────────┘    │  - Rate Limiting │    │  - OpenFGA      │
                       └──────────────────┘    └─────────────────┘
                                │
                       ┌──────────────────┐
                       │  Observability   │
                       │  - Metrics       │
                       │  - Tracing       │
                       │  - Health        │
                       └──────────────────┘
```

### Core Components

| Component | Purpose | Features |
|-----------|---------|----------|
| **`main.go`** | Service orchestration | Signal handling, graceful shutdown, startup coordination |
| **`config/`** | Configuration management | YAML parsing, environment variables, validation |
| **`fetcher/`** | OpenFGA client | Retry logic, parsing, statistics, rate limiting |
| **`storage/`** | Database adapters | Multi-backend support, schema management, transactions |
| **`telemetry/`** | Observability | OpenTelemetry tracing, metrics collection |
| **`server/`** | HTTP endpoints | Health checks, metrics exposure, admin interface |

### Storage Modes

#### 📝 Changelog Mode
- **Table**: `fga_changelog`
- **Purpose**: Complete audit trail of all authorization changes
- **Use Cases**: Compliance, forensics, change analysis, debugging
- **Schema**: Stores every change event with timestamps and raw JSON

#### 🎯 Stateful Mode  
- **Table**: `fga_tuples`
- **Purpose**: Current state representation for efficient queries
- **Use Cases**: Authorization queries, replication, performance optimization
- **Schema**: Maintains only current authorization relationships

### Change Event Structure

```go
type ChangeEvent struct {
    // Parsed structured data
    ObjectType string    `json:"object_type"`  // e.g., "document"
    ObjectID   string    `json:"object_id"`    // e.g., "readme.md"
    Relation   string    `json:"relation"`     // e.g., "viewer"
    UserType   string    `json:"user_type"`    // e.g., "employee" 
    UserID     string    `json:"user_id"`      // e.g., "alice"
    ChangeType string    `json:"change_type"`  // "tuple_write" or "tuple_delete"
    Timestamp  time.Time `json:"timestamp"`    // Change occurrence time
    
    // Audit and compliance
    RawJSON    string    `json:"raw_json"`     // Original OpenFGA response
}
```

## 🗄️ Storage Backends

### PostgreSQL
**Production-grade relational database**

```yaml
backend:
  type: "postgres"
  dsn: "postgres://user:password@localhost:5432/openfga_sync?sslmode=disable"
  mode: "changelog"  # or "stateful"
```

**Features:**
- ✅ JSONB storage for complex queries
- ✅ Advanced indexing and performance optimization
- ✅ Concurrent connections and transactions
- ✅ Full ACID compliance
- ✅ Comprehensive test coverage

**Best for:** Production deployments, high-volume scenarios, enterprise environments

---

### SQLite
**Lightweight embedded database**

```yaml
backend:
  type: "sqlite"
  dsn: "/var/lib/openfga-sync/data.db"  # or ":memory:"
  mode: "stateful"  # or "changelog"
```

**Features:**
- ✅ Single-file database, no server required
- ✅ WAL mode for better concurrency
- ✅ In-memory support for testing
- ✅ ACID transactions
- ✅ Cross-platform compatibility

**Best for:** Development, testing, single-instance deployments, edge computing

---

### OpenFGA Replication
**Replicate to another OpenFGA instance**

**Simple Format:**
```yaml
backend:
  type: "openfga"
  dsn: "http://backup-openfga:8080/01BACKUP-STORE-ID"
  mode: "stateful"
```

**Advanced JSON Format:**
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
      "batch_size": 250
    }
  mode: "changelog"
```

**Features:**
- ✅ Backup and disaster recovery
- ✅ Multi-region synchronization
- ✅ Development/staging sync
- ✅ Cross-cloud migration support
- ✅ Configurable batch processing

**Best for:** Backup scenarios, multi-environment sync, migration projects

---

### 🔮 Coming Soon

#### MySQL/MariaDB
- Enterprise-grade relational database
- Replication and clustering support
- DSN: `user:password@tcp(host:port)/database?parseTime=true`

## 🚀 Installation

### Prerequisites
- **Go 1.23+** for building from source
- **Storage Backend**: PostgreSQL 12+ or SQLite 3.x
- **OpenFGA Server**: Access to OpenFGA API instance

### Option 1: Download Binary
```bash
# Download latest release (replace with actual release URL)
curl -L -o openfga-sync https://github.com/aaguiarz/openfga-sync/releases/latest/download/openfga-sync-linux-amd64
chmod +x openfga-sync
```

### Option 2: Build from Source
```bash
# Clone repository
git clone https://github.com/aaguiarz/openfga-sync.git
cd openfga-sync

# Install dependencies
go mod tidy

# Build binary
go build -o openfga-sync

# Optional: Install globally
go install
```

### Option 3: Docker
```bash
# Pull from registry (when available)
docker pull ghcr.io/aaguiarz/openfga-sync:latest

# Or build locally
docker build -t openfga-sync .
```

### Option 4: Docker Compose
```bash
# Quick start with PostgreSQL
docker compose up -d

# View logs
docker compose logs -f openfga-sync
```

## ⚙️ Configuration

The service supports comprehensive configuration through YAML files with environment variable overrides.

### Quick Start Configuration

Create a `config.yaml` file:

```yaml
# OpenFGA connection
openfga:
  endpoint: "https://api.fga.example.com"
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

See [`config.example.yaml`](config.example.yaml) for complete options:

```yaml
# Server configuration
server:
  port: 8080

# Advanced service settings
service:
  poll_interval: "5s"              # How often to poll for changes
  batch_size: 100                  # Changes per batch
  max_changes: 0                   # Limit total changes (0 = unlimited)
  request_timeout: "30s"           # OpenFGA request timeout
  max_retries: 3                   # Retry attempts on failure
  retry_delay: "1s"                # Initial retry delay
  max_retry_delay: "5s"            # Maximum retry delay
  backoff_factor: 2.0              # Exponential backoff multiplier
  rate_limit_delay: "50ms"         # Inter-request delay
  enable_validation: true          # Validate change events

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

### Environment Variables

All configuration options support environment variable overrides:

```bash
# Core settings
export OPENFGA_ENDPOINT="https://api.fga.example.com"
export OPENFGA_STORE_ID="01HXXX-YOUR-STORE-ID"
export OPENFGA_TOKEN="your-api-token"

# Backend configuration
export BACKEND_TYPE="postgres"
export BACKEND_DSN="postgres://user:password@localhost:5432/openfga_sync"
export BACKEND_MODE="changelog"

# Service settings
export POLL_INTERVAL="5s"
export BATCH_SIZE="100"
export LOG_LEVEL="info"

# Observability
export OTEL_ENDPOINT="http://otel-collector:4317"
export OTEL_ENABLED="true"
export METRICS_ENABLED="true"
```

### 🔐 OIDC Authentication

The service supports OIDC authentication using the OAuth 2.0 client credentials flow, particularly useful for Auth0 FGA and other OIDC-enabled OpenFGA instances:

#### YAML Configuration
```yaml
openfga:
  endpoint: "https://api.us1.fga.dev"
  store_id: "01HAUTH0-FGA-STORE-ID"
  
  # OIDC configuration (alternative to token)
  oidc:
    issuer: "https://your-company.auth0.com/"
    audience: "https://api.us1.fga.dev/"
    client_id: "your-m2m-client-id"
    client_secret: "your-m2m-client-secret"
    scopes: ["read:tuples", "write:tuples"]
    token_issuer: "https://your-company.auth0.com/"
```

#### Environment Variables
```bash
export OPENFGA_ENDPOINT="https://api.us1.fga.dev"
export OPENFGA_STORE_ID="01HAUTH0-FGA-STORE-ID"
export OPENFGA_OIDC_ISSUER="https://your-company.auth0.com/"
export OPENFGA_OIDC_AUDIENCE="https://api.us1.fga.dev/"
export OPENFGA_OIDC_CLIENT_ID="your-m2m-client-id"
export OPENFGA_OIDC_CLIENT_SECRET="your-m2m-client-secret"
export OPENFGA_OIDC_SCOPES="read:tuples,write:tuples"
```

#### Authentication Priority
- **OIDC**: Used if both `client_id` and `client_secret` are provided
- **API Token**: Used if `token` is provided and no OIDC configuration
- **Error**: Configuration validation fails if both are provided

> 📖 **Detailed Setup Guide**: See [OIDC_AUTHENTICATION.md](OIDC_AUTHENTICATION.md) for complete Auth0 FGA setup instructions.

```bash

### Configuration Validation

The service validates configuration on startup:

```bash
# Test configuration
./openfga-sync -config config.yaml -validate

# Test with environment variables
BACKEND_DSN="invalid-dsn" ./openfga-sync -validate
```

## 📚 Usage Examples

### Basic Usage

```bash
# Run with default config (config.yaml)
./openfga-sync

# Run with custom config file
./openfga-sync -config /path/to/config.yaml

# Run with environment variables only
OPENFGA_ENDPOINT="https://api.fga.example.com" \
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
docker run -e OPENFGA_ENDPOINT="https://api.fga.example.com" \
           -e BACKEND_DSN="postgres://user:pass@db:5432/openfga_sync" \
           openfga-sync
```

### Configuration Examples

#### SQLite for Development
```yaml
openfga:
  endpoint: "http://localhost:8080"
  store_id: "01HDEV-STORE-ID"

backend:
  type: "sqlite"
  dsn: "./dev-data.db"
  mode: "stateful"

service:
  poll_interval: "2s"
  batch_size: 50
  
logging:
  level: "debug"
  format: "text"
```

#### PostgreSQL for Production
```yaml
openfga:
  endpoint: "https://api.fga.example.com"
  store_id: "01HPROD-STORE-ID"
  token: "${OPENFGA_TOKEN}"

backend:
  type: "postgres"
  dsn: "${DATABASE_URL}"
  mode: "changelog"

service:
  poll_interval: "5s"
  batch_size: 500
  max_retries: 5
  rate_limit_delay: "10ms"
  
observability:
  opentelemetry:
    enabled: true
    endpoint: "http://otel-collector:4317"
  metrics:
    enabled: true

leadership:
  enabled: true
  namespace: "production"
```

#### OpenFGA Replication
```yaml
openfga:
  endpoint: "https://source.fga.example.com"
  store_id: "01HSOURCE-STORE-ID"
  token: "${SOURCE_TOKEN}"

backend:
  type: "openfga"
  dsn: |
    {
      "endpoint": "https://backup.fga.example.com",
      "store_id": "01HBACKUP-STORE-ID",
      "token": "${BACKUP_TOKEN}",
      "batch_size": 100
    }
  mode: "stateful"
```

## Monitoring & Observability

The service provides extensive monitoring and observability features:

### Metrics

Prometheus-compatible metrics are exposed at `/metrics`:

- **Change Processing Metrics:**
  - `openfga_sync_changes_processed_total`: Total changes processed successfully
  - `openfga_sync_changes_errors_total`: Total change processing errors  
  - `openfga_sync_changes_lag_seconds`: Lag between latest change timestamp and current time

- **Sync Operation Metrics:**
  - `openfga_sync_duration_seconds`: Histogram of sync operation durations
  - `openfga_sync_last_timestamp`: Unix timestamp of last successful sync

- **OpenFGA API Metrics:**
  - `openfga_sync_openfga_requests_total{status="success|error"}`: API request counts by status
  - `openfga_sync_openfga_request_duration_seconds{endpoint="changes"}`: API request duration histogram
  - `openfga_sync_openfga_last_successful_fetch`: Unix timestamp of last successful fetch

- **Storage Metrics:**
  - `openfga_sync_storage_operations_total{operation,status}`: Storage operation counts
  - `openfga_sync_storage_operation_duration_seconds{operation}`: Storage operation durations
  - `openfga_sync_storage_connection_status`: Storage connection status (1=connected, 0=disconnected)

- **Service Health Metrics:**
  - `openfga_sync_service_uptime_seconds_total`: Total service uptime
  - `openfga_sync_service_start_timestamp`: Service start timestamp

### Health Endpoints

#### `/healthz` - Health Check
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

#### `/readyz` - Readiness Check
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

#### `/metrics` - Prometheus Metrics
```bash
curl http://localhost:8080/metrics
```

### OpenTelemetry Integration

Enable distributed tracing and metrics export:

```yaml
observability:
  opentelemetry:
    endpoint: "http://otel-collector:4317"
    service_name: "openfga-sync"
    enabled: true
```

**Features:**
- 🔍 **Distributed Tracing**: Complete request traces across OpenFGA API and storage operations
- 📊 **Metrics Export**: OTLP HTTP exporter for metrics to observability platforms
- 🏷️ **Rich Attributes**: Detailed span attributes for debugging and analysis
- 🔄 **Context Propagation**: Proper trace context handling across service boundaries

**Supported Platforms:**
- Jaeger
- Zipkin  
- New Relic
- Datadog
- Grafana Cloud
- Any OTLP-compatible platform

See [`OPENTELEMETRY.md`](OPENTELEMETRY.md) for detailed configuration and examples.

### Monitoring Integration

#### Prometheus Configuration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'openfga-sync'
    static_configs:
      - targets: ['openfga-sync:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

#### Kubernetes ServiceMonitor
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

#### Grafana Dashboard
Import the provided dashboard JSON or create custom dashboards using the available metrics.

## 🚀 Deployment

### Docker

#### Single Container
```bash
# Build
docker build -t openfga-sync .

# Run with environment variables
docker run -d \
  --name openfga-sync \
  -p 8080:8080 \
  -e OPENFGA_ENDPOINT="https://api.fga.example.com" \
  -e OPENFGA_STORE_ID="01HXXX-STORE-ID" \
  -e BACKEND_DSN="postgres://user:pass@db:5432/openfga_sync" \
  openfga-sync
```

#### Docker Compose
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: openfga_sync
      POSTGRES_USER: openfga_user
      POSTGRES_PASSWORD: openfga_password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  openfga-sync:
    build: .
    environment:
      OPENFGA_ENDPOINT: "${OPENFGA_ENDPOINT}"
      OPENFGA_STORE_ID: "${OPENFGA_STORE_ID}"
      BACKEND_DSN: "postgres://openfga_user:openfga_password@postgres:5432/openfga_sync?sslmode=disable"
      BACKEND_MODE: "changelog"
    ports:
      - "8080:8080"
    depends_on:
      - postgres

volumes:
  postgres_data:
```

### Kubernetes

#### Basic Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openfga-sync
  labels:
    app: openfga-sync
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
        ports:
        - containerPort: 8080
        env:
        - name: OPENFGA_ENDPOINT
          value: "https://api.fga.example.com"
        - name: OPENFGA_STORE_ID
          value: "01HXXX-STORE-ID"
        - name: BACKEND_DSN
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: dsn
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: openfga-sync
  labels:
    app: openfga-sync
spec:
  selector:
    app: openfga-sync
  ports:
  - name: http
    port: 8080
    targetPort: 8080
```

#### High Availability Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openfga-sync
spec:
  replicas: 3
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
        - name: LEADERSHIP_ENABLED
          value: "true"
        - name: LEADERSHIP_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # ...other environment variables...
      serviceAccountName: openfga-sync
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: openfga-sync
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: openfga-sync-leader-election
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: openfga-sync-leader-election
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: openfga-sync-leader-election
subjects:
- kind: ServiceAccount
  name: openfga-sync
```

### Helm Chart

A Helm chart is available for easier Kubernetes deployments:

```bash
# Add the helm repository
helm repo add openfga-sync https://aaguiarz.github.io/openfga-sync

# Install with custom values
helm install openfga-sync openfga-sync/openfga-sync \
  --set openfga.endpoint="https://api.fga.example.com" \
  --set openfga.storeId="01HXXX-STORE-ID" \
  --set backend.type="postgres" \
  --set backend.dsn="postgres://user:pass@postgres:5432/openfga_sync"
```

### Cloud Platforms

#### AWS ECS
```json
{
  "family": "openfga-sync",
  "taskRoleArn": "arn:aws:iam::123456789012:role/ecsTaskRole",
  "containerDefinitions": [
    {
      "name": "openfga-sync",
      "image": "openfga-sync:latest",
      "memory": 512,
      "cpu": 256,
      "environment": [
        {"name": "OPENFGA_ENDPOINT", "value": "https://api.fga.example.com"},
        {"name": "OPENFGA_STORE_ID", "value": "01HXXX-STORE-ID"}
      ],
      "portMappings": [
        {"containerPort": 8080, "protocol": "tcp"}
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/openfga-sync",
          "awslogs-region": "us-east-1"
        }
      }
    }
  ]
}
```

#### Google Cloud Run
```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: openfga-sync
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: "1"
        autoscaling.knative.dev/maxScale: "10"
    spec:
      containers:
      - image: gcr.io/PROJECT-ID/openfga-sync:latest
        ports:
        - containerPort: 8080
        env:
        - name: OPENFGA_ENDPOINT
          value: "https://api.fga.example.com"
        - name: OPENFGA_STORE_ID
          value: "01HXXX-STORE-ID"
        resources:
          limits:
            memory: 512Mi
            cpu: 500m
```

## 🛠️ Development

### Prerequisites
- **Go 1.23+** 
- **Storage Backend**: PostgreSQL 12+ or SQLite 3.x
- **OpenFGA Server**: Local or remote instance for testing

### Local Development Setup

```bash
# Clone repository
git clone https://github.com/aaguiarz/openfga-sync.git
cd openfga-sync

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run with verbose testing
go test -v ./...

# Run specific test suites
go test ./config ./fetcher ./storage

# Build and run locally
go build -o openfga-sync
./openfga-sync -config config.example.yaml
```

### Testing

The project includes comprehensive test coverage:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./storage
go test -bench=. ./fetcher

# Test specific packages
go test ./config -v
go test ./storage -v -run TestStorageAdapter
```

### Running Examples

```bash
# Basic OpenFGA changes demo
go run examples/changes_demo/main.go

# Enhanced fetcher demo
go run examples/enhanced_demo/main.go

# SQLite demo
go run examples/sqlite_demo/main.go

# OpenFGA replication demo
go run examples/openfga_demo/main.go
```

### Code Organization

```
.
├── main.go                 # Service entry point
├── config/                 # Configuration management
│   ├── config.go          # Configuration structures and parsing
│   └── config_test.go     # Configuration tests
├── fetcher/                # OpenFGA client
│   ├── openfga.go         # Main fetcher implementation
│   └── openfga_test.go    # Fetcher tests
├── storage/                # Storage adapters
│   ├── adapter.go         # Storage interface
│   ├── postgres.go        # PostgreSQL implementation
│   ├── sqlite.go          # SQLite implementation
│   ├── openfga.go         # OpenFGA replication
│   └── *_test.go          # Comprehensive test suite
├── telemetry/              # Observability
│   └── telemetry.go       # OpenTelemetry setup
├── server/                 # HTTP server
│   └── server.go          # Health checks and metrics
├── metrics/                # Prometheus metrics
│   └── metrics.go         # Metrics definitions
└── examples/               # Usage examples
    ├── changes_demo/       # Basic usage
    ├── enhanced_demo/      # Advanced features
    └── sqlite_demo/        # SQLite specific
```

### Contributing

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature-name`
3. **Make** changes with comprehensive tests
4. **Run** the test suite: `go test ./...`
5. **Update** documentation for user-facing changes
6. **Submit** a pull request

#### Code Standards

- Follow Go conventions and `gofmt` formatting
- Add comprehensive tests for new functionality
- Update documentation for user-facing changes
- Use structured logging with appropriate levels
- Follow the existing error handling patterns
- Add OpenTelemetry tracing for new operations

#### Running Quality Checks

```bash
# Format code
go fmt ./...

# Run linter (install golangci-lint first)
golangci-lint run

# Check for security issues (install gosec first)
gosec ./...

# Check dependencies for vulnerabilities
go list -json -deps ./... | nancy sleuth
```

## 🐛 Troubleshooting

### Common Issues

#### 🔌 Connection Problems

**OpenFGA Connectivity:**
```bash
# Test OpenFGA API access
curl -v "https://api.fga.example.com/stores/01HXXX-STORE-ID/changes"

# Check store access with authentication
curl -H "Authorization: Bearer YOUR-TOKEN" \
     "https://api.fga.example.com/stores/01HXXX-STORE-ID/changes"
```

**Database Connectivity:**
```bash
# Test PostgreSQL connection
psql "postgres://user:pass@localhost:5432/openfga_sync" -c "SELECT 1;"

# Test SQLite file access
sqlite3 /path/to/data.db "SELECT 1;"
```

#### 📈 Performance Issues

**High Memory Usage:**
- Reduce `batch_size` in configuration (try 50-100)
- Enable `rate_limit_delay` to slow down processing
- Monitor with: `go tool pprof http://localhost:8080/debug/pprof/heap`

**High CPU Usage:**
- Increase `poll_interval` to reduce polling frequency
- Reduce `batch_size` for smaller processing chunks
- Check for infinite retry loops in logs

**Slow Processing:**
- Increase `batch_size` for better throughput
- Reduce `rate_limit_delay` if API allows
- Check database performance and indexing

#### 📝 Data Issues

**Missing Changes:**
```bash
# Check continuation token in sync_state table
psql "postgres://..." -c "SELECT * FROM sync_state;"

# Verify OpenFGA store ID is correct
curl "https://api.fga.example.com/stores" | jq '.stores[].id'

# Review logs for parsing errors
./openfga-sync 2>&1 | grep -i error
```

**Duplicate Changes:**
- Check for multiple service instances running
- Verify leader election is enabled in Kubernetes
- Check sync_state table for token corruption

**Schema Issues:**
```bash
# Check table existence
psql "postgres://..." -c "\dt"

# Recreate tables (will lose data)
psql "postgres://..." -c "DROP TABLE IF EXISTS fga_changelog, fga_tuples, sync_state;"
# Restart service to recreate tables
```

#### 🔧 Configuration Issues

**Invalid Configuration:**
```bash
# Validate configuration file
./openfga-sync -config config.yaml -validate

# Test with minimal configuration
echo "openfga:
  endpoint: http://localhost:8080
  store_id: test
backend:
  type: sqlite
  dsn: ':memory:'
  mode: stateful" > test-config.yaml

./openfga-sync -config test-config.yaml
```

**Environment Variable Issues:**
```bash
# List all environment variables
env | grep -i openfga
env | grep -i backend

# Test environment variable parsing
OPENFGA_ENDPOINT="http://test" \
BACKEND_DSN="sqlite://:memory:" \
./openfga-sync -validate
```

### Debug Mode

Enable detailed logging for troubleshooting:

```yaml
logging:
  level: "debug"
  format: "json"  # or "text" for easier reading
```

```bash
# Run with debug logging
LOG_LEVEL=debug ./openfga-sync

# Filter specific components
./openfga-sync 2>&1 | grep -i "fetcher\|storage"
```

### Getting Help

1. **Check Logs**: Enable debug logging and review output
2. **Validate Config**: Use `-validate` flag to check configuration
3. **Test Components**: Use examples to test individual components
4. **Review Documentation**: Check specific documentation files:
   - [`CONFIGURATION.md`](CONFIGURATION.md) - Detailed configuration options
   - [`OPENTELEMETRY.md`](OPENTELEMETRY.md) - OpenTelemetry setup
   - [`QUICKSTART.md`](QUICKSTART.md) - Quick start guide
5. **Report Issues**: Create GitHub issues with:
   - Configuration (sanitized)
   - Logs (with sensitive data removed)
   - Environment details
   - Expected vs actual behavior

## 📚 Documentation

- **[Configuration Guide](CONFIGURATION.md)** - Detailed configuration options
- **[OpenTelemetry Setup](OPENTELEMETRY.md)** - Observability configuration
- **[Quick Start Guide](QUICKSTART.md)** - Get started quickly
- **[OpenFGA Implementation](OPENFGA_IMPLEMENTATION.md)** - OpenFGA specifics
- **[SQLite Implementation](SQLITE_IMPLEMENTATION.md)** - SQLite specifics
- **[HTTP Metrics](HTTP_METRICS_IMPLEMENTATION.md)** - Metrics details

## 🗺️ Roadmap

### 📅 Planned Features

#### Storage Backends
- **MySQL/MariaDB Support** - Enterprise-grade relational database
- **MongoDB Support** - Document-based storage for flexible schemas
- **Redis Support** - High-performance caching and pub/sub
- **ClickHouse Support** - Analytics and time-series data

#### High Availability
- **✅ Leader Election** - Kubernetes-native HA (implemented)
- **Multi-Region Support** - Cross-region replication and failover
- **Auto-Failover** - Automatic failover between instances
- **Load Balancing** - Request distribution across instances

#### Enhanced Observability
- **✅ OpenTelemetry Tracing** - Distributed tracing (implemented)
- **Custom Dashboards** - Pre-built Grafana dashboards
- **Alerting Rules** - Prometheus alerting configurations
- **Log Aggregation** - Structured logging with correlation IDs

#### Performance & Scalability
- **Concurrent Processing** - Parallel change processing
- **Change Deduplication** - Intelligent duplicate detection
- **Compression Support** - Reduce storage and network overhead
- **Streaming API** - Real-time change streaming

#### Developer Experience
- **Helm Charts** - Kubernetes deployment made easy
- **Terraform Modules** - Infrastructure as code
- **VS Code Extension** - Configuration and debugging support
- **CLI Tools** - Management and troubleshooting utilities

### 🚀 Contributing to Roadmap

We welcome contributions and feature requests! Please:

1. **Open an Issue** to discuss new features
2. **Review the Codebase** to understand current architecture
3. **Submit PRs** with comprehensive tests and documentation
4. **Join Discussions** about future direction

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

### Getting Help

- **📖 Documentation**: Start with this README and linked documentation
- **🐛 Bug Reports**: [Create an issue](https://github.com/aaguiarz/openfga-sync/issues/new) with detailed information
- **💡 Feature Requests**: [Open a discussion](https://github.com/aaguiarz/openfga-sync/discussions) about new features
- **❓ Questions**: Use [GitHub Discussions](https://github.com/aaguiarz/openfga-sync/discussions) for general questions

### Support Channels

- **GitHub Issues**: Bug reports and technical issues
- **GitHub Discussions**: General questions and feature discussions
- **Documentation**: Comprehensive guides and examples
- **Examples**: Working code examples in the `examples/` directory

### Commercial Support

For commercial support, consulting, or custom feature development, please contact the maintainers.

---

**Made with ❤️ for the OpenFGA community**
