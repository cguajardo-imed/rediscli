package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testClient *redis.Client
var testCtx context.Context

// setupRedisContainer starts a Redis container for testing
func setupRedisContainer(t *testing.T) (testcontainers.Container, *redis.Client) {
	testCtx = context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(testCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	host, err := redisContainer.Host(testCtx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := redisContainer.MappedPort(testCtx, "6379")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port.Port()),
		Password: "",
		DB:       0,
	})

	// Wait for Redis to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(testCtx).Err(); err == nil {
			break
		}
		if i == maxRetries-1 {
			t.Fatalf("Redis container failed to become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	return redisContainer, client
}

func TestConnectionStatus(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Set global variables for the function to use
	ctx = testCtx
	client = redisClient

	// Test successful connection
	if !ConnectionStatus() {
		t.Error("ConnectionStatus() should return true for valid connection")
	}

	// Test with closed connection
	redisClient.Close()
	if ConnectionStatus() {
		t.Error("ConnectionStatus() should return false for closed connection")
	}
}

func TestRedisSetAndGet(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Test SET command
	err := redisClient.Set(testCtx, "test-key", "test-value", 0).Err()
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	// Test GET command
	val, err := redisClient.Get(testCtx, "test-key").Result()
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if val != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", val)
	}
}

func TestRedisDoCommand(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	tests := []struct {
		name     string
		setup    func()
		command  []interface{}
		wantErr  bool
		validate func(result interface{}) bool
	}{
		{
			name:    "SET command",
			setup:   func() {},
			command: []interface{}{"SET", "mykey", "myvalue"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == "OK"
			},
		},
		{
			name: "GET command",
			setup: func() {
				redisClient.Set(testCtx, "getkey", "getvalue", 0)
			},
			command: []interface{}{"GET", "getkey"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == "getvalue"
			},
		},
		{
			name: "DEL command",
			setup: func() {
				redisClient.Set(testCtx, "delkey", "delvalue", 0)
			},
			command: []interface{}{"DEL", "delkey"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == int64(1)
			},
		},
		{
			name:    "EXISTS command - key exists",
			setup:   func() { redisClient.Set(testCtx, "existkey", "value", 0) },
			command: []interface{}{"EXISTS", "existkey"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == int64(1)
			},
		},
		{
			name:    "EXISTS command - key doesn't exist",
			setup:   func() {},
			command: []interface{}{"EXISTS", "nonexistent"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == int64(0)
			},
		},
		{
			name: "KEYS command",
			setup: func() {
				redisClient.Set(testCtx, "key1", "val1", 0)
				redisClient.Set(testCtx, "key2", "val2", 0)
			},
			command: []interface{}{"KEYS", "key*"},
			wantErr: false,
			validate: func(result interface{}) bool {
				keys, ok := result.([]interface{})
				return ok && len(keys) >= 2
			},
		},
		{
			name: "INCR command",
			setup: func() {
				redisClient.Set(testCtx, "counter", "10", 0)
			},
			command: []interface{}{"INCR", "counter"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == int64(11)
			},
		},
		{
			name:    "PING command",
			setup:   func() {},
			command: []interface{}{"PING"},
			wantErr: false,
			validate: func(result interface{}) bool {
				return result == "PONG"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setup != nil {
				tt.setup()
			}

			// Execute command
			result, err := redisClient.Do(testCtx, tt.command...).Result()

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Validate result
			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("Result validation failed. Got: %v (type: %T)", result, result)
			}

			// Cleanup
			if tt.name == "SET command" || tt.name == "GET command" {
				redisClient.Del(testCtx, "mykey", "getkey")
			}
		})
	}
}

func TestRedisList(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Test LPUSH
	err := redisClient.LPush(testCtx, "mylist", "value1", "value2", "value3").Err()
	if err != nil {
		t.Fatalf("LPUSH failed: %v", err)
	}

	// Test LRANGE using Do
	result, err := redisClient.Do(testCtx, "LRANGE", "mylist", "0", "-1").Result()
	if err != nil {
		t.Fatalf("LRANGE failed: %v", err)
	}

	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(list) != 3 {
		t.Errorf("Expected list length 3, got %d", len(list))
	}
}

