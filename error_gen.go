package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// ErrorData represents an error record to be stored in Redis.
type ErrorData struct {
	Message   string `json:"message"`
	Code      string `json:"code,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Details   string `json:"details,omitempty"`
}

// CreateErrorRecords creates error records in Redis following the documented spec.
// Key format: ERRORS:<fingerprint>:<message>
// This is a best-effort operation and always returns nil.
func CreateErrorRecords(data []ErrorData, fingerprint string) error {
	return CreateErrorRecordsWithClient(client, ctx, data, fingerprint)
}

// CreateErrorRecordsWithClient is the testable core of CreateErrorRecords.
func CreateErrorRecordsWithClient(redisClient *redis.Client, redisCtx context.Context, data []ErrorData, fingerprint string) error {
	for _, d := range data {
		// Build Redis key as ERRORS:<fingerprint>:<message>
		key := fmt.Sprintf("ERRORS:%s:%s", fingerprint, d.Message)

		// Marshal to JSON
		value, err := json.Marshal(d)
		if err != nil {
			// Silently skip marshal errors (best-effort)
			continue
		}

		// Write to Redis (best-effort)
		setErr := redisClient.Set(redisCtx, key, string(value), 0).Err()
		if setErr == nil {
			LogInfo(fmt.Sprintf("ERROR payload sent - Key: %s, Value: %s", key, string(value)))

			// Pretty-print JSON for console display
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, value, "", "  "); err == nil {
				indented := "    " + strings.ReplaceAll(prettyJSON.String(), "\n", "\n    ")
				fmt.Printf("✓ ERROR sent\n  Key: %s\n  Payload:\n%s\n", key, indented)
			} else {
				fmt.Printf("✓ ERROR sent\n  Key: %s\n  Payload: %s\n", key, string(value))
			}
		}
	}

	// Always return nil per documented behavior
	return nil
}
