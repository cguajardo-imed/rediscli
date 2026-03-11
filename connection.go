package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	clientMutex sync.RWMutex
)

// discardLogger implements the Redis internal.Logging interface but discards all logs.
// This is used to suppress Redis client library warnings that are not relevant to the application,
// such as "auto mode fallback: maint_notifications disabled due to handshake error".
// These warnings occur when the Redis server doesn't support certain optional features,
// but they don't affect the application's functionality.
type discardLogger struct{}

func (d *discardLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	// Discard all Redis library logs to suppress harmless warnings
}

func initConnection() {
	config, err := LoadConfig()
	if err != nil {
		LogError("Failed to load configuration: " + err.Error())
		panic(err)
	}

	if err := config.Validate(); err != nil {
		LogError("Invalid configuration: " + err.Error())
		panic(err)
	}

	ctx = context.Background()

	// Suppress Redis library warnings (like maint_notifications errors)
	redis.SetLogger(&discardLogger{})

	client = redis.NewClient(&redis.Options{
		Addr:         config.Address(),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
	})

	LogSuccess("Redis connection initialized on " + config.Address())
	LogConfig(config)
}

func connectionStatus() bool {
	redisPingResponse := client.Ping(ctx)
	LogDebug(client.Conn().String())
	if redisPingResponse.Val() != "PONG" {
		errorString, err := redisPingResponse.Result()
		if err != nil {
			LogError("Error trying to connect with Redis: " + err.Error())
		} else {
			LogError("Error trying to connect with Redis: " + errorString)
		}
		return false
	}
	LogSuccess("Redis connection is healthy")
	return true
}

// reconnect attempts to reconnect to Redis
func reconnect() error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	LogInfo("Attempting to reconnect to Redis...")

	// Close existing connection if any
	if client != nil {
		client.Close()
	}

	// Re-initialize the connection
	initConnection()

	// Test the new connection
	if !connectionStatus() {
		return fmt.Errorf("failed to reconnect to Redis")
	}

	LogSuccess("Successfully reconnected to Redis")
	return nil
}

// healthCheck performs a periodic health check on the Redis connection
func healthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := client.Ping(ctx).Err(); err != nil {
			LogWarning("Redis health check failed: " + err.Error())
			if err := reconnect(); err != nil {
				LogError("Reconnection failed: " + err.Error())
			}
		}
	}
}

// getClientInfo returns information about the Redis connection
func getClientInfo() map[string]string {
	info := make(map[string]string)
	info["addr"] = client.Options().Addr
	info["db"] = fmt.Sprintf("%d", client.Options().DB)
	info["pool_size"] = fmt.Sprintf("%d", client.Options().PoolSize)
	return info
}

// parseCommand parses command-line arguments into a slice for Redis Do
func parseCommand(args []string) []any {
	result := make([]any, len(args))
	for i, arg := range args {
		result[i] = arg
	}
	return result
}

// executeCommand executes a Redis command from command-line arguments
func executeCommand(args []string) bool {
	if len(args) == 0 {
		fmt.Println("Error: No command provided")
		return false
	}

	// Convert args to interface slice
	cmdArgs := parseCommand(args)

	// Send command to Redis using Do
	res, err := client.Do(ctx, cmdArgs...).Result()
	if err != nil {
		fmt.Printf("Redis command error: %v\n", err)
		return false
	}

	// Print the result based on its type for better output
	printResult(res)
	return true
}

// printResult formats and prints Redis command results
func printResult(res any) {
	switch val := res.(type) {
	case string:
		fmt.Println(val)
	case []byte:
		fmt.Println(string(val))
	case []any:
		if len(val) == 0 {
			fmt.Println("(empty array)")
		} else {
			for i, v := range val {
				fmt.Printf("%d) %v\n", i+1, v)
			}
		}
	case int64:
		fmt.Printf("(integer) %d\n", val)
	case nil:
		fmt.Println("(nil)")
	default:
		fmt.Printf("%v\n", val)
	}
}

func fakeRecord() (string, string) {
	return fakeRecordWithIteration(0, 1)
}

func fakeRecordWithIteration(iteration, total int) (string, string) {
	return fakeRecordWithIterationAndParams("", "", "", iteration, total)
}

func fakeRecordWithIterationAndParams(placeCode, serviceName, customParams string, iteration, total int) (string, string) {
	// COUNTRY_CODE:PLACE_CODE:SERVICE_NAME:CUSTOM_PARAMS:PARENT_MESSAGE_UUID

	// Default values
	if placeCode == "" {
		placeCode = "0"
	}
	if serviceName == "" {
		serviceName = "demo"
	}

	// UUIDs remain constant as per requirements
	parentUUID := "15130809-cd02-450a-909e-4f33d06d0397"

	// Build key: COUNTRY_CODE:PLACE_CODE:SERVICE_NAME:CUSTOM_PARAMS:PARENT_MESSAGE_UUID
	key := fmt.Sprintf("cl:%s:%s:%s:%s", placeCode, serviceName, customParams, parentUUID)
	value := parentUUID

	messageContent := fmt.Sprintf("This is a test message %s", time.Now().Format(time.UnixDate))
	createdAt := time.Now().Format("2006-01-02T15:04:05Z07:00")

	fullContent := fmt.Sprintf(`{"criticality":"low","title":"test","messages":[{"uuid":"12d8254b-f557-49fc-a665-98762d268a5d","content":"\u003cp\u003e%s\u003c/p\u003e","plain_text":"%s\n","created_at":"2026-02-12T14:29:44.896Z"}],"action":"","type":"alert","id":"15130809-cd02-450a-909e-4f33d06d0397","status":"pending","created_at":"%s"}`,
		messageContent, messageContent, createdAt,
	)

	duration := time.Minute
	err := client.Set(ctx, key, value, duration).Err()
	if err != nil {
		LogRedisError("set", key, err, iteration, total)
	}
	err = client.Set(ctx, value, fullContent, duration).Err()
	if err != nil {
		LogRedisError("set full content for", value, err, iteration, total)
	} else {
		LogRedisOperation("create", key, "", iteration, total)
	}
	return key, serviceName
}

func publishRecord(key, channel string) {
	publishRecordWithIteration(key, channel, 0, 1)
}

func publishRecordWithIteration(key, channel string, iteration, total int) {
	err := client.Publish(ctx, channel, key).Err()
	if err != nil {
		LogRedisError("publish", key, err, iteration, total)
	} else {
		LogRedisOperation("publish", key, channel, iteration, total)
	}
}

// getStats returns Redis server statistics
func getStats() (map[string]string, error) {
	info, err := client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]string)
	lines := strings.SplitSeq(info, "\r\n")
	for line := range lines {
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				stats[parts[0]] = parts[1]
			}
		}
	}
	return stats, nil
}

// flushDB flushes the current database
func flushDB() error {
	return client.FlushDB(ctx).Err()
}

// subscribe subscribes to Redis channels and processes messages
func subscribe(channels []string, handler func(channel, message string)) error {
	pubsub := client.Subscribe(ctx, channels...)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		handler(msg.Channel, msg.Payload)
	}

	return nil
}

// batchSet sets multiple key-value pairs in a single pipeline
func batchSet(pairs map[string]string, expiration time.Duration) error {
	pipe := client.Pipeline()

	for key, value := range pairs {
		pipe.Set(ctx, key, value, expiration)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// getKeysByPattern retrieves all keys matching a pattern
func getKeysByPattern(pattern string) ([]string, error) {
	var keys []string
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}
