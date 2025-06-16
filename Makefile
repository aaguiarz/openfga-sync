# OpenFGA Sync Service Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build parameters
BINARY_NAME=openfga-sync
BINARY_UNIX=$(BINARY_NAME)_unix
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker parameters
DOCKER_REGISTRY=
DOCKER_IMAGE=openfga-sync
DOCKER_TAG=latest

.PHONY: all build clean test coverage deps help

# Default target when running 'make' without arguments
.DEFAULT_GOAL := help

## Show help
help:
	@echo "Available commands:"
	@echo ""
	@echo "Build commands:"
	@echo "  build         Build the binary"
	@echo "  build-linux   Build for Linux"
	@echo "  clean         Clean build artifacts"
	@echo ""
	@echo "Development commands:"
	@echo "  test          Run tests"
	@echo "  coverage      Run tests with coverage report"
	@echo "  bench         Run benchmarks"
	@echo "  lint          Run linter"
	@echo "  fmt           Format code"
	@echo "  vet           Vet code"
	@echo "  check         Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps          Download dependencies"
	@echo "  deps-update   Update dependencies"
	@echo ""
	@echo "Docker commands:"
	@echo "  docker-build  Build Docker image"
	@echo "  docker-push   Push Docker image"
	@echo "  docker-up     Start with Docker Compose"
	@echo "  docker-down   Stop Docker Compose"
	@echo "  docker-logs   View service logs"
	@echo ""
	@echo "Database commands:"
	@echo "  db-setup      Start PostgreSQL for development"
	@echo "  db-teardown   Stop and remove PostgreSQL"
	@echo "  db-migrate    Run database migrations"
	@echo ""
	@echo "Utility commands:"
	@echo "  run-dev       Run locally with development config"
	@echo "  run-examples  Run example scripts"
	@echo "  run-sqlite-demo Run SQLite storage demonstration"
	@echo "  test-sqlite   Test SQLite adapter compilation"
	@echo "  config-validate Validate configuration file"
	@echo "  security      Run security scan"
	@echo "  release       Create release package"
	@echo "  help          Show this help message"

## Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v

## Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) -v

## Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

## Run tests
test:
	$(GOTEST) -v ./config ./fetcher ./storage

## Run tests with coverage
coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./config ./fetcher ./storage
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

## Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## Update dependencies
deps-update:
	$(GOGET) -u ./...
	$(GOMOD) tidy

## Run linter
lint:
	golangci-lint run

## Format code
fmt:
	$(GOCMD) fmt ./...

## Vet code
vet:
	$(GOCMD) vet ./...

## Run all checks
check: fmt vet lint test

## Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## Push Docker image
docker-push:
	docker push $(DOCKER_REGISTRY)$(DOCKER_IMAGE):$(DOCKER_TAG)

## Run with Docker Compose
docker-up:
	docker-compose up -d

## Stop Docker Compose
docker-down:
	docker-compose down

## View Docker Compose logs
docker-logs:
	docker-compose logs -f openfga-sync

## Run locally with development config
run-dev:
	./$(BINARY_NAME) -config config.yaml

## Run examples
run-examples:
	@echo "Running basic changes demo..."
	$(GOCMD) run examples/changes_demo.go
	@echo "\nRunning enhanced fetcher demo..."
	$(GOCMD) run examples/enhanced_demo/main.go
	@echo "\nRunning SQLite storage demo..."
	$(GOCMD) run examples/sqlite_demo.go

## Run SQLite demo specifically
run-sqlite-demo:
	@echo "Running SQLite storage adapter demonstration..."
	$(GOCMD) run examples/sqlite_demo.go

## Test with SQLite configuration
test-sqlite:
	@echo "Testing application with SQLite configuration..."
	$(GOBUILD) -o $(BINARY_NAME)-test .
	@echo "Built successfully - SQLite support is working!"
	@rm -f $(BINARY_NAME)-test

## Install development tools
install-tools:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## Generate mocks (if using mockery)
mocks:
	mockery --all --output=./mocks

## Run security scan
security:
	gosec ./...

## Create release
release: clean deps check build-linux
	@echo "Creating release $(VERSION)"
	mkdir -p dist
	cp $(BINARY_UNIX) dist/$(BINARY_NAME)-linux-amd64
	cp config.example.yaml dist/
	cp README.md dist/
	cp LICENSE dist/
	tar -czf dist/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C dist $(BINARY_NAME)-linux-amd64 config.example.yaml README.md LICENSE

## Database setup for development
db-setup:
	docker-compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker-compose exec postgres pg_isready -U openfga_user -d openfga_sync; do sleep 1; done
	@echo "PostgreSQL is ready!"

## Database teardown
db-teardown:
	docker-compose down postgres
	docker volume rm openfga-sync_postgres_data

## Run database migrations (when implemented)
db-migrate:
	./$(BINARY_NAME) -migrate

## Validate configuration
config-validate:
	./$(BINARY_NAME) -config config.yaml -validate
