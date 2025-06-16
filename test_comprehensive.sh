#!/bin/bash

# Comprehensive test for OpenFGA Sync HTTP endpoints and metrics
set -e

echo "🚀 OpenFGA Sync HTTP Endpoints & Metrics Test"
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_PORT=8080
CONFIG_FILE="config.test.yaml"
WAIT_TIME=5

# Function to print colored output
print_status() {
    if [ "$2" = "OK" ]; then
        echo -e "${GREEN}✅ $1${NC}"
    elif [ "$2" = "WARN" ]; then
        echo -e "${YELLOW}⚠️  $1${NC}"
    else
        echo -e "${RED}❌ $1${NC}"
    fi
}

# Function to test endpoint
test_endpoint() {
    local endpoint=$1
    local expected_status=$2
    local description=$3
    
    echo ""
    echo "Testing $description ($endpoint)..."
    
    local response=$(curl -s -w "\n%{http_code}" "http://localhost:${SERVICE_PORT}${endpoint}")
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "$expected_status" ]; then
        print_status "$description" "OK"
        echo "Response: $body" | head -c 200
        if [ ${#body} -gt 200 ]; then echo "..."; fi
        echo ""
        return 0
    else
        print_status "$description (Expected: $expected_status, Got: $http_code)" "ERROR"
        echo "Response: $body"
        return 1
    fi
}

# Function to check if port is in use
check_port() {
    if lsof -Pi :$SERVICE_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "⚠️  Port $SERVICE_PORT is already in use. Attempting to stop existing service..."
        pkill -f "openfga-sync.*$CONFIG_FILE" 2>/dev/null || true
        sleep 2
    fi
}

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    pkill -f "openfga-sync.*$CONFIG_FILE" 2>/dev/null || true
    wait 2>/dev/null || true
    print_status "Cleanup completed" "OK"
}

# Set up trap for cleanup
trap cleanup EXIT

# Main test execution
main() {
    echo "📋 Test Configuration:"
    echo "   - Service Port: $SERVICE_PORT"
    echo "   - Config File: $CONFIG_FILE"
    echo "   - Wait Time: ${WAIT_TIME}s"
    echo ""
    
    # Check if binary exists
    if [ ! -f "./openfga-sync" ]; then
        echo "🔨 Building openfga-sync binary..."
        go build -o openfga-sync
        print_status "Binary built successfully" "OK"
    fi
    
    # Check if config exists
    if [ ! -f "$CONFIG_FILE" ]; then
        print_status "Config file $CONFIG_FILE not found" "ERROR"
        exit 1
    fi
    
    # Check and cleanup port
    check_port
    
    # Start the service
    echo "🚀 Starting OpenFGA Sync service..."
    ./openfga-sync -config "$CONFIG_FILE" &
    SERVICE_PID=$!
    
    # Wait for service to start
    echo "⏳ Waiting ${WAIT_TIME}s for service to initialize..."
    sleep $WAIT_TIME
    
    # Check if service is still running
    if ! kill -0 $SERVICE_PID 2>/dev/null; then
        print_status "Service failed to start" "ERROR"
        exit 1
    fi
    
    print_status "Service started successfully (PID: $SERVICE_PID)" "OK"
    
    # Test endpoints
    echo ""
    echo "🔍 Testing HTTP Endpoints..."
    
    local tests_passed=0
    local total_tests=3
    
    # Test health endpoint
    if test_endpoint "/healthz" "200" "Health Check Endpoint"; then
        ((tests_passed++))
    fi
    
    # Test readiness endpoint
    if test_endpoint "/readyz" "200" "Readiness Check Endpoint"; then
        ((tests_passed++))
    fi
    
    # Test metrics endpoint
    if test_endpoint "/metrics" "200" "Prometheus Metrics Endpoint"; then
        ((tests_passed++))
    fi
    
    echo ""
    echo "📊 Testing Specific Metrics..."
    
    # Check for specific OpenFGA sync metrics
    local metrics_response=$(curl -s "http://localhost:${SERVICE_PORT}/metrics")
    local custom_metrics=(
        "openfga_sync_changes_processed_total"
        "openfga_sync_changes_errors_total"
        "openfga_sync_changes_lag_seconds"
        "openfga_sync_service_uptime_seconds_total"
        "openfga_sync_storage_connection_status"
    )
    
    local metrics_found=0
    for metric in "${custom_metrics[@]}"; do
        if echo "$metrics_response" | grep -q "$metric"; then
            print_status "Metric '$metric' found" "OK"
            ((metrics_found++))
        else
            print_status "Metric '$metric' missing" "ERROR"
        fi
    done
    
    # Final results
    echo ""
    echo "📈 Test Results:"
    echo "   - HTTP Endpoints: $tests_passed/$total_tests passed"
    echo "   - Custom Metrics: $metrics_found/${#custom_metrics[@]} found"
    
    if [ $tests_passed -eq $total_tests ] && [ $metrics_found -eq ${#custom_metrics[@]} ]; then
        print_status "All tests passed! 🎉" "OK"
        exit 0
    else
        print_status "Some tests failed!" "ERROR"
        exit 1
    fi
}

# Run main function
main "$@"
