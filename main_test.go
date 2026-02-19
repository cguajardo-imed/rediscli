package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupRedisContainer starts a Redis container for testing
func setupRedisContainer(t *testing.T) (testcontainers.Container, *redis.Client, context.Context) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port.Port()),
		Password:     "",
		DB:           0,
		PoolSize:     200,
		MinIdleConns: 50,
	})

	// Wait for Redis to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(ctx).Err(); err == nil {
			break
		}
		if i == maxRetries-1 {
			t.Fatalf("Redis container failed to become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	return redisContainer, client, ctx
}

// TestBasicRedisOperations tests basic Redis commands like SET, GET, DEL, EXISTS
func TestBasicRedisOperations(t *testing.T) {
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
			name:    "SET command",
			setup:   func() error { return nil },
			command: []interface{}{"SET", "testkey", "testvalue"},
			validate: func(result interface{}) error {
				if result != "OK" {
					return fmt.Errorf("expected 'OK', got %v", result)
				}
				return nil
			},
		},
		{
			name: "GET command",
			setup: func() error {
				return redisClient.Set(ctx, "getkey", "getvalue", 0).Err()
			},
			command: []interface{}{"GET", "getkey"},
			validate: func(result interface{}) error {
				if result != "getvalue" {
					return fmt.Errorf("expected 'getvalue', got %v", result)
				}
				return nil
			},
		},
		{
			name: "DEL command",
			setup: func() error {
				return redisClient.Set(ctx, "delkey", "delvalue", 0).Err()
			},
			command: []interface{}{"DEL", "delkey"},
			validate: func(result interface{}) error {
				if result != int64(1) {
					return fmt.Errorf("expected 1, got %v", result)
				}
				return nil
			},
		},
		{
			name: "EXISTS command - key exists",
			setup: func() error {
				return redisClient.Set(ctx, "existkey", "value", 0).Err()
			},
			command: []interface{}{"EXISTS", "existkey"},
			validate: func(result interface{}) error {
				if result != int64(1) {
					return fmt.Errorf("expected 1, got %v", result)
				}
				return nil
			},
		},
		{
			name:    "PING command",
			setup:   func() error { return nil },
			command: []interface{}{"PING"},
			validate: func(result interface{}) error {
				if result != "PONG" {
					return fmt.Errorf("expected 'PONG', got %v", result)
				}
				return nil
			},
		},
		{
			name: "INCR command",
			setup: func() error {
				return redisClient.Set(ctx, "counter", "10", 0).Err()
			},
			command: []interface{}{"INCR", "counter"},
			validate: func(result interface{}) error {
				if result != int64(11) {
					return fmt.Errorf("expected 11, got %v", result)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if err := tt.setup(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Execute command
			result, err := redisClient.Do(ctx, tt.command...).Result()
			if err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			// Validate result
			if err := tt.validate(result); err != nil {
				t.Errorf("Validation failed: %v", err)
			}
		})
	}
}

// TestRedisDataStructures tests Redis data structures (Lists, Hashes, Sets)
func TestRedisDataStructures(t *testing.T) {
	container, redisClient, ctx := setupRedisContainer(t)
	defer func() {
		redisClient.Close()
		container.Terminate(ctx)
	}()

	t.Run("List operations", func(t *testing.T) {
		// Test LPUSH
		result, err := redisClient.Do(ctx, "LPUSH", "mylist", "value1", "value2", "value3").Result()
		if err != nil {
			t.Fatalf("LPUSH failed: %v", err)
		}
		if result != int64(3) {
			t.Errorf("Expected LPUSH to return 3, got %v", result)
		}

		// Test LRANGE
		result, err = redisClient.Do(ctx, "LRANGE", "mylist", "0", "-1").Result()
		if err != nil {
			t.Fatalf("LRANGE failed: %v", err)
		}
		list, ok := result.([]interface{})
		if !ok || len(list) != 3 {
			t.Errorf("Expected list of length 3, got %v", result)
		}
	})

	t.Run("Hash operations", func(t *testing.T) {
		// Test HSET
		result, err := redisClient.Do(ctx, "HSET", "myhash", "field1", "value1").Result()
		if err != nil {
			t.Fatalf("HSET failed: %v", err)
		}
		if result != int64(1) {
			t.Errorf("Expected HSET to return 1, got %v", result)
		}

		// Test HGET
		result, err = redisClient.Do(ctx, "HGET", "myhash", "field1").Result()
		if err != nil {
			t.Fatalf("HGET failed: %v", err)
		}
		if result != "value1" {
			t.Errorf("Expected 'value1', got %v", result)
		}

		// Test HGETALL
		redisClient.Do(ctx, "HSET", "myhash", "field2", "value2")
		result, err = redisClient.Do(ctx, "HGETALL", "myhash").Result()
		if err != nil {
			t.Fatalf("HGETALL failed: %v", err)
		}
		fields, ok := result.([]interface{})
		if !ok || len(fields) < 4 {
			t.Errorf("Expected hash with at least 4 elements, got %v", result)
		}
	})

	t.Run("Set operations", func(t *testing.T) {
		// Test SADD
		result, err := redisClient.Do(ctx, "SADD", "myset", "member1", "member2", "member3").Result()
		if err != nil {
			t.Fatalf("SADD failed: %v", err)
		}
		if result != int64(3) {
			t.Errorf("Expected SADD to return 3, got %v", result)
		}

		// Test SMEMBERS
		result, err = redisClient.Do(ctx, "SMEMBERS", "myset").Result()
		if err != nil {
			t.Fatalf("SMEMBERS failed: %v", err)
		}
		members, ok := result.([]interface{})
		if !ok || len(members) != 3 {
			t.Errorf("Expected 3 members, got %v", result)
		}
	})
}

// TestConnectionAndAuthentication tests connection status and authentication
func TestConnectionAndAuthentication(t *testing.T) {
	t.Run("Connection status check", func(t *testing.T) {
		container, redisClient, ctx := setupRedisContainer(t)
		defer func() {
			redisClient.Close()
			container.Terminate(ctx)
		}()

		// Test successful connection using PING
		err := redisClient.Ping(ctx).Err()
		if err != nil {
			t.Errorf("Expected successful connection, got error: %v", err)
		}

		// Test connection status
		status := redisClient.Ping(ctx).Val()
		if status != "PONG" {
			t.Errorf("Expected PONG, got %s", status)
		}
	})

	t.Run("Multiple database support", func(t *testing.T) {
		container, _, ctx := setupRedisContainer(t)
		defer container.Terminate(ctx)

		host, _ := container.Host(ctx)
		port, _ := container.MappedPort(ctx, "6379")

		// Create clients for different databases
		client0 := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%s", host, port.Port()),
			DB:   0,
		})
		defer client0.Close()

		client1 := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%s", host, port.Port()),
			DB:   1,
		})
		defer client1.Close()

		// Set key in DB 0
		err := client0.Set(ctx, "dbkey", "db0value", 0).Err()
		if err != nil {
			t.Fatalf("Failed to set key in DB 0: %v", err)
		}

		// Set same key in DB 1 with different value
		err = client1.Set(ctx, "dbkey", "db1value", 0).Err()
		if err != nil {
			t.Fatalf("Failed to set key in DB 1: %v", err)
		}

		// Verify values are isolated
		val0, _ := client0.Get(ctx, "dbkey").Result()
		val1, _ := client1.Get(ctx, "dbkey").Result()

		if val0 != "db0value" {
			t.Errorf("DB 0: expected 'db0value', got '%s'", val0)
		}

		if val1 != "db1value" {
			t.Errorf("DB 1: expected 'db1value', got '%s'", val1)
		}
	})

	t.Run("Key expiration", func(t *testing.T) {
		container, redisClient, ctx := setupRedisContainer(t)
		defer func() {
			redisClient.Close()
			container.Terminate(ctx)
		}()

		// Set key with expiration
		result, err := redisClient.Do(ctx, "SETEX", "expkey", "2", "expvalue").Result()
		if err != nil {
			t.Fatalf("SETEX failed: %v", err)
		}
		if result != "OK" {
			t.Errorf("Expected 'OK', got %v", result)
		}

		// Check TTL
		ttlResult, err := redisClient.Do(ctx, "TTL", "expkey").Result()
		if err != nil {
			t.Fatalf("TTL failed: %v", err)
		}
		ttl, ok := ttlResult.(int64)
		if !ok || ttl <= 0 || ttl > 2 {
			t.Errorf("Expected TTL between 1-2, got %v", ttlResult)
		}

		// Wait for expiration
		time.Sleep(3 * time.Second)

		// Key should not exist
		existsResult, err := redisClient.Do(ctx, "EXISTS", "expkey").Result()
		if err != nil {
			t.Fatalf("EXISTS failed: %v", err)
		}
		if existsResult != int64(0) {
			t.Errorf("Expected key to be expired, EXISTS returned %v", existsResult)
		}
	})
}

