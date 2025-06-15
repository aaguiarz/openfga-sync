# OpenFGA Sync Service

A Go service that consumes OpenFGA `/changes` API and writes changes to a database for auditing, analytics, or replication purposes.

## Features

- Consumes OpenFGA changes API with continuation token support
- Writes changes to PostgreSQL database
- Configurable via YAML configuration file
- Structured logging with configurable levels
- Graceful shutdown handling
- Automatic database schema initialization

## Architecture

The service is organized into the following components:

- **`main.go`**: Entry point with configuration parsing and service orchestration
- **`config/`**: Configuration management with YAML support
- **`fetcher/`**: OpenFGA API client for fetching changes
- **`storage/`**: Database storage adapters (currently PostgreSQL)

## Configuration

Create a `config.yaml` file:

```yaml
server:
  port: 8080

openfga:
  url: "http://localhost:8080"
  store_id: "your-store-id"
  api_token: "" # Optional: leave empty if not using authentication

database:
  driver: "postgres"
  dsn: "postgres://user:password@localhost:5432/openfga_sync?sslmode=disable"

logging:
  level: "info" # debug, info, warn, error
  format: "text" # text or json
```

## Usage

### Running the service

```bash
# Build the service
go build -o openfga-sync ./main.go

# Run with default config
./openfga-sync

# Run with custom config file
./openfga-sync -config /path/to/config.yaml
```

### Database Setup

The service automatically creates the necessary database tables:

- `openfga_changes`: Stores the change events
- `sync_state`: Stores the continuation token for resuming

### Dependencies

- Go 1.23+
- PostgreSQL database
- OpenFGA server

## Development

### Installing dependencies

```bash
go mod tidy
```

### Running locally

1. Start PostgreSQL database
2. Start OpenFGA server
3. Update `config.yaml` with your settings
4. Run the service: `go run main.go`

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

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 

Absolutely! Here’s an updated RFC where the `user` field is **split into `user_type` and `user_id`** in both the changelog and state tables, and throughout the relevant parts of the specification.

---

# RFC: OpenFGA Change Streamer Service

## Summary

This document specifies a Kubernetes-native, high-availability Go service that continuously consumes changes from an OpenFGA instance via the `/changes` API and writes those changes to a configurable backend store. The service supports two storage modes—changelog (append-only) and stateful (applies mutations)—and is designed for observability, operational ease, and pluggability.

---

## Goals

* Continuously and reliably consume the OpenFGA `/changes` API.
* Write changes to a supported backend store (MySQL, Postgres, SQLite, or another OpenFGA instance).
* Support **two storage modes**:

  * **Changelog**: Store raw change events in an append-only log.
  * **Stateful**: Apply changes to a table so the backend reflects the current state.
* Represent users as `{user_type, user_id}` pairs in all storage.
* Run reliably and scalably in Kubernetes, supporting high availability.
* Provide observability via OpenTelemetry metrics, logs, and traces.
* Expose health and readiness endpoints for operational use.

---

## Non-Goals

* Real-time data transformation beyond simple mutation (e.g., no enrichment).
* Change deduplication across multiple OpenFGA instances.
* External orchestration or pipeline management.

---

## Service Specification

### 1. **Configuration**

The service must be configurable via environment variables or a configuration file, supporting:

* OpenFGA API endpoint and authentication.
* Backend store type and connection parameters.
* Storage mode (`changelog` or `stateful`).
* Polling interval and batch size.
* OpenTelemetry endpoint and configuration.
* Service port for health/metrics endpoints.

### 2. **Fetching Changes**

* Continuously call the OpenFGA `/changes` API with a continuation token, starting from a configured initial token (or from the beginning if not set).
* Handle paging with continuation tokens, fetching changes until no new data is available, then sleep for the polling interval.

### 3. **Storage Modes**

#### 3.1. **Changelog Mode**

* Each change event is written to a dedicated append-only table.
* Table schema must include: change type (`add`/`delete`), object type, object ID, relation, **user type**, **user ID**, timestamp, and raw event (as JSON).

#### 3.2. **Stateful Mode**

* The service applies each change event:

  * For `add`, insert or upsert the tuple into a state table.
  * For `delete`, delete the tuple from the state table.
* The state table represents the current set of tuples as per OpenFGA, with users split into `user_type` and `user_id`.

### 4. **Backend Store Abstraction**

* The service must define a **Store Adapter** interface with methods to handle both changelog and stateful modes.
* Implementations must exist for MySQL, Postgres, SQLite, and OpenFGA (writing changes as API calls if configured).
* Backends must be selected at runtime based on configuration.

### 5. **High Availability and Scalability**

* Service must be stateless and horizontally scalable.
* **Leader election**: By default, only one pod actively consumes and processes changes at a time. If the leader fails, another pod takes over automatically.
* **Future extensibility**: The design should allow sharding if OpenFGA supports partitioned changelogs.

### 6. **Observability**

* **Metrics**: Expose operational metrics (e.g., number of changes processed, errors, lag, etc.) via a Prometheus-compatible `/metrics` endpoint.
* **Tracing**: Integrate with OpenTelemetry for distributed tracing.
* **Logging**: Structured, JSON logs with request and operation context.
* **Health endpoints**: Provide `/healthz` and `/readyz` endpoints.

### 7. **Operational Concerns**

* The service must support graceful shutdown and draining.
* Must provide meaningful error handling, retries with exponential backoff, and alert on repeated failures.
* Configuration reload on SIGHUP is optional.

---

## Table Schemas

### Changelog Table Example

```sql
CREATE TABLE fga_changelog (
    id BIGSERIAL PRIMARY KEY,
    change_type VARCHAR,
    object_type VARCHAR,
    object_id VARCHAR,
    relation VARCHAR,
    user_type VARCHAR,
    user_id VARCHAR,
    timestamp TIMESTAMPTZ,
    raw_event JSONB
);
```

### State Table Example

```sql
CREATE TABLE fga_tuples (
    object_type VARCHAR,
    object_id VARCHAR,
    relation VARCHAR,
    user_type VARCHAR,
    user_id VARCHAR,
    PRIMARY KEY (object_type, object_id, relation, user_type, user_id)
);
```

---

## API Endpoints

* `/metrics` (Prometheus)
* `/healthz` (liveness)
* `/readyz` (readiness)

---

## Out of Scope

* API for manual tuple editing or replay.
* UI/dashboard.

---

## Example Configuration

```yaml
openfga:
  endpoint: https://openfga.example.com
  token: <secret>
backend:
  type: postgres
  dsn: postgres://user:pass@host/db
  mode: changelog
poll_interval: 5s
batch_size: 100
otel_endpoint: http://otel-collector:4317
```

---

## Example Pseudocode (Go)

```go
for {
    changes, token, err := fetchChanges(lastToken)
    for _, c := range changes {
        // c.UserType and c.UserID used instead of c.User
        if config.Mode == "changelog" {
            store.WriteChange(ctx, c)
        } else {
            store.ApplyChange(ctx, c)
        }
    }
    lastToken = token
    sleep(config.PollInterval)
}
```

---

## References

* [OpenFGA API Reference](https://openfga.dev/docs/api)
* [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
* [Kubernetes Leader Election](https://kubernetes.io/docs/tasks/administer-cluster/configure-leader-election-cluster/)
* [Prometheus Metrics](https://prometheus.io/docs/introduction/overview/)

---

## Future Work

* Support for change stream partitioning and sharding when available.
* Support for additional backends (e.g., Kafka, S3).

---

**End of RFC**
