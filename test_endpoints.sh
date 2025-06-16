#!/bin/bash

# Test script for HTTP endpoints
echo "Testing OpenFGA Sync HTTP Endpoints"
echo "===================================="

# Start the service in the background
echo "Starting OpenFGA Sync service..."
./openfga-sync -config config.test.yaml &
SERVICE_PID=$!

# Wait for service to start
echo "Waiting for service to start..."
sleep 3

# Test health endpoint
echo ""
echo "Testing /healthz endpoint:"
curl -s http://localhost:8080/healthz | jq '.' || echo "Failed to reach health endpoint"

# Test readiness endpoint  
echo ""
echo "Testing /readyz endpoint:"
curl -s http://localhost:8080/readyz | jq '.' || echo "Failed to reach readiness endpoint"

# Test metrics endpoint
echo ""
echo "Testing /metrics endpoint:"
curl -s http://localhost:8080/metrics | head -20 || echo "Failed to reach metrics endpoint"

# Cleanup
echo ""
echo "Stopping service..."
kill $SERVICE_PID 2>/dev/null
wait $SERVICE_PID 2>/dev/null

echo ""
echo "Test completed!"
