package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
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

func (d *discardLogger) Printf(ctx context.Context, format string, v ...any) {
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

	record := NotificationRecord{
		Criticality: "low",
		Title:       "test",
		Messages: []NotificationMessage{
			{
				UUID:      "12d8254b-f557-49fc-a665-98762d268a5d",
				Content:   fmt.Sprintf("<p>%s</p>", messageContent),
				PlainText: messageContent + "\n",
				CreatedAt: "2026-02-12T14:29:44.896Z",
			},
		},
		Action:    "",
		Type:      "alert",
		ID:        parentUUID,
		Status:    "pending",
		CreatedAt: createdAt,
	}

	recordBytes, err := json.Marshal(record)
	if err != nil {
		LogError(fmt.Sprintf("failed to marshal notification record: %v", err))
		return key, ""
	}
	fullContent := string(recordBytes)

	duration := time.Minute
	err = client.Set(ctx, key, value, duration).Err()
	if err != nil {
		LogRedisError("set", key, err, iteration, total)
	}
	err = client.Set(ctx, value, fullContent, duration).Err()
	if err != nil {
		LogRedisError("set full content for", value, err, iteration, total)
	} else {
		LogRedisOperation("create", key, "", iteration, total)
	}
	return key, fullContent
}

func publishRecord(key string, value string) {
	publishRecordWithIteration(ChannelValue{Key: key, Value: value}, 0, 1)
}

type ChannelValue struct {
	Key   string
	Value string
}

func publishRecordWithIteration(cv ChannelValue, iteration, total int) {
	const notificationsChannel = "gns_notifications_channel"channel"
	data, err := json.Marshal(cv)
	if err != nil {
		LogRedisError("publish", cv.Key, err, iteration, total)
		return
	}
	err = client.Publish(ctx, notificationsChannel, data).Err()
	if err != nil {
		LogRedisError("publish", cv.Key, err, iteration, total)
	} else {
		LogRedisOperation("publish", cv.Key, notificationsChannel, iteration, total)
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

// ackRecord represents the JSON payload stored under an ack:* key.
type ackRecord struct {
	Action      string `json:"action"`
	UUID        string `json:"uuid"`
	Date        string `json:"date"`
	Fingerprint string `json:"fingerprint"`
}

// generateAckRecords creates `count` ack records in Redis.
// action must be "receive" or "read".
// It looks up existing notification UUIDs (values stored under cl:* keys)
// and picks them round-robin so every generated ack points to a real UUID.
// If no notification keys exist the function returns an error.
func generateAckRecords(action string, count int, iteration, total int) (int, error) {
	return generateAckRecordsWithClient(client, ctx, action, count, iteration, total)
}

// generateAckRecordsWithClient is the testable core of generateAckRecords.
// It accepts explicit redis client and context so tests can inject a containerised instance.
func generateAckRecordsWithClient(redisClient *redis.Client, redisCtx context.Context, action string, count int, iteration, total int) (int, error) {
	// Collect candidate UUIDs from existing notification keys.
	// The fakeRecordWithIterationAndParams function stores keys like
	//   cl:<placeCode>:<serviceName>:<customParams>:<parentUUID>
	// whose value IS the parentUUID, and a second key equal to the parentUUID
	// whose value is the full JSON payload.
	// We scan for UUID-shaped keys (the parentUUID keys) directly.
	var uuids []string
	iter := redisClient.Scan(redisCtx, 0, "*-*-*-*-*", 0).Iterator()
	for iter.Next(redisCtx) {
		uuids = append(uuids, iter.Val())
	}

	if len(uuids) > 0 {
		LogInfo(fmt.Sprintf("Found %d notification UUID(s) for ack generation", len(uuids)))
	}

	if len(uuids) == 0 {
		// Fallback: scan cl:* keys and read their values
		var clKeys []string
		clIter := redisClient.Scan(redisCtx, 0, "cl:*", 0).Iterator()
		for clIter.Next(redisCtx) {
			clKeys = append(clKeys, clIter.Val())
		}
		if clErr := clIter.Err(); clErr != nil {
			return 0, fmt.Errorf("failed to scan Redis keys: %w", clErr)
		}
		if len(clKeys) == 0 {
			return 0, fmt.Errorf("no notification records found in Redis; please create some first")
		}
		for _, k := range clKeys {
			val, vErr := redisClient.Get(redisCtx, k).Result()
			if vErr == nil && val != "" {
				uuids = append(uuids, val)
			}
		}
		if len(uuids) > 0 {
			LogInfo(fmt.Sprintf("Found %d notification UUID(s) via cl:* key fallback", len(uuids)))
		}
		if len(uuids) == 0 {
			return 0, fmt.Errorf("no notification UUIDs found in Redis; please create some first")
		}
	}

	created := 0
	for i := range count {
		notifUUID := uuids[i%len(uuids)]

		// Build a random fingerprint (96-hex-char string).
		fp := generateFingerprint()
		rec := ackRecord{
			Action:      action,
			UUID:        notifUUID,
			Date:        time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			Fingerprint: fp,
		}

		payload, jsonErr := json.Marshal(rec)
		if jsonErr != nil {
			LogError(fmt.Sprintf("ack[%d]: failed to marshal JSON: %v", i+1, jsonErr))
			continue
		}

		// Key = ack:<md5(payload)>
		hash := md5.Sum(payload)
		key := fmt.Sprintf("ack:%x", hash)

		setErr := redisClient.Set(redisCtx, key, string(payload), 0).Err()
		if setErr != nil {
			LogRedisError("set ack", key, setErr, iteration, total)
			fmt.Printf("ERROR ACK Record: %s\n\r\n\r", setErr.Error())
		} else {
			LogRedisOperation("create ack", key, "", iteration, total)
			fmt.Printf("ACK Record: %s \n\r %+v \n\r\n\r", key, rec)
			created++
		}
	}

	return created, nil
}
