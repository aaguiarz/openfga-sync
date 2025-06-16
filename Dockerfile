# Multi-stage build for minimal production image
FROM golang:1.23-alpine AS builder

# Install git, ca-certificates, and sqlite for dependencies
RUN apk add --no-cache git ca-certificates sqlite-dev gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for SQLite support
RUN CGO_ENABLED=1 go build \
    -ldflags='-w -s' \
    -a \
    -o openfga-sync \
    .

# Production stage - use alpine instead of scratch for SQLite support
FROM alpine:latest

# Install ca-certificates and sqlite runtime
RUN apk --no-cache add ca-certificates sqlite

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/openfga-sync /app/openfga-sync

# Copy example configuration
COPY --from=builder /app/config.example.yaml /app/config.example.yaml

# Create a minimal config for health checks
RUN echo 'server:\n  port: 8080\nopenfga:\n  endpoint: "http://localhost"\n  store_id: "health-check"\nbackend:\n  type: "sqlite"\n  dsn: ":memory:"\n  mode: "stateful"\nlogging:\n  level: "error"' > /app/health-config.yaml

# Create non-root user and group
RUN addgroup -g 1000 appgroup && adduser -D -u 1000 -G appgroup appuser

# Change ownership of app directory
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER 1000:1000

# Expose health check and metrics port
EXPOSE 8080

# Health check using HTTP endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Default command
ENTRYPOINT ["/app/openfga-sync"]
CMD ["-config", "/app/config.example.yaml"]

# Metadata
LABEL maintainer="OpenFGA Sync Service" \
      description="OpenFGA authorization change synchronization service" \
      version="1.0.0" \
      org.opencontainers.image.title="OpenFGA Sync Service" \
      org.opencontainers.image.description="Synchronizes OpenFGA authorization changes to databases" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.vendor="OpenFGA Community" \
      org.opencontainers.image.licenses="MIT"
