.PHONY: help build test test-verbose test-coverage clean install run docker-up docker-down lint fmt vet all

# Default target
help:
	@echo "Available targets:"
	@echo "  make build           - Build the executable"
	@echo "  make build-all       - Build for all platforms"
	@echo "  make test            - Run tests"
	@echo "  make test-verbose    - Run tests with verbose output"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make install         - Install the binary to /usr/local/bin"
	@echo "  make run             - Run the application"
	@echo "  make docker-up       - Start Redis container for testing"
	@echo "  make docker-down     - Stop Redis container"
	@echo "  make lint            - Run linter"
	@echo "  make fmt             - Format code"
	@echo "  make vet             - Run go vet"
	@echo "  make all             - Format, vet, test, and build"

# Build the executable for current platform
build:
	go build -ldflags="-s -w" -o rediscli .

# Build for all platforms
build-all:
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/rediscli-linux-amd64 .
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/rediscli-linux-arm64 .
	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/rediscli-windows-amd64.exe .
	@echo "Building for macOS AMD64..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/rediscli-darwin-amd64 .
	@echo "Building for macOS ARM64..."
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/rediscli-darwin-arm64 .
	@echo "All builds complete! Check the dist/ directory"

# Run tests
test:
	go test -v ./...

# Run tests with verbose output
test-verbose:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	rm -f rediscli rediscli.exe
	rm -rf dist/
	rm -f coverage.txt coverage.html

# Install binary to system (Unix-like systems)
install: build
	@if [ "$(OS)" = "Windows_NT" ]; then \
		echo "Please manually copy rediscli.exe to a directory in your PATH"; \
	else \
		sudo mv rediscli /usr/local/bin/rediscli; \
		echo "Installed to /usr/local/bin/rediscli"; \
	fi

# Run the application (example)
run: build
	@echo "Example: REDIS_HOST=localhost ./rediscli PING"
	@echo "Set REDIS_HOST and other env vars before running"

# Start Redis container for local testing
docker-up:
	docker run -d --name redis-test -p 6379:6379 redis:7-alpine
	@echo "Redis container started on port 6379"
	@echo "Run: export REDIS_HOST=localhost"

# Stop Redis container
docker-down:
	docker stop redis-test || true
	docker rm redis-test || true
	@echo "Redis container stopped and removed"

# Run golangci-lint (if installed)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks and build
all: fmt vet test build
	@echo "All checks passed and binary built successfully!"

# Download dependencies
deps:
	go mod download
	go mod tidy

# Update dependencies
update-deps:
	go get -u ./...
	go mod tidy

# Create dist directory
dist:
	mkdir -p dist
