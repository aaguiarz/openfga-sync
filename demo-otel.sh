#!/bin/bash

# OpenTelemetry Demo Script for OpenFGA Sync
# This script demonstrates the OpenTelemetry tracing and metrics functionality

echo "🔭 OpenFGA Sync - OpenTelemetry Demo"
echo "===================================="
echo ""

# Check if Jaeger is running (optional)
if command -v curl &> /dev/null; then
    if curl -s http://localhost:16686 > /dev/null 2>&1; then
        echo "✅ Jaeger UI detected at http://localhost:16686"
    else
        echo "ℹ️  Jaeger UI not detected. You can start Jaeger with:"
        echo "   docker run -d --name jaeger \\"
        echo "     -p 16686:16686 \\"
        echo "     -p 14268:14268 \\"
        echo "     -p 4317:4317 \\"
        echo "     -p 4318:4318 \\"
        echo "     jaegertracing/all-in-one:latest"
        echo ""
    fi
fi

# Check if OTEL collector is running (optional)
if curl -s http://localhost:4318/v1/traces > /dev/null 2>&1; then
    echo "✅ OpenTelemetry Collector detected at http://localhost:4318"
else
    echo "ℹ️  OpenTelemetry Collector not detected."
    echo "   For a simple setup, you can use Jaeger which accepts OTLP directly."
fi

echo ""
echo "🚀 Starting OpenFGA Sync with OpenTelemetry enabled..."
echo "   Configuration: config.otel.yaml"
echo "   Tracing: Enabled"
echo "   Metrics: Enabled"
echo "   Storage: SQLite (in-memory)"
echo ""

# Run the application with OpenTelemetry configuration
./openfga-sync -config config.otel.yaml
