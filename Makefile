.PHONY: build run test test-unit test-integration clean fmt check-fmt lint help

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

# Install dependencies
deps:
	go mod download
	go mod tidy

# Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html

# Help
help:
	@echo "ArgusGo Makefile commands:"
	@echo ""
	@echo "  build           - Build the application binary"
	@echo "  run             - Run the application"
	@echo "  test            - Run all tests (unit + integration)"
	@echo "  test-unit       - Run unit tests only"
	@echo "  test-integration- Run integration tests only"
	@echo "  it              - Alias for test-integration"
	@echo "  clean           - Clean build artifacts"
	@echo "  fmt             - Format code"
	@echo "  check-fmt       - Check code formatting (CI)"
	@echo "  lint            - Run linter"
	@echo "  deps            - Download and tidy dependencies"
	@echo "  coverage        - Generate test coverage report"
	@echo "  help            - Show this help message"
