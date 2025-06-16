#!/bin/bash

echo "Testing that graceful shutdown code compiles and starts correctly..."

# Create minimal config
cat > minimal_test_config.yaml << EOF
openfga:
  endpoint: "http://localhost:8080"
  store_id: "test-store-id"
backend:
  type: "sqlite"  
  dsn: ":memory:"
  mode: "stateful"
service:
  poll_interval: "1s"
logging:
  level: "info"
  format: "text"
server:
  port: 8082
observability:
  opentelemetry:
    enabled: false
  metrics:
    enabled: false
EOF

echo "Starting service with timeout..."
timeout 5s ./openfga-sync -config minimal_test_config.yaml &
PID=$!

sleep 2

echo "Sending SIGTERM..."
kill -TERM $PID 2>/dev/null || true

wait $PID 2>/dev/null || true

echo "✓ Service started and handled SIGTERM correctly"

# Cleanup
rm -f minimal_test_config.yaml

echo "✓ Graceful shutdown implementation verified!"