func TestRedisHash(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Test HSET
	result, err := redisClient.Do(testCtx, "HSET", "myhash", "field1", "value1").Result()
	if err != nil {
		t.Fatalf("HSET failed: %v", err)
	}

	if result != int64(1) {
		t.Errorf("Expected HSET to return 1, got %v", result)
	}

	// Test HGET
	result, err = redisClient.Do(testCtx, "HGET", "myhash", "field1").Result()
	if err != nil {
		t.Fatalf("HGET failed: %v", err)
	}

	if result != "value1" {
		t.Errorf("Expected 'value1', got %v", result)
	}

	// Test HGETALL
	redisClient.Do(testCtx, "HSET", "myhash", "field2", "value2")
	result, err = redisClient.Do(testCtx, "HGETALL", "myhash").Result()
	if err != nil {
		t.Fatalf("HGETALL failed: %v", err)
	}

	fields, ok := result.([]interface{})
	if !ok || len(fields) < 4 {
		t.Errorf("Expected hash with multiple fields, got %v", result)
	}
}

func TestRedisSet(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Test SADD
	result, err := redisClient.Do(testCtx, "SADD", "myset", "member1", "member2", "member3").Result()
	if err != nil {
		t.Fatalf("SADD failed: %v", err)
	}

	if result != int64(3) {
		t.Errorf("Expected SADD to return 3, got %v", result)
	}

	// Test SMEMBERS
	result, err = redisClient.Do(testCtx, "SMEMBERS", "myset").Result()
	if err != nil {
		t.Fatalf("SMEMBERS failed: %v", err)
	}

	members, ok := result.([]interface{})
	if !ok || len(members) != 3 {
		t.Errorf("Expected 3 members, got %v", result)
	}
}

func TestRedisExpiration(t *testing.T) {
	container, redisClient := setupRedisContainer(t)
	defer container.Terminate(testCtx)
	defer redisClient.Close()

	// Set key with expiration
	result, err := redisClient.Do(testCtx, "SETEX", "expkey", "2", "expvalue").Result()
	if err != nil {
		t.Fatalf("SETEX failed: %v", err)
	}

	if result != "OK" {
		t.Errorf("Expected 'OK', got %v", result)
	}

	// Check TTL
	ttlResult, err := redisClient.Do(testCtx, "TTL", "expkey").Result()
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
	existsResult, err := redisClient.Do(testCtx, "EXISTS", "expkey").Result()
	if err != nil {
		t.Fatalf("EXISTS failed: %v", err)
	}

	if existsResult != int64(0) {
		t.Errorf("Expected key to be expired, EXISTS returned %v", existsResult)
	}
}

func TestRedisWithPassword(t *testing.T) {
	testCtx := context.Background()
	password := "testpassword123"

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		Cmd:          []string{"redis-server", "--requirepass", password},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(testCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer redisContainer.Terminate(testCtx)

	host, _ := redisContainer.Host(testCtx)
	port, _ := redisContainer.MappedPort(testCtx, "6379")

	// Test connection without password (should fail)
	clientNoAuth := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	defer clientNoAuth.Close()

	err = clientNoAuth.Ping(testCtx).Err()
	if err == nil {
		t.Error("Expected error when connecting without password")
	}

	// Test connection with password (should succeed)
	clientWithAuth := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port.Port()),
		Password: password,
	})
	defer clientWithAuth.Close()

	time.Sleep(500 * time.Millisecond)
	err = clientWithAuth.Ping(testCtx).Err()
	if err != nil {
		t.Errorf("Expected successful connection with password, got error: %v", err)
	}
}

func TestRedisMultipleDB(t *testing.T) {
	container, _ := setupRedisContainer(t)
	defer container.Terminate(testCtx)

	host, _ := container.Host(testCtx)
	port, _ := container.MappedPort(testCtx, "6379")

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
	err := client0.Set(testCtx, "dbkey", "db0value", 0).Err()
	if err != nil {
		t.Fatalf("Failed to set key in DB 0: %v", err)
	}

	// Set same key in DB 1 with different value
	err = client1.Set(testCtx, "dbkey", "db1value", 0).Err()
	if err != nil {
		t.Fatalf("Failed to set key in DB 1: %v", err)
	}

	// Verify values are isolated
	val0, _ := client0.Get(testCtx, "dbkey").Result()
	val1, _ := client1.Get(testCtx, "dbkey").Result()

	if val0 != "db0value" {
		t.Errorf("DB 0: expected 'db0value', got '%s'", val0)
	}

	if val1 != "db1value" {
		t.Errorf("DB 1: expected 'db1value', got '%s'", val1)
	}
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Exit
	os.Exit(code)
}
