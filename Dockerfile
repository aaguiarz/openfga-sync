# Multi-stage build for minimal production image
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates for downloading dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o openfga-sync \
    .

# Production stage
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder stage
COPY --from=builder /app/openfga-sync /openfga-sync

# Copy example configuration
COPY --from=builder /app/config.example.yaml /config.example.yaml

# Create non-root user
USER 1000:1000

# Expose health check port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/openfga-sync", "-config", "/dev/null", "--health-check"] || exit 1

# Default command
ENTRYPOINT ["/openfga-sync"]

# Metadata
LABEL maintainer="OpenFGA Sync Service" \
      description="OpenFGA authorization change synchronization service" \
      version="1.0.0" \
      org.opencontainers.image.title="OpenFGA Sync Service" \
      org.opencontainers.image.description="Synchronizes OpenFGA authorization changes to databases" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.vendor="OpenFGA Community" \
      org.opencontainers.image.licenses="MIT"
