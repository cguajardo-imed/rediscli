# Testing Guide

This document provides comprehensive information about testing the Redis CLI project.

## Overview

The test suite uses **testcontainers-go** to spin up real Redis instances in Docker containers, ensuring that tests run against actual Redis servers rather than mocks. This provides high confidence that the CLI works correctly with real Redis deployments.

## Prerequisites

### Required Software

1. **Go 1.24+**: For running the tests
2. **Docker Desktop**: Required by testcontainers to spin up Redis containers
3. **Docker Desktop Running**: Docker Desktop must be started and running

**⚠️ Important for Windows Users**: You must use Docker Desktop for Windows, not rootless Docker in WSL2

### Verify Prerequisites

```bash
# Check Go version
go version

# Check Docker is installed and running
docker ps

# Check Docker version
docker --version
```

## Running Tests

### Quick Start

```bash
# Run all tests
go test -v ./...

# Run tests with race detector
go test -v -race ./...

# Run specific test
go test -v -run TestConnectionStatus

# Run tests with coverage
go test -v -coverprofile=coverage.txt ./...
go tool cover -html=coverage.txt -o coverage.html
```

### Using Make

```bash
# Run tests
make test

# Run tests with verbose output
make test-verbose

# Generate coverage report
make test-coverage

# Start a local Redis instance for manual testing
make docker-up

# Stop the Redis instance
make docker-down
```

## Test Structure

### Test File: `main_test.go`

The test suite is organized into **3 comprehensive test functions**:

#### 1. **TestBasicRedisOperations**
Tests basic Redis commands using the `Do()` method.

**Commands tested:**
- `SET` - Setting key-value pairs
- `GET` - Getting values
- `DEL` - Deleting keys
- `EXISTS` - Checking key existence
- `PING` - Server connectivity check
- `INCR` - Incrementing counters

**What it validates:**
- Command execution
- Data integrity
- Result type validation
- Error handling

#### 2. **TestRedisDataStructures**
Tests Redis data structures (Lists, Hashes, Sets).

**Subtests included:**
- **List operations**: `LPUSH`, `LRANGE`
- **Hash operations**: `HSET`, `HGET`, `HGETALL`
- **Set operations**: `SADD`, `SMEMBERS`

**What it validates:**
- Data structure integrity
- Multiple item operations
- Data retrieval accuracy

#### 3. **TestConnectionAndAuthentication**
Tests connection features and advanced functionality.

**Subtests included:**
- **Connection status check**: PING command validation
- **Multiple database support**: Testing DB 0 and DB 1 isolation
- **Key expiration**: `SETEX`, `TTL`, automatic expiration

**What it validates:**
- Connection health
- Database isolation
- Time-to-live functionality
- Key expiration behavior

## Test Helpers

### `setupRedisContainer(t *testing.T)`

A helper function that:
1. Creates a Redis container using testcontainers
2. Waits for Redis to be ready
3. Returns the container, a configured Redis client, and context
4. Handles all container lifecycle management

**Signature:**
```go
func setupRedisContainer(t *testing.T) (testcontainers.Container, *redis.Client, context.Context)
```

**Usage:**
```go
container, redisClient, ctx := setupRedisContainer(t)
defer func() {
    redisClient.Close()
    container.Terminate(ctx)
}()
```

## Continuous Integration

### GitHub Actions

The project includes two CI workflows:

#### 1. Test Workflow (`.github/workflows/test.yml`)

Runs on every push and pull request to main/master/develop branches.

**What it does:**
- Sets up Go environment
- Installs dependencies
- Runs all tests with race detector
- Generates coverage report
- Uploads coverage to Codecov (optional)
- Creates coverage HTML report as artifact

#### 2. Release Workflow (`.github/workflows/release.yml`)

Runs when you push a version tag (e.g., `v1.0.0`).

**What it does:**
- Builds executables for multiple platforms
- Creates a GitHub release
- Uploads binaries as release assets

## Coverage

### Viewing Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.txt ./...

# View coverage in terminal
go tool cover -func=coverage.txt

# Generate HTML report
go tool cover -html=coverage.txt -o coverage.html

# Open in browser (Linux/macOS)
open coverage.html

# Open in browser (Windows)
start coverage.html
```

### Coverage Goals

The test suite aims for:
- **>80% code coverage** for core functionality
- **100% coverage** for critical paths (connection, command execution)
- **Edge case testing** for error conditions

## Troubleshooting Tests

### Issue: Tests Fail with "Cannot connect to Docker daemon"

**Cause:** Docker is not running or not accessible.

**Solution:**
```bash
# Start Docker Desktop (macOS/Windows)
# Or start Docker daemon (Linux)
sudo systemctl start docker

