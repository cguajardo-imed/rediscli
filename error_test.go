package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupErrorRedisContainer starts a dedicated miniredis instance for error tests.
func setupErrorRedisContainer(t *testing.T) (*miniredis.Miniredis, *redis.Client, context.Context) {
	t.Helper()
	testCtx := context.Background()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	// Suppress Redis client library warnings
	redis.SetLogger(&discardLogger{})

	redisClient := redis.NewClient(&redis.Options{
		Addr:     mr.Addr(),
		Password: "",
		DB:       0,
	})

	return mr, redisClient, testCtx
}

// ───────────────────────────────────────────────────────────────────────────
// Happy-path tests
// ───────────────────────────────────────────────────────────────────────────

// TestCreateErrorRecords_SingleError verifies that a single error record is
// correctly stored in Redis with the documented key format.
func TestCreateErrorRecords_SingleError(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "abc123def456"
	errorData := []ErrorData{
		{
			Message:   "Database connection failed",
			Code:      "DB_CONN_ERR",
			Timestamp: "2024-01-15T10:30:00Z",
			Details:   "Unable to connect to primary database",
		},
	}

	err := CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	// Always returns nil per documented behavior
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Verify key format: ERRORS:<fingerprint>:<message>
	expectedKey := fmt.Sprintf("ERRORS:%s:%s", fingerprint, errorData[0].Message)
	exists, checkErr := redisClient.Exists(redisCtx, expectedKey).Result()
	if checkErr != nil {
		t.Fatalf("Failed to check key existence: %v", checkErr)
	}
	if exists != 1 {
		t.Errorf("Expected key %q to exist", expectedKey)
	}

	// Verify stored value is valid JSON matching ErrorData
	raw, getErr := redisClient.Get(redisCtx, expectedKey).Result()
	if getErr != nil {
		t.Fatalf("Failed to GET error key: %v", getErr)
	}

	var stored ErrorData
	if unmarshalErr := json.Unmarshal([]byte(raw), &stored); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal stored JSON: %v", unmarshalErr)
	}

	if stored.Message != errorData[0].Message {
		t.Errorf("Message mismatch: got %q, want %q", stored.Message, errorData[0].Message)
	}
	if stored.Code != errorData[0].Code {
		t.Errorf("Code mismatch: got %q, want %q", stored.Code, errorData[0].Code)
	}
	if stored.Timestamp != errorData[0].Timestamp {
		t.Errorf("Timestamp mismatch: got %q, want %q", stored.Timestamp, errorData[0].Timestamp)
	}
	if stored.Details != errorData[0].Details {
		t.Errorf("Details mismatch: got %q, want %q", stored.Details, errorData[0].Details)
	}
}

// TestCreateErrorRecords_MultipleErrors verifies that multiple error records
// are all stored correctly.
func TestCreateErrorRecords_MultipleErrors(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "fingerprint123"
	errorData := []ErrorData{
		{Message: "Error 1", Code: "E001"},
		{Message: "Error 2", Code: "E002"},
		{Message: "Error 3", Code: "E003"},
	}

	err := CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Verify all keys exist
	for _, ed := range errorData {
		key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, ed.Message)
		exists, _ := redisClient.Exists(redisCtx, key).Result()
		if exists != 1 {
			t.Errorf("Expected key %q to exist", key)
		}

		// Verify payload
		raw, _ := redisClient.Get(redisCtx, key).Result()
		var stored ErrorData
		json.Unmarshal([]byte(raw), &stored)
		if stored.Message != ed.Message {
			t.Errorf("Message mismatch for key %q", key)
		}
		if stored.Code != ed.Code {
			t.Errorf("Code mismatch for key %q", key)
		}
	}
}

