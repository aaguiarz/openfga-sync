# OpenTelemetry Integration

This document describes the OpenTelemetry tracing and metrics integration in OpenFGA Sync.

## Overview

OpenFGA Sync includes comprehensive OpenTelemetry instrumentation to provide observability into:

- **OpenFGA API calls** - Tracing fetch operations with timing and error details
- **Database operations** - Instrumentation for SQLite, PostgreSQL, and other storage backends
- **Sync operations** - End-to-end tracing of the synchronization process
- **Metrics export** - Integration with OTLP metrics exporters

## Configuration

Enable OpenTelemetry in your configuration file:

```yaml
observability:
  opentelemetry:
    endpoint: "http://localhost:4318"  # OTLP HTTP endpoint
    service_name: "openfga-sync"       # Service name for traces
    enabled: true                      # Enable OpenTelemetry
  metrics:
    enabled: true                      # Enable Prometheus metrics
    path: "/metrics"                   # Metrics endpoint
```

## Trace Spans

### OpenFGA Fetcher Spans

- **`openfga.fetch_changes`** - Traces OpenFGA ReadChanges API calls
  - Attributes: `openfga.store_id`, `openfga.continuation_token`, `openfga.page_size`
  - Success metrics: `openfga.changes_count`, `openfga.next_token`, `openfga.has_more`

### Storage Spans

- **`sqlite.write_changes`** - SQLite changelog operations
- **`sqlite.apply_changes`** - SQLite stateful operations  
- **`postgres.write_changes`** - PostgreSQL changelog operations
  - Attributes: `db.changes_count`, `db.storage_mode`, `db.system`
  - Success metrics: `db.rows_affected`, `db.operation`

### Sync Process Spans

- **`sync.changes`** - End-to-end sync operation
  - Attributes: `sync.continuation_token`, `sync.storage_mode`, `sync.batch_size`
  - Metrics: `sync.changes_found`, `sync.changes_processed`, `sync.duration_ms`

## Quick Start with Jaeger

1. **Start Jaeger (includes OTLP receiver):**
   ```bash
   docker run -d --name jaeger \
     -p 16686:16686 \
     -p 4317:4317 \
     -p 4318:4318 \
     jaegertracing/all-in-one:latest
   ```

2. **Run OpenFGA Sync with tracing:**
   ```bash
   ./openfga-sync -config config.otel.yaml
   ```

3. **View traces at:** http://localhost:16686

## Demo Script

Use the included demo script to see OpenTelemetry in action:

```bash
./demo-otel.sh
```

## Trace Attributes

### Common Attributes

- `service.name` - "openfga-sync"
- `service.version` - "1.0.0"

### OpenFGA Attributes

- `openfga.store_id` - OpenFGA store identifier
- `openfga.continuation_token` - Pagination token
- `openfga.changes_count` - Number of changes fetched
- `openfga.has_more` - Whether more changes are available

### Database Attributes

- `db.system` - Database type (sqlite, postgresql)
- `db.storage_mode` - Storage mode (changelog, stateful)
- `db.changes_count` - Number of changes processed
- `db.operation` - Operation type (insert, upsert, delete)

### Sync Attributes

- `sync.storage_type` - Backend storage type
- `sync.batch_size` - Configured batch size
- `sync.lag_seconds` - Processing lag in seconds

## Error Handling

All spans automatically capture errors with:
- `error.type` - Classification of error
- `error.message` - Error description
- Stack traces for debugging

## Performance Impact

OpenTelemetry instrumentation adds minimal overhead:
- Spans are batched for efficient export
- Sampling can be configured if needed
- Instrumentation is disabled when `enabled: false`

## Integration Examples

### Custom Span Creation

```go
tracer := otel.Tracer("openfga-sync/custom")
ctx, span := tracer.Start(ctx, "custom.operation")
defer span.End()

span.SetAttributes(
    attribute.String("custom.key", "value"),
    attribute.Int("custom.count", 42),
)
```

### Adding Custom Attributes

```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.String("resource.type", resourceType),
)
```

## Troubleshooting

### Common Issues

1. **No traces appearing:** Check OTLP endpoint connectivity
2. **High memory usage:** Adjust batch sizes in telemetry configuration
3. **Missing spans:** Ensure context propagation through function calls

### Debug Mode

Enable debug logging to see telemetry initialization:

```yaml
logging:
  level: "debug"
```

## Export Formats

- **Jaeger:** Native OTLP support via HTTP (port 4318)
- **OTLP Collector:** Direct export to OpenTelemetry Collector
- **Custom exporters:** Configurable via collector configuration
