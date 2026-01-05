.PHONY: build run test test-unit test-integration clean fmt check-fmt lint security-scan help \
	dev-infra-up dev-infra-down dev-infra-logs dev-deploy dev-deploy-stop

# Default target
all: build

# Build the application
build:
	go build -o bin/argus ./cmd/argus

# Run the application
run:
	go run ./cmd/argus -config config/config.yaml

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v -race ./internal/...

# Run integration tests
test-integration:
	go test -v -count=1 ./integration/...

# Shorthand for integration tests (backward compatibility)
it: test-integration

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Format code
fmt:
	go fmt ./...

# Check formatting (fails if code is not formatted)
check-fmt:
	@test -z "$$(gofmt -l .)" || (echo "Code is not formatted. Run 'make fmt' to fix." && gofmt -l . && exit 1)

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run security scan (requires gosec)
security-scan:
	gosec ./...

# Install dependencies
deps:
	go mod download
	go mod tidy

# Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html

# Start development infrastructure (Kafka, Redis, PostgreSQL)
dev-infra-up:
	docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@docker-compose ps

# Stop development infrastructure
dev-infra-down:
	docker-compose down

# View development infrastructure logs
dev-infra-logs:
	docker-compose logs -f

# Deploy ArgusGo with storage backend (starts infra and runs app)
dev-deploy: dev-infra-up build
	@echo "Starting ArgusGo in storage mode..."
	./bin/argus -config config/config-storage.yaml

# Stop ArgusGo development deployment
dev-deploy-stop: dev-infra-down

# Help
help:
	@echo "ArgusGo Makefile commands:"
	@echo ""
	@echo "  build            - Build the application binary"
	@echo "  run              - Run the application (memory mode)"
	@echo "  test             - Run all tests (unit + integration)"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  it               - Alias for test-integration"
	@echo "  clean            - Clean build artifacts"
	@echo "  fmt              - Format code"
	@echo "  check-fmt        - Check code formatting (CI)"
	@echo "  lint             - Run linter"
	@echo "  security-scan    - Run security scan (gosec)"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  coverage         - Generate test coverage report"
	@echo ""
	@echo "Development with Storage Backend:"
	@echo "  dev-infra-up     - Start Kafka, Redis, PostgreSQL containers"
	@echo "  dev-infra-down   - Stop infrastructure containers"
	@echo "  dev-infra-logs   - View infrastructure container logs"
	@echo "  dev-deploy       - Start infra and run ArgusGo in storage mode"
	@echo "  dev-deploy-stop  - Stop ArgusGo and infrastructure"
	@echo ""
	@echo "  help             - Show this help message"
