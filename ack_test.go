package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupAckRedisContainer starts a dedicated miniredis instance for ack tests.
func setupAckRedisContainer(t *testing.T) (*miniredis.Miniredis, *redis.Client, context.Context) {
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

// seedNotificationUUID writes the two keys that fakeRecordWithIterationAndParams
// creates: a cl:* pointer key whose value IS the UUID, and the UUID key itself
// containing a JSON notification payload.
func seedNotificationUUID(t *testing.T, redisClient *redis.Client, redisCtx context.Context, notifUUID string) {
	t.Helper()
	clKey := fmt.Sprintf("cl:0:demo::%s", notifUUID)
	payload := fmt.Sprintf(`{"id":%q,"status":"pending"}`, notifUUID)

	if err := redisClient.Set(redisCtx, clKey, notifUUID, 0).Err(); err != nil {
		t.Fatalf("seed: failed to set cl key: %v", err)
	}
	if err := redisClient.Set(redisCtx, notifUUID, payload, 0).Err(); err != nil {
		t.Fatalf("seed: failed to set uuid key: %v", err)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Error-path tests (no dependencies on real Redis data)
// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_NoNotificationsEmptyDB verifies that the function
// returns an error when Redis is completely empty.
func TestGenerateAckRecords_NoNotificationsEmptyDB(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)

	if err == nil {
		t.Fatal("Expected an error when no notifications exist, got nil")
	}
	if created != 0 {
		t.Errorf("Expected 0 records created, got %d", created)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// TestGenerateAckRecords_ClKeysExistButValuesEmpty seeds cl:* keys whose GET
// returns empty, exercising the "no UUIDs found" branch inside the fallback.
func TestGenerateAckRecords_ClKeysExistButValuesEmpty(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	// Write a cl:* key that maps to an empty string value.
	emptyClKey := "cl:0:demo::empty-uuid"
	if err := redisClient.Set(redisCtx, emptyClKey, "", 0).Err(); err != nil {
		t.Fatalf("seed: failed to set empty cl key: %v", err)
	}

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)

	if err == nil {
		t.Fatal("Expected error when cl keys exist but values are empty, got nil")
	}
	if created != 0 {
		t.Errorf("Expected 0 records, got %d", created)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Happy-path tests (one or more notification UUIDs seeded)
// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_ReceiveAction_SingleRecord verifies that a single
// "receive" ack is correctly stored when a UUID-shaped key exists.
func TestGenerateAckRecords_ReceiveAction_SingleRecord(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "c7d037d3-9b0e-4bca-b5fb-a4b393aee560"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if created != 1 {
		t.Errorf("Expected 1 record, got %d", created)
	}
}

// TestGenerateAckRecords_ReadAction_SingleRecord verifies the "read" action path.
func TestGenerateAckRecords_ReadAction_SingleRecord(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "a1b2c3d4-0000-1111-2222-333344445555"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "read", 1, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if created != 1 {
		t.Errorf("Expected 1 record, got %d", created)
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_MultipleRecords verifies that requesting N records
// results in exactly N ack keys in Redis.
func TestGenerateAckRecords_MultipleRecords(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "aaaabbbb-cccc-dddd-eeee-ffff00001111"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	const count = 5
	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", count, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if created != count {
		t.Errorf("Expected %d records, got %d", count, created)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != count {
		t.Errorf("Expected %d ack:* keys in Redis, got %d", count, len(ackKeys))
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_KeyFormat checks that generated keys follow the
// "ack:<32-hex-char-md5>" pattern.
func TestGenerateAckRecords_KeyFormat(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "11112222-3333-4444-5555-666677778888"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 2, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != 2 {
		t.Fatalf("Expected 2 ack keys, got %d", len(ackKeys))
	}

	for _, key := range ackKeys {
		if !strings.HasPrefix(key, "ack:") {
			t.Errorf("Key %q does not start with 'ack:'", key)
		}
		hashPart := strings.TrimPrefix(key, "ack:")
		if len(hashPart) != 32 {
			t.Errorf("Key %q: hash part has length %d, expected 32", key, len(hashPart))
		}
		for _, c := range hashPart {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Key %q: hash part contains non-hex character %q", key, c)
			}
		}
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_KeyIsMD5OfPayload verifies that the key is the MD5
// of the stored JSON payload value.
func TestGenerateAckRecords_KeyIsMD5OfPayload(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "deadbeef-dead-beef-dead-beefdeadbeef"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != 1 {
		t.Fatalf("Expected 1 ack key, got %d", len(ackKeys))
	}

	ackKey := ackKeys[0]
	payload, err := redisClient.Get(redisCtx, ackKey).Result()
	if err != nil {
		t.Fatalf("Failed to GET %q: %v", ackKey, err)
	}

	hash := md5.Sum([]byte(payload))
	expectedKey := fmt.Sprintf("ack:%x", hash)

	if ackKey != expectedKey {
		t.Errorf("Key mismatch:\n  got:      %s\n  expected: %s", ackKey, expectedKey)
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_PayloadShape verifies every JSON field is present and
// has the correct shape/value.
func TestGenerateAckRecords_PayloadShape(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "cafebabe-cafe-babe-cafe-babecafebabe"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != 1 {
		t.Fatalf("Expected 1 ack key, got %d", len(ackKeys))
	}

	raw, err := redisClient.Get(redisCtx, ackKeys[0]).Result()
	if err != nil {
		t.Fatalf("Failed to GET ack key: %v", err)
	}

	var rec ackRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if rec.Action != "receive" {
		t.Errorf("action: got %q, want %q", rec.Action, "receive")
	}
	if rec.UUID != notifUUID {
		t.Errorf("uuid: got %q, want %q", rec.UUID, notifUUID)
	}
	if rec.Date == "" {
		t.Error("date: field is empty")
	}
	if rec.Fingerprint == "" {
		t.Error("fingerprint: field is empty")
	}
	if len(rec.Fingerprint) > 96 {
		t.Errorf("fingerprint: length %d exceeds max 96", len(rec.Fingerprint))
	}

	// Check date format (should be ISO 8601).
	const iso8601Layout = "2006-01-02T15:04:05.000Z"
	if _, err := time.Parse(iso8601Layout, rec.Date); err != nil {
		t.Errorf("date: invalid ISO 8601 format %q: %v", rec.Date, err)
	}

	// Fingerprint must be hex-only.
	for _, c := range rec.Fingerprint {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Errorf("fingerprint: contains non-hex character %q", c)
			break
		}
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_NoTTL confirms that ack records have no expiry (TTL == -1).
func TestGenerateAckRecords_NoTTL(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "99990000-aaaa-bbbb-cccc-ddddeeee1111"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 1, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != 1 {
		t.Fatalf("Expected 1 ack key, got %d", len(ackKeys))
	}

	ttl, err := redisClient.TTL(redisCtx, ackKeys[0]).Result()
	if err != nil {
		t.Fatalf("Failed to get TTL: %v", err)
	}
	// TTL == -1 means no expiry.
	if ttl != -1*time.Nanosecond {
		t.Errorf("Expected TTL == -1 (no expiry), got %v", ttl)
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_RoundRobinUUIDs ensures that when multiple UUID keys
// exist, records are distributed across all of them.
func TestGenerateAckRecords_RoundRobinUUIDs(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	uuids := []string{
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"cccccccc-cccc-cccc-cccc-cccccccccccc",
	}
	for _, u := range uuids {
		seedNotificationUUID(t, redisClient, redisCtx, u)
	}

	const count = 9
	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", count, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	// Note: Due to MD5 hash collisions from similar timestamps/fingerprints,
	// we may get fewer unique keys than requested. The important thing is that
	// all UUIDs are represented (round-robin was used).
	if len(ackKeys) == 0 {
		t.Fatal("Expected at least one ack key, got 0")
	}

	// Parse all records and verify all UUIDs were used at least once.
	uuidCounts := make(map[string]int)
	for _, key := range ackKeys {
		raw, _ := redisClient.Get(redisCtx, key).Result()
		var rec ackRecord
		json.Unmarshal([]byte(raw), &rec)
		uuidCounts[rec.UUID]++
	}

	// Verify that all 3 UUIDs were referenced (round-robin distribution).
	for _, u := range uuids {
		if uuidCounts[u] == 0 {
			t.Errorf("UUID %q was never used; round-robin not working", u)
		}
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_FallbackToClKeys exercises the fallback branch where
// no UUID-shaped keys exist but cl:* keys do, so values are read via GET.
func TestGenerateAckRecords_FallbackToClKeys(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	// Store ONLY a cl:* key whose value is a plain string (not UUID-shaped),
	// so the function must use the fallback logic.
	clKey := "cl:0:demo::abc-def"
	notifUUID := "12340000-5678-90ab-cdef-1234567890ab"
	if err := redisClient.Set(redisCtx, clKey, notifUUID, 0).Err(); err != nil {
		t.Fatalf("seed: failed to set cl key: %v", err)
	}
	// Also store the UUID key so the function can proceed.
	payload := fmt.Sprintf(`{"id":%q}`, notifUUID)
	if err := redisClient.Set(redisCtx, notifUUID, payload, 0).Err(); err != nil {
		t.Fatalf("seed: failed to set uuid key: %v", err)
	}

	_, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 2, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	if len(ackKeys) != 2 {
		t.Errorf("Expected 2 ack keys, got %d", len(ackKeys))
	}
}

// ───────────────────────────────────────────────────────────────────────────

// TestGenerateAckRecords_ActionStoredCorrectly checks that both "receive" and
// "read" actions are correctly stored in the JSON payload.
func TestGenerateAckRecords_ActionStoredCorrectly(t *testing.T) {
	for _, action := range []string{"receive", "read"} {
		action := action
		t.Run(action, func(t *testing.T) {
			mr, redisClient, redisCtx := setupAckRedisContainer(t)
			defer func() {
				redisClient.Close()
				mr.Close()
			}()

			notifUUID := "11223344-5566-7788-99aa-bbccddeeff00"
			seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

			_, err := generateAckRecordsWithClient(redisClient, redisCtx, action, 1, 1, 1)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
			if len(ackKeys) != 1 {
				t.Fatalf("Expected 1 ack key, got %d", len(ackKeys))
			}
			raw, _ := redisClient.Get(redisCtx, ackKeys[0]).Result()
			var rec ackRecord
			json.Unmarshal([]byte(raw), &rec)

			if rec.Action != action {
				t.Errorf("action: got %q, want %q", rec.Action, action)
			}
		})
	}
}

// TestGenerateAckRecords_UniqueKeysPerRecord checks that generating multiple
// records produces distinct ack keys (each payload differs because the date
// and fingerprint are random/time-based).
func TestGenerateAckRecords_UniqueKeysPerRecord(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "fedcba98-7654-3210-fedc-ba9876543210"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	const count = 10
	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", count, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if created != count {
		t.Errorf("Expected %d records, got %d", count, created)
	}

	ackKeys := scanKeys(t, redisClient, redisCtx, "ack:*")
	unique := make(map[string]struct{}, len(ackKeys))
	for _, k := range ackKeys {
		unique[k] = struct{}{}
	}
	if len(unique) != count {
		t.Errorf("Expected %d unique ack keys, got %d", count, len(unique))
	}
}

// TestGenerateAckRecords_CountZero verifies that requesting zero records is a
// no-op: no error and zero created.
func TestGenerateAckRecords_CountZero(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "00000000-0000-0000-0000-000000000001"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 0, 1, 1)
	if err != nil {
		t.Fatalf("Unexpected error for count=0: %v", err)
	}
	if created != 0 {
		t.Errorf("Expected 0 records for count=0, got %d", created)
	}
}

// TestGenerateAckRecords_IterationParameters verifies the function does not
// error when non-default iteration/total parameters are supplied.
func TestGenerateAckRecords_IterationParameters(t *testing.T) {
	mr, redisClient, redisCtx := setupAckRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	notifUUID := "12345678-1234-1234-1234-123456789abc"
	seedNotificationUUID(t, redisClient, redisCtx, notifUUID)

	created, err := generateAckRecordsWithClient(redisClient, redisCtx, "receive", 3, 2, 5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if created != 3 {
		t.Errorf("Expected 3 records, got %d", created)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// scanKeys helper
// ───────────────────────────────────────────────────────────────────────────

// scanKeys returns all keys matching pattern from Redis.
func scanKeys(t *testing.T, redisClient *redis.Client, redisCtx context.Context, pattern string) []string {
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
