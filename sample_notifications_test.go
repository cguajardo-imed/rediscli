package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSampleVariantNames(t *testing.T) {
	variants := sampleVariantNames()
	if len(variants) != 11 {
		t.Fatalf("expected 11 variants, got %d", len(variants))
	}
}

func TestGenerateSampleNotificationWithClient_Variants(t *testing.T) {
	tests := []struct {
		name               string
		variant            int
		expectType         string
		expectCriticality  string
		containsInContent  []string
		minPlainTextLength int
		maxContentLength   int
	}{
		{
			name:              "markdown content",
			variant:           0,
			expectType:        "alert",
			expectCriticality: "low",
			containsInContent: []string{"# Alert Title", "**bold**", "[Learn more](https://example.com)"},
		},
		{
			name:              "html content",
			variant:           1,
			expectType:        "alert",
			expectCriticality: "low",
			containsInContent: []string{"<h2>System Alert</h2>", "<strong>HTML</strong>", "<ul>"},
		},
		{
			name:               "long content",
			variant:            2,
			expectType:         "alert",
			expectCriticality:  "low",
			containsInContent:  []string{"<p>", "stress conditions"},
			minPlainTextLength: 401,
		},
		{
			name:              "short content",
			variant:           3,
			expectType:        "alert",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
			maxContentLength:  99,
		},
		{
			name:              "type alert",
			variant:           4,
			expectType:        "alert",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "type info",
			variant:           5,
			expectType:        "info",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "type warning",
			variant:           6,
			expectType:        "warning",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "type error",
			variant:           7,
			expectType:        "error",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "criticality low",
			variant:           8,
			expectType:        "alert",
			expectCriticality: "low",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "criticality medium",
			variant:           9,
			expectType:        "alert",
			expectCriticality: "medium",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
		{
			name:              "criticality high",
			variant:           10,
			expectType:        "alert",
			expectCriticality: "high",
			containsInContent: []string{"<p>Brief alert.</p>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, redisClient, redisCtx := setupRedisContainer(t)
			defer func() {
				redisClient.Close()
				mr.Close()
			}()

			sub := redisClient.Subscribe(redisCtx, sampleNotificationsChannel)
			defer sub.Close()

			msgCh := sub.Channel()

			result, err := generateSampleNotificationWithClient(redisClient, redisCtx, tt.variant)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, "Generated sample notification") {
				t.Fatalf("unexpected success message: %q", result)
			}

			clKeys := scanKeys(t, redisClient, redisCtx, "cl:0:sample::*")
			if len(clKeys) != 1 {
				t.Fatalf("expected exactly one cl key, got %d", len(clKeys))
			}

			parentUUID, err := redisClient.Get(redisCtx, clKeys[0]).Result()
			if err != nil {
				t.Fatalf("failed to read pointer key: %v", err)
			}
			if parentUUID == "" {
				t.Fatal("pointer key value (uuid) is empty")
			}

			recordRaw, err := redisClient.Get(redisCtx, parentUUID).Result()
			if err != nil {
				t.Fatalf("failed to read record key: %v", err)
			}

			var rec NotificationRecord
			if err := json.Unmarshal([]byte(recordRaw), &rec); err != nil {
				t.Fatalf("failed to unmarshal NotificationRecord: %v", err)
			}

			if rec.ID != parentUUID {
				t.Fatalf("record ID mismatch: got %q, want %q", rec.ID, parentUUID)
			}
			if rec.Type != tt.expectType {
				t.Fatalf("type mismatch: got %q, want %q", rec.Type, tt.expectType)
			}
			if rec.Criticality != tt.expectCriticality {
				t.Fatalf("criticality mismatch: got %q, want %q", rec.Criticality, tt.expectCriticality)
			}
			if len(rec.Messages) != 1 {
				t.Fatalf("expected one message, got %d", len(rec.Messages))
			}

			content := rec.Messages[0].Content
			plainText := rec.Messages[0].PlainText
			for _, expected := range tt.containsInContent {
				if !strings.Contains(content, expected) {
					t.Fatalf("content does not contain %q\ncontent: %s", expected, content)
				}
			}

			if tt.minPlainTextLength > 0 && len(plainText) < tt.minPlainTextLength {
				t.Fatalf("plain_text length too short: got %d, want >= %d", len(plainText), tt.minPlainTextLength)
			}
			if tt.maxContentLength > 0 && len(content) >= tt.maxContentLength {
				t.Fatalf("content length too long: got %d, want < %d", len(content), tt.maxContentLength)
			}

			ttlPointer, err := redisClient.TTL(redisCtx, clKeys[0]).Result()
			if err != nil {
				t.Fatalf("failed to get pointer ttl: %v", err)
			}
			if ttlPointer <= 0 || ttlPointer > time.Minute {
				t.Fatalf("unexpected pointer ttl: %v", ttlPointer)
			}

			ttlPayload, err := redisClient.TTL(redisCtx, parentUUID).Result()
			if err != nil {
				t.Fatalf("failed to get payload ttl: %v", err)
			}
			if ttlPayload <= 0 || ttlPayload > time.Minute {
				t.Fatalf("unexpected payload ttl: %v", ttlPayload)
			}

			select {
			case msg := <-msgCh:
				if msg.Channel != sampleNotificationsChannel {
					t.Fatalf("unexpected channel: got %q, want %q", msg.Channel, sampleNotificationsChannel)
				}
				var cv ChannelValue
				if err := json.Unmarshal([]byte(msg.Payload), &cv); err != nil {
					t.Fatalf("failed to unmarshal channel payload: %v", err)
				}
				if cv.Key != clKeys[0] {
					t.Fatalf("published key mismatch: got %q, want %q", cv.Key, clKeys[0])
				}
				if cv.Value != recordRaw {
					t.Fatalf("published payload mismatch")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for published message")
			}
		})
	}
}

func TestGenerateSampleNotificationWithClient_InvalidVariant(t *testing.T) {
	mr, redisClient, redisCtx := setupRedisContainer(t)
	defer func() {
		redisClient.Close()
		mr.Close()
	}()

	_, err := generateSampleNotificationWithClient(redisClient, redisCtx, 99)
	if err == nil {
		t.Fatal("expected error for invalid variant, got nil")
	}
}
