# Test Suite Summary

## Overview

The Redis CLI test suite has been simplified to **3 focused, comprehensive test functions** that use **testcontainers-go** to spin up real Redis instances in Docker containers. This ensures high-fidelity testing against actual Redis servers.

## Test Structure

### 1. TestBasicRedisOperations
**Purpose**: Validates basic Redis commands using table-driven tests

**Commands Tested**:
- `SET` - Setting key-value pairs
- `GET` - Retrieving values
- `DEL` - Deleting keys
- `EXISTS` - Checking key existence
- `PING` - Server connectivity
- `INCR` - Incrementing counters

**Key Features**:
- Table-driven test design for easy expansion
- Setup and validation functions for each test case
- Tests command execution via `Do()` method
- Validates result types and values

### 2. TestRedisDataStructures
**Purpose**: Tests Redis data structures (Lists, Hashes, Sets)

**Subtests**:
- **List operations**: 
  - `LPUSH` - Push multiple items to list
  - `LRANGE` - Retrieve list items
  - Validates list length and order

- **Hash operations**:
  - `HSET` - Set hash fields
  - `HGET` - Get hash field values
  - `HGETALL` - Retrieve all hash fields
  - Validates hash structure integrity

- **Set operations**:
  - `SADD` - Add members to set
  - `SMEMBERS` - Get all set members
  - Validates set uniqueness and member count

### 3. TestConnectionAndAuthentication
**Purpose**: Tests connection features and advanced functionality

**Subtests**:
- **Connection status check**:
  - Validates PING command
  - Verifies connection health
  - Tests client connectivity

- **Multiple database support**:
  - Creates clients for DB 0 and DB 1
  - Sets same key in both databases with different values
  - Validates data isolation between databases

- **Key expiration**:
  - `SETEX` - Set key with TTL
  - `TTL` - Check time-to-live
  - Validates automatic key expiration after timeout
  - Confirms key no longer exists after expiration

## Prerequisites

### Required Software
1. **Go 1.24+** - For running tests
2. **Docker Desktop** - Required for testcontainers
3. **Docker Desktop must be RUNNING** - Tests will fail if Docker is not started

### Windows-Specific Requirements
- Use **Docker Desktop for Windows** (not rootless Docker in WSL2)
- Docker Desktop must be fully started before running tests
- Common error if Docker not running: `rootless Docker is not supported on Windows`

## Running Tests

### Quick Start
```bash
# 1. Start Docker Desktop first!

# 2. Verify Docker is running
docker ps

# 3. Run all tests
go test -v ./...

# 4. Or use make
make test
```

### Using the Docker Check Script

**Linux/macOS**:
```bash
chmod +x check-docker.sh
./check-docker.sh
```

**Windows**:
```cmd
check-docker.bat
```

The check script will:
- ✓ Verify Docker is installed
- ✓ Verify Docker daemon is running
- ✓ Check Docker version
- ✓ Ensure Redis image is available
- ✓ Test container creation capability

### Test Commands

```bash
# Run all tests
go test -v ./...

# Run specific test
go test -v -run TestBasicRedisOperations

# Run with race detector
go test -v -race ./...

# Generate coverage report
go test -v -coverprofile=coverage.txt ./...
go tool cover -html=coverage.txt -o coverage.html

# Run with timeout (recommended)
go test -v -timeout 5m ./...
```

## Helper Functions

### setupRedisContainer(t *testing.T)
**Returns**: `(testcontainers.Container, *redis.Client, context.Context)`

**Purpose**: Creates and configures a Redis container for testing

**What it does**:
1. Spins up a Redis 7 Alpine container
2. Waits for Redis to be ready (with retries)
3. Creates a configured Redis client with connection pooling
4. Returns container, client, and context for use in tests

**Usage Pattern**:
```go
container, redisClient, ctx := setupRedisContainer(t)
defer func() {
    redisClient.Close()
    container.Terminate(ctx)
}()
```

## Test Execution Flow