# Verify Docker is running
docker ps
```

### Issue: "rootless Docker is not supported on Windows"

**Cause:** Testcontainers has limitations with rootless Docker on Windows.

**Solution:**
1. Download and install [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/)
2. Start Docker Desktop and wait for it to fully start
3. Verify Docker is running: `docker ps`
4. Ensure Docker Desktop is running in Windows mode (not WSL2 rootless mode)
5. Run tests again: `go test -v ./...`

### Issue: Tests Timeout

**Cause:** Container startup might be slow or network issues.

**Solution:**
- Increase the timeout in `wait.ForLog().WithStartupTimeout()`
- Check Docker resources (CPU, Memory)
- Pull the Redis image beforehand: `docker pull redis:7-alpine`

### Issue: Port Already in Use

**Cause:** Another Redis instance or test container is using port 6379.

**Solution:**
```bash
# Find what's using the port
lsof -i :6379  # macOS/Linux
netstat -ano | findstr :6379  # Windows

# Stop any existing Redis containers
docker ps
docker stop <container-id>
```

### Issue: Tests Pass Locally but Fail in CI

**Possible causes:**
1. Different Docker versions
2. Network configurations in CI environment
3. Timing issues

**Solution:**
- Check CI logs for specific errors
- Ensure container wait strategies are adequate
- Add retries for flaky tests if needed

## Writing New Tests

### Best Practices

1. **Always use testcontainers** for Redis tests
2. **Clean up resources** in defer statements
3. **Use descriptive test names** that explain what is being tested
4. **Test both success and failure cases**
5. **Isolate tests** - each test should be independent

### Example Test Template

```go
func TestNewFeature(t *testing.T) {
    // Setup Redis container
    container, redisClient, ctx := setupRedisContainer(t)
    defer func() {
        redisClient.Close()
        container.Terminate(ctx)
    }()

    // Setup test data
    // ... prepare your test data ...

    // Execute the command
    result, err := redisClient.Do(ctx, "COMMAND", "args").Result()

    // Assertions
    if err != nil {
        t.Fatalf("Command failed: %v", err)
    }

    if result != expectedValue {
        t.Errorf("Expected %v, got %v", expectedValue, result)
    }

    // Cleanup (if needed)
    // ... cleanup test data ...
}
```

### Table-Driven Tests

For testing multiple scenarios:

```go
func TestMultipleScenarios(t *testing.T) {
    container, redisClient, ctx := setupRedisContainer(t)
    defer func() {
        redisClient.Close()
        container.Terminate(ctx)
    }()

    tests := []struct {
        name     string
        setup    func() error
        command  []interface{}
        validate func(result interface{}) error
    }{
        {
            name:    "Test case 1",
            setup:   func() error { return nil },
            command: []interface{}{"SET", "key1", "value1"},
            validate: func(result interface{}) error {
                if result != "OK" {
                    return fmt.Errorf("expected 'OK', got %v", result)
                }
                return nil
            },
        },
        // Add more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if err := tt.setup(); err != nil {
                t.Fatalf("Setup failed: %v", err)
            }
            
            result, err := redisClient.Do(ctx, tt.command...).Result()
            if err != nil {
                t.Fatalf("Command failed: %v", err)
            }
            
            if err := tt.validate(result); err != nil {
                t.Errorf("Validation failed: %v", err)
            }
        })
    }
}
```

## Performance Testing

### Benchmark Tests

Create benchmark tests for performance-critical operations:

```go
func BenchmarkRedisGet(b *testing.B) {
    t := &testing.T{}
    container, redisClient, ctx := setupRedisContainer(t)
    defer func() {
        redisClient.Close()
        container.Terminate(ctx)
    }()

    // Setup
    redisClient.Set(ctx, "benchkey", "benchvalue", 0)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        redisClient.Get(ctx, "benchkey")
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```

## Manual Testing

For manual testing during development:

```bash
# Start a local Redis instance
make docker-up

# Set environment variables
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Build and test the CLI
make build
./rediscli PING
./rediscli SET test "Hello World"
./rediscli GET test

# Stop Redis when done
make docker-down
```

## Additional Resources

- [Testcontainers Go Documentation](https://golang.testcontainers.org/)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Redis Commands Reference](https://redis.io/commands/)
- [go-redis Documentation](https://redis.uptrace.dev/)

## Contributing Tests

When contributing new features:

1. Write tests first (TDD approach recommended)
2. Ensure all existing tests pass
3. Add tests for edge cases
4. Update this documentation if adding new test categories
5. Run `make test-coverage` and verify coverage doesn't decrease

---

**Questions or Issues?**

Open an issue on GitHub with the `testing` label.