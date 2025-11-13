# Redis CLI

A lightweight, cross-platform Redis command-line interface tool written in Go.

## Features

- 🚀 Fast and lightweight
- 🔐 Supports password authentication
- 📦 Single binary executable
- 🌐 Cross-platform (Linux, macOS, Windows)
- 🔄 Supports all Redis commands
- 🎯 Multiple database selection
- 🧪 Comprehensive test coverage with testcontainers

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/yourusername/rediscli/releases) page:

- **Linux (AMD64)**: `rediscli-linux-amd64`
- **Linux (ARM64)**: `rediscli-linux-arm64`
- **macOS (Intel)**: `rediscli-darwin-amd64`
- **macOS (Apple Silicon)**: `rediscli-darwin-arm64`
- **Windows**: `rediscli-windows-amd64.exe`

### Make it Executable (Linux/macOS)

```bash
chmod +x rediscli-*
sudo mv rediscli-* /usr/local/bin/rediscli
```

### Build from Source

```bash
git clone https://github.com/yourusername/rediscli.git
cd rediscli
go build -o rediscli .
```

## Usage

### Environment Variables

Configure Redis connection using environment variables:

```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379          # Optional, defaults to 6379
export REDIS_PASSWORD=yourpass  # Optional
export REDIS_DB=0               # Optional, defaults to 0
```

### Basic Commands

```bash
# Set a key
./rediscli SET mykey "Hello World"

# Get a key
./rediscli GET mykey

# Check if key exists
./rediscli EXISTS mykey

# Delete a key
./rediscli DEL mykey

# List all keys
./rediscli KEYS "*"

# Increment a counter
./rediscli INCR counter

# Set with expiration (seconds)
./rediscli SETEX tempkey 60 "expires in 60 seconds"

# Get TTL
./rediscli TTL tempkey

# Ping Redis
./rediscli PING
```

### Working with Lists

```bash
# Push to list
./rediscli LPUSH mylist "item1" "item2" "item3"

# Get list range
./rediscli LRANGE mylist 0 -1

# Pop from list
./rediscli LPOP mylist
```

### Working with Hashes

```bash
# Set hash field
./rediscli HSET user:1 name "John Doe"
./rediscli HSET user:1 email "john@example.com"

# Get hash field
./rediscli HGET user:1 name

# Get all hash fields
./rediscli HGETALL user:1
```

### Working with Sets

```bash
# Add to set
./rediscli SADD myset member1 member2 member3

# Get all members
./rediscli SMEMBERS myset

# Check membership
./rediscli SISMEMBER myset member1
```

## Docker Example

```bash
# Start Redis with Docker
docker run -d -p 6379:6379 --name redis redis:7-alpine

# Set environment variables
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Use the CLI
./rediscli PING
./rediscli SET hello world
./rediscli GET hello
```

## Development

### Prerequisites

- Go 1.24 or higher
- Docker (for running tests)

### Running Tests

The project uses testcontainers-go for integration testing, which requires Docker to be running.

```bash
# Make sure Docker is running
docker ps

# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# View coverage report
go tool cover -html=coverage.txt
```

### Test Coverage

The test suite includes:

- ✅ Connection status validation
- ✅ Basic Redis commands (GET, SET, DEL, EXISTS, KEYS)
- ✅ List operations (LPUSH, LRANGE)
- ✅ Hash operations (HSET, HGET, HGETALL)
- ✅ Set operations (SADD, SMEMBERS)
- ✅ Expiration and TTL
- ✅ Password authentication
- ✅ Multiple database support
- ✅ Command execution via Do()

### Building

```bash
# Build for current platform
go build -o rediscli .

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o rediscli-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o rediscli-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o rediscli-windows-amd64.exe .

# Build with size optimization
go build -ldflags="-s -w" -o rediscli .
```

## Configuration Options

| Environment Variable | Description | Default | Required |
|---------------------|-------------|---------|----------|
| `REDIS_HOST` | Redis server hostname | - | Yes |
| `REDIS_PORT` | Redis server port | `6379` | No |
| `REDIS_PASSWORD` | Redis password | - | No |
| `REDIS_DB` | Redis database number | `0` | No |

## CI/CD

### GitHub Actions Workflows

#### Build and Release (`release.yml`)

Automatically builds executables for multiple platforms and creates a GitHub release when you push a tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

#### Run Tests (`test.yml`)

Runs the test suite on every push and pull request to main/master/develop branches.

## Project Structure

```
rediscli/
├── .github/
│   └── workflows/
│       ├── release.yml    # Build and release workflow
│       └── test.yml       # Test workflow
├── main.go                # Main application
├── main_test.go           # Comprehensive tests
├── go.mod                 # Go module definition
├── go.sum                 # Dependency checksums
└── README.md             # This file
```

## Dependencies

- [go-redis/redis](https://github.com/redis/go-redis) - Redis client for Go
- [testcontainers-go](https://github.com/testcontainers/testcontainers-go) - Docker containers for testing

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Add tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

MIT License - feel free to use this project however you'd like!

## Troubleshooting

### Connection Refused

```
Error trying to connect with Redis: dial tcp [::1]:6379: connect: connection refused
```

**Solution**: Make sure Redis is running and the `REDIS_HOST` and `REDIS_PORT` are correct.

### Authentication Failed

```
Redis command error: NOAUTH Authentication required
```

**Solution**: Set the `REDIS_PASSWORD` environment variable with the correct password.

### Tests Failing on Windows

**Issue**: Testcontainers requires Docker Desktop to be running on Windows.

**Solution**: 
1. Install Docker Desktop for Windows
2. Start Docker Desktop
3. Run tests again

### Tests Failing - "Cannot connect to Docker daemon"

**Solution**: Ensure Docker is running:
```bash
docker ps
```

## Examples

### Batch Operations

```bash
# Set multiple keys
./rediscli MSET key1 "value1" key2 "value2" key3 "value3"

# Get multiple keys
./rediscli MGET key1 key2 key3
```

### Atomic Operations

```bash
# Increment
./rediscli INCR counter
./rediscli INCRBY counter 5

# Decrement
./rediscli DECR counter
./rediscli DECRBY counter 3
```

### Scripting

```bash
#!/bin/bash
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Initialize counters
./rediscli SET page_views 0
./rediscli SET unique_visitors 0

# Simulate tracking
for i in {1..100}; do
    ./rediscli INCR page_views
    ./rediscli SADD visitors "user_$i"
done

# Get results
echo "Page Views: $(./rediscli GET page_views)"
echo "Unique Visitors: $(./rediscli SCARD visitors)"
```

## Performance

This CLI tool uses connection pooling for optimal performance:
- Pool Size: 200 connections
- Minimum Idle Connections: 50

## Support

If you encounter any issues or have questions:
- Open an issue on [GitHub Issues](https://github.com/yourusername/rediscli/issues)
- Check existing issues for solutions
- Provide as much detail as possible (OS, Go version, Redis version, error messages)

---

Made with ❤️ using Go and Redis