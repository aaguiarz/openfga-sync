#!/bin/bash

# Test graceful shutdown functionality
set -e

echo "Testing graceful shutdown functionality..."

# Create a minimal test configuration
cat > test_shutdown_config.yaml << EOF
openfga:
  endpoint: "http://localhost:8080"
  store_id: "test-store-id"

backend:
  type: "sqlite"
  dsn: ":memory:"
  mode: "stateful"

service:
  poll_interval: "1s"
  batch_size: 10

logging:
  level: "info"
  format: "text"

server:
  port: 8081

observability:
  opentelemetry:
    enabled: false
  metrics:
    enabled: true
EOF

echo "Starting openfga-sync in background..."

# Start the service in background
./openfga-sync -config test_shutdown_config.yaml &
SERVICE_PID=$!

echo "Service started with PID: $SERVICE_PID"

# Wait a moment for the service to start
sleep 3

echo "Checking if service is running..."
if kill -0 $SERVICE_PID 2>/dev/null; then
    echo "✓ Service is running"
else
    echo "✗ Service failed to start"
    exit 1
fi

# Test health endpoint
echo "Testing health endpoint..."
if curl -s http://localhost:8081/healthz > /dev/null; then
    echo "✓ Health endpoint is responding"
else
    echo "⚠ Health endpoint not responding (may be expected if OpenFGA not available)"
fi

# Test graceful shutdown with SIGTERM
echo "Sending SIGTERM for graceful shutdown..."
kill -TERM $SERVICE_PID

# Wait for graceful shutdown
echo "Waiting for graceful shutdown..."
WAIT_COUNT=0
while kill -0 $SERVICE_PID 2>/dev/null; do
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
    if [ $WAIT_COUNT -gt 15 ]; then
        echo "✗ Service did not shut down gracefully within 15 seconds"
        kill -KILL $SERVICE_PID
        exit 1
    fi
done

echo "✓ Service shut down gracefully in ${WAIT_COUNT} seconds"

# Test force shutdown with second signal
echo ""
echo "Testing force shutdown with second signal..."

# Start service again
./openfga-sync -config test_shutdown_config.yaml &
SERVICE_PID=$!

sleep 2

echo "Sending first SIGTERM..."
kill -TERM $SERVICE_PID
sleep 1

echo "Sending second SIGTERM (should force immediate exit)..."
kill -TERM $SERVICE_PID

# Wait for forced shutdown
WAIT_COUNT=0
while kill -0 $SERVICE_PID 2>/dev/null; do
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
    if [ $WAIT_COUNT -gt 5 ]; then
        echo "✗ Service did not force shutdown within 5 seconds"
        kill -KILL $SERVICE_PID
        exit 1
    fi
done

echo "✓ Service force shutdown in ${WAIT_COUNT} seconds"

# Cleanup
rm -f test_shutdown_config.yaml

echo ""
echo "✓ All graceful shutdown tests passed!"
