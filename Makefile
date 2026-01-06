.PHONY: build run test test-unit test-integration clean fmt check-fmt lint security-scan help \
	image image-run image-stop test-integration-container \
	dev-infra-up dev-infra-down dev-infra-logs dev-deploy dev-deploy-stop

# Container image settings
IMAGE_NAME ?= argus-go
IMAGE_TAG ?= latest
CONTAINER_NAME ?= argus-go-dev

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

# Build container image
image:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Run container (for development)
image-run: image
	docker run -d --name $(CONTAINER_NAME) -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)
	@echo "ArgusGo container started on http://localhost:8080"
	@echo "Stop with: make image-stop"

# Stop and remove container
image-stop:
	-docker stop $(CONTAINER_NAME)
	-docker rm $(CONTAINER_NAME)

# Run integration tests against container
test-integration-container: image
	@echo "Starting ArgusGo container for integration tests..."
	-docker stop $(CONTAINER_NAME)-test 2>/dev/null || true
	-docker rm $(CONTAINER_NAME)-test 2>/dev/null || true
	docker run -d --name $(CONTAINER_NAME)-test -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)
	@echo "Waiting for container to be ready..."
	@sleep 3
	@echo "Running integration tests..."
	ARGUS_BASE_URL=http://localhost:8080 go test -v -count=1 ./integration/...
	@echo "Stopping container..."
	-docker stop $(CONTAINER_NAME)-test
	-docker rm $(CONTAINER_NAME)-test

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
	@echo "  build                      - Build the application binary"
	@echo "  run                        - Run the application (memory mode)"
	@echo "  test                       - Run all tests (unit + integration)"
	@echo "  test-unit                  - Run unit tests only"
	@echo "  test-integration           - Run integration tests only"
	@echo "  test-integration-container - Run integration tests against container"
	@echo "  it                         - Alias for test-integration"
	@echo "  clean                      - Clean build artifacts"
	@echo "  fmt                        - Format code"
	@echo "  check-fmt                  - Check code formatting (CI)"
	@echo "  lint                       - Run linter"
	@echo "  security-scan              - Run security scan (gosec)"
	@echo "  deps                       - Download and tidy dependencies"
	@echo "  coverage                   - Generate test coverage report"
	@echo ""
	@echo "Container commands:"
	@echo "  image                      - Build container image"
	@echo "  image-run                  - Build and run container"
	@echo "  image-stop                 - Stop and remove container"
	@echo ""
	@echo "Development with Storage Backend:"
	@echo "  dev-infra-up               - Start Kafka, Redis, PostgreSQL containers"
	@echo "  dev-infra-down             - Stop infrastructure containers"
	@echo "  dev-infra-logs             - View infrastructure container logs"
	@echo "  dev-deploy                 - Start infra and run ArgusGo in storage mode"
	@echo "  dev-deploy-stop            - Stop ArgusGo and infrastructure"
	@echo ""
	@echo "  help                       - Show this help message"