// TestCreateErrorRecords_EmptyArray verifies that passing an empty array is a
// no-op and returns nil.
func TestCreateErrorRecords_EmptyArray(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "fp123"
	errorData := []ErrorData{}

	err := CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)
	if err != nil {
		t.Errorf("Expected nil error for empty array, got %v", err)
	}

	// Verify no ERRORS:* keys were created
	keys := scanErrorKeys(t, redisClient, redisCtx, "ERRORS:*")
	if len(keys) != 0 {
		t.Errorf("Expected 0 error keys, got %d", len(keys))
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Key format tests
// ───────────────────────────────────────────────────────────────────────────

// TestCreateErrorRecords_KeyFormat verifies the exact key format matches
// the documented spec: ERRORS:<fingerprint>:<message>
func TestCreateErrorRecords_KeyFormat(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "test-fingerprint-42"
	message := "Test error message"
	errorData := []ErrorData{
		{Message: message},
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	expectedKey := fmt.Sprintf("ERRORS:%s:%s", fingerprint, message)
	exists, _ := redisClient.Exists(redisCtx, expectedKey).Result()
	if exists != 1 {
		t.Errorf("Expected key %q to exist with exact format", expectedKey)
	}

	// Verify the key starts with "ERRORS:"
	keys := scanErrorKeys(t, redisClient, redisCtx, "ERRORS:*")
	if len(keys) != 1 {
		t.Fatalf("Expected 1 error key, got %d", len(keys))
	}
	if !strings.HasPrefix(keys[0], "ERRORS:") {
		t.Errorf("Key %q does not start with 'ERRORS:'", keys[0])
	}

	// Verify the key contains the fingerprint
	if !strings.Contains(keys[0], fingerprint) {
		t.Errorf("Key %q does not contain fingerprint %q", keys[0], fingerprint)
	}

	// Verify the key contains the message
	if !strings.Contains(keys[0], message) {
		t.Errorf("Key %q does not contain message %q", keys[0], message)
	}
}

// TestCreateErrorRecords_SameFingerprintSameMessage verifies that identical
// messages under the same fingerprint overwrite (map to the same key).
func TestCreateErrorRecords_SameFingerprintSameMessage(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "fp-duplicate-test"
	errorData := []ErrorData{
		{Message: "Duplicate error", Code: "E001", Details: "First occurrence"},
		{Message: "Duplicate error", Code: "E001", Details: "Second occurrence"},
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	// Should have only 1 key since message+fingerprint are identical
	expectedKey := fmt.Sprintf("ERRORS:%s:%s", fingerprint, errorData[0].Message)
	keys := scanErrorKeys(t, redisClient, redisCtx, "ERRORS:*")
	if len(keys) != 1 {
		t.Errorf("Expected 1 key (duplicate should overwrite), got %d", len(keys))
	}

	// Verify the stored value is the last one written
	raw, _ := redisClient.Get(redisCtx, expectedKey).Result()
	var stored ErrorData
	json.Unmarshal([]byte(raw), &stored)
	if stored.Details != "Second occurrence" {
		t.Errorf("Expected last write to win, got details: %q", stored.Details)
	}
}

// TestCreateErrorRecords_DifferentFingerprints verifies that the same message
// under different fingerprints creates distinct keys.
func TestCreateErrorRecords_DifferentFingerprints(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	message := "Common error"
	fp1 := "fingerprint-A"
	fp2 := "fingerprint-B"

	CreateErrorRecordsWithClient(redisClient, redisCtx, []ErrorData{{Message: message}}, fp1)
	CreateErrorRecordsWithClient(redisClient, redisCtx, []ErrorData{{Message: message}}, fp2)

	// Should have 2 distinct keys
	keys := scanErrorKeys(t, redisClient, redisCtx, "ERRORS:*")
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys (different fingerprints), got %d", len(keys))
	}

	// Verify both keys exist
	key1 := fmt.Sprintf("ERRORS:%s:%s", fp1, message)
	key2 := fmt.Sprintf("ERRORS:%s:%s", fp2, message)

	exists1, _ := redisClient.Exists(redisCtx, key1).Result()
	exists2, _ := redisClient.Exists(redisCtx, key2).Result()

	if exists1 != 1 {
		t.Errorf("Expected key %q to exist", key1)
	}
	if exists2 != 1 {
		t.Errorf("Expected key %q to exist", key2)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Best-effort behavior tests
// ───────────────────────────────────────────────────────────────────────────

// TestCreateErrorRecords_AlwaysReturnsNil verifies that the function always
// returns nil per documented behavior, even if Redis operations fail.
func TestCreateErrorRecords_AlwaysReturnsNil(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	errorData := []ErrorData{
		{Message: "Test error"},
	}

	// Even with a closed connection, should return nil (best-effort)
	mr.Close()
	redisClient.Close()

	err := CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, "fp")
	if err != nil {
		t.Errorf("Expected nil even on Redis failure (best-effort), got %v", err)
	}
}

// TestCreateErrorRecords_NoTTL confirms that error records have no expiry.
func TestCreateErrorRecords_NoTTL(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "fp-ttl-test"
	errorData := []ErrorData{
		{Message: "Persistent error"},
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, errorData[0].Message)
	ttl, err := redisClient.TTL(redisCtx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get TTL: %v", err)
	}

	// TTL == -1 means no expiry
	if ttl != -1*time.Nanosecond {
		t.Errorf("Expected TTL == -1 (no expiry), got %v", ttl)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// JSON payload tests
// ───────────────────────────────────────────────────────────────────────────

// TestCreateErrorRecords_JSONFields verifies that all ErrorData fields are
// correctly serialized to JSON.
func TestCreateErrorRecords_JSONFields(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "json-test"
	errorData := []ErrorData{
		{
			Message:   "Full error data",
			Code:      "ERR_FULL",
			Timestamp: "2024-01-15T12:00:00Z",
			Details:   "Complete error information",
		},
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, errorData[0].Message)
	raw, _ := redisClient.Get(redisCtx, key).Result()

	// Verify JSON is valid
	var stored ErrorData
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("Stored value is not valid JSON: %v", err)
	}

	// Verify all fields
	if stored.Message != errorData[0].Message {
		t.Errorf("Message: got %q, want %q", stored.Message, errorData[0].Message)
	}
	if stored.Code != errorData[0].Code {
		t.Errorf("Code: got %q, want %q", stored.Code, errorData[0].Code)
	}
	if stored.Timestamp != errorData[0].Timestamp {
		t.Errorf("Timestamp: got %q, want %q", stored.Timestamp, errorData[0].Timestamp)
	}
	if stored.Details != errorData[0].Details {
		t.Errorf("Details: got %q, want %q", stored.Details, errorData[0].Details)
	}
}

// TestCreateErrorRecords_OptionalFields verifies that omitempty works for
// optional fields (Code, Timestamp, Details).
func TestCreateErrorRecords_OptionalFields(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "optional-test"
	errorData := []ErrorData{
		{Message: "Minimal error"}, // Only required field
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, errorData[0].Message)
	raw, _ := redisClient.Get(redisCtx, key).Result()

	// Verify optional fields are omitted from JSON when empty
	if strings.Contains(raw, `"code"`) {
		t.Error("Expected omitted 'code' field in JSON")
	}
	if strings.Contains(raw, `"timestamp"`) {
		t.Error("Expected omitted 'timestamp' field in JSON")
	}
	if strings.Contains(raw, `"details"`) {
		t.Error("Expected omitted 'details' field in JSON")
	}

	// Message should always be present
	if !strings.Contains(raw, `"message"`) {
		t.Error("Expected 'message' field in JSON")
	}
}

// TestCreateErrorRecords_SpecialCharactersInMessage verifies that special
// characters in the message are handled correctly in both key and value.
func TestCreateErrorRecords_SpecialCharactersInMessage(t *testing.T) {
	mr, redisClient, redisCtx := setupErrorRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	fingerprint := "special-chars"
	errorData := []ErrorData{
		{Message: "Error: connection failed (timeout) at 10:30:45"},
		{Message: "Validation failed: field 'name' is required"},
	}

	CreateErrorRecordsWithClient(redisClient, redisCtx, errorData, fingerprint)

	// Verify both keys exist with special characters preserved
	for _, ed := range errorData {
		key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, ed.Message)
		exists, _ := redisClient.Exists(redisCtx, key).Result()
		if exists != 1 {
			t.Errorf("Expected key with special chars to exist: %q", key)
		}

		raw, _ := redisClient.Get(redisCtx, key).Result()
		var stored ErrorData
		json.Unmarshal([]byte(raw), &stored)
		if stored.Message != ed.Message {
			t.Errorf("Message with special chars mismatch: got %q, want %q", stored.Message, ed.Message)
		}
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Helper functions
// ───────────────────────────────────────────────────────────────────────────

// scanErrorKeys returns all error keys matching pattern from Redis.
func scanErrorKeys(t *testing.T, redisClient *redis.Client, redisCtx context.Context, pattern string) []string {
	t.Helper()
	var keys []string
	iter := redisClient.Scan(redisCtx, 0, pattern, 0).Iterator()
	for iter.Next(redisCtx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("SCAN error: %v", err)
	}
	return keys
}