// TestConfigValidation tests the configuration validation logic
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				Host:         "localhost",
				Port:         "6379",
				DB:           0,
				PoolSize:     100,
				MinIdleConns: 10,
			},
			expectErr: false,
		},
		{
			name: "Empty host",
			config: &Config{
				Host:         "",
				Port:         "6379",
				DB:           0,
				PoolSize:     100,
				MinIdleConns: 10,
			},
			expectErr: true,
		},
		{
			name: "Invalid DB - too high",
			config: &Config{
				Host:         "localhost",
				Port:         "6379",
				DB:           16,
				PoolSize:     100,
				MinIdleConns: 10,
			},
			expectErr: true,
		},
		{
			name: "Invalid DB - negative",
			config: &Config{
				Host:         "localhost",
				Port:         "6379",
				DB:           -1,
				PoolSize:     100,
				MinIdleConns: 10,
			},
			expectErr: true,
		},
		{
			name: "Invalid pool size",
			config: &Config{
				Host:         "localhost",
				Port:         "6379",
				DB:           0,
				PoolSize:     0,
				MinIdleConns: 10,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkRedisSet(b *testing.B) {
	container, redisClient, ctx := setupRedisContainerForBench(b)
	defer func() {
		redisClient.Close()
		container.Terminate(ctx)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("benchkey:%d", i)
		redisClient.Set(ctx, key, "benchvalue", 0)
	}
}

func BenchmarkRedisGet(b *testing.B) {
	container, redisClient, ctx := setupRedisContainerForBench(b)
	defer func() {
		redisClient.Close()
		container.Terminate(ctx)
	}()

	// Pre-populate data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("benchkey:%d", i)
		redisClient.Set(ctx, key, "benchvalue", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("benchkey:%d", i%1000)
		redisClient.Get(ctx, key)
	}
}

func BenchmarkRedisPipeline(b *testing.B) {
	container, redisClient, ctx := setupRedisContainerForBench(b)
	defer func() {
		redisClient.Close()
		container.Terminate(ctx)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipe := redisClient.Pipeline()
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("pipekey:%d:%d", i, j)
			pipe.Set(ctx, key, "pipevalue", 0)
		}
		pipe.Exec(ctx)
	}
}

// Helper function for benchmarks
func setupRedisContainerForBench(b *testing.B) (testcontainers.Container, *redis.Client, context.Context) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		b.Fatalf("Failed to start Redis container: %v", err)
	}

	host, err := redisContainer.Host(ctx)
	if err != nil {
		b.Fatalf("Failed to get container host: %v", err)
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		b.Fatalf("Failed to get container port: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port.Port()),
		Password:     "",
		DB:           0,
		PoolSize:     200,
		MinIdleConns: 50,
	})

	// Wait for Redis to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(ctx).Err(); err == nil {
			break
		}
		if i == maxRetries-1 {
			b.Fatalf("Redis container failed to become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	return redisContainer, client, ctx
}