```
For each test:
1. setupRedisContainer() creates new Redis container
2. Wait for Redis to be ready (max 30 retries × 100ms)
3. Execute test logic
4. Defer cleanup: close client and terminate container
5. Next test gets fresh Redis instance
```

## CI/CD Integration

### GitHub Actions - Test Workflow
**File**: `.github/workflows/test.yml`

**Triggers**:
- Push to main/master/develop branches
- Pull requests to main/master/develop
- Manual workflow dispatch

**Steps**:
1. Checkout code
2. Setup Go 1.24
3. Cache Go modules
4. Download dependencies
5. Run tests with race detector and coverage
6. Upload coverage to Codecov (optional)
7. Generate HTML coverage report
8. Upload coverage report as artifact

### GitHub Actions - Release Workflow
**File**: `.github/workflows/release.yml`

**Trigger**: Tag push (e.g., `v1.0.0`)

**Steps**:
1. Build executables for 5 platforms
2. Create GitHub release
3. Upload binaries as release assets

## Troubleshooting

### Error: "rootless Docker is not supported on Windows"
**Cause**: Docker Desktop is not running or using WSL2 rootless mode

**Solution**:
1. Install Docker Desktop for Windows
2. Start Docker Desktop and wait for full startup
3. Run `docker ps` to verify
4. Run tests again

### Error: "cannot connect to the Docker daemon"
**Cause**: Docker daemon is not running

**Solution**:
- **Windows/macOS**: Start Docker Desktop
- **Linux**: `sudo systemctl start docker`
- Verify with: `docker ps`

### Tests Timeout
**Cause**: Container startup is slow or network issues

**Solution**:
- Pre-pull Redis image: `docker pull redis:7-alpine`
- Increase test timeout: `go test -v -timeout 10m ./...`
- Check Docker resources in Docker Desktop settings

### Port Already in Use
**Cause**: Another Redis instance on port 6379

**Solution**:
```bash
# Find process using port
docker ps

# Stop conflicting containers
docker stop <container-id>
```

## Test Coverage Goals

- **>80% code coverage** for core functionality
- **100% coverage** for critical paths (connection, command execution)
- **Edge case testing** for error conditions

## Best Practices

1. **Always use testcontainers** for Redis integration tests
2. **Clean up resources** with defer statements
3. **Use descriptive test names** and subtests
4. **Test both success and failure scenarios**
5. **Isolate tests** - each test gets a fresh Redis instance
6. **Use table-driven tests** for testing multiple scenarios
7. **Validate both result values and types**

## File Structure

```
rediscli/
├── main.go                    # Main application
├── main_test.go               # Test suite (3 test functions)
├── check-docker.sh            # Docker check script (Linux/macOS)
├── check-docker.bat           # Docker check script (Windows)
├── .github/
│   └── workflows/
│       ├── test.yml           # CI test workflow
│       └── release.yml        # Release workflow
├── Makefile                   # Build and test automation
├── README.md                  # User documentation
└── TESTING.md                 # Detailed testing guide
```

## Quick Reference

### Make Commands
```bash
make test              # Run tests (checks Docker first)
make test-coverage     # Generate coverage report
make docker-check      # Verify Docker is ready
make docker-up         # Start local Redis for manual testing
make docker-down       # Stop local Redis
```

### Environment Variables for CLI
```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=yourpass  # Optional
export REDIS_DB=0               # Optional
```

## Contributing Tests

When adding new features:
1. Write tests first (TDD recommended)
2. Add to existing test function or create new subtest
3. Use `setupRedisContainer()` helper
4. Follow table-driven test pattern for multiple scenarios
5. Ensure Docker is running before testing
6. Run `make test-coverage` to verify coverage
7. Update documentation if adding new test categories

## Summary

✅ **3 comprehensive test functions** covering all major Redis operations
✅ **Real Redis containers** via testcontainers for high-fidelity testing
✅ **Easy to run** with `make test` or `go test -v ./...`
✅ **CI/CD ready** with GitHub Actions workflows
✅ **Well documented** with clear troubleshooting guides
✅ **Docker check scripts** to verify environment before testing

**Total Test Coverage**: ~200 lines of focused, maintainable test code