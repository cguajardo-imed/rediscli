package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

const sampleNotificationsChannel = "gns_notifications_channel"

func sampleVariantNames() []string {
	return []string{
		"Markdown content",
		"HTML content",
		"Long content (>400 chars)",
		"Short content",
		"Type: alert",
		"Type: info",
		"Type: warning",
		"Type: error",
		"Criticality: low",
		"Criticality: medium",
		"Criticality: high",
	}
}

func generateSampleNotification(variant int) (string, error) {
	return generateSampleNotificationWithClient(client, ctx, variant)
}

func generateSampleNotificationWithClient(redisClient *redis.Client, redisCtx context.Context, variant int) (string, error) {
	if variant < 0 || variant >= len(sampleVariantNames()) {
		return "", fmt.Errorf("invalid sample variant: %d", variant)
	}

	rec, pointerKey, parentUUID, err := buildSampleNotificationRecord(variant)
	if err != nil {
		return "", err
	}

	recordBytes, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sample notification record: %w", err)
	}
	fullContent := string(recordBytes)

	if err := redisClient.Set(redisCtx, pointerKey, parentUUID, time.Minute).Err(); err != nil {
		return "", fmt.Errorf("failed to set sample pointer key: %w", err)
	}
	if err := redisClient.Set(redisCtx, parentUUID, fullContent, time.Minute).Err(); err != nil {
		return "", fmt.Errorf("failed to set sample payload key: %w", err)
	}

	channelValue := ChannelValue{Key: pointerKey, Value: fullContent}
	msg, err := json.Marshal(channelValue)
	if err != nil {
		return "", fmt.Errorf("failed to marshal publish payload: %w", err)
	}
	if err := redisClient.Publish(redisCtx, sampleNotificationsChannel, msg).Err(); err != nil {
		return "", fmt.Errorf("failed to publish sample notification: %w", err)
	}

	return fmt.Sprintf("Generated sample notification: %s", rec.Title), nil
}

func buildSampleNotificationRecord(variant int) (NotificationRecord, string, string, error) {
	createdAt := time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	parentUUID := uuid.NewString()
	messageUUID := uuid.NewString()

	title := "[Sample] Notification"
	content := "<p>Brief alert.</p>"
	plainText := "Brief alert."
	notificationType := "alert"
	criticality := "low"

	longPlainText := "This is a long sample notification intended to verify how clients render large payloads in a realistic scenario. It includes enough descriptive text to exceed four hundred characters while remaining readable and coherent for manual verification, automated checks, and downstream processing paths that may truncate, wrap, or otherwise transform content under stress conditions in dashboards, message centers, and archival flows used by notification consumers."

	switch variant {
	case 0:
		title = "[Sample] Markdown Notification"
		content = "# Alert Title\n\nThis is a **bold** notification with *emphasis*.\n\n- Item 1\n- Item 2\n- Item 3\n\n[Learn more](https://example.com)"
		plainText = "Alert Title\n\nThis is a bold notification with emphasis.\n\n- Item 1\n- Item 2\n- Item 3\n\nLearn more: https://example.com"
	case 1:
		title = "[Sample] HTML Notification"
		content = "<h2>System Alert</h2><p>This notification contains <strong>HTML</strong> content with <em>rich formatting</em>.</p><ul><li>First point</li><li>Second point</li></ul>"
		plainText = "System Alert\nThis notification contains HTML content with rich formatting.\n- First point\n- Second point"
	case 2:
		title = "[Sample] Long Notification"
		content = fmt.Sprintf("<p>%s</p>", longPlainText)
		plainText = longPlainText
	case 3:
		title = "[Sample] Short Notification"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
	case 4:
		title = "[Sample] Type: alert"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		notificationType = "alert"
	case 5:
		title = "[Sample] Type: info"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		notificationType = "info"
	case 6:
		title = "[Sample] Type: warning"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		notificationType = "warning"
	case 7:
		title = "[Sample] Type: error"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		notificationType = "error"
	case 8:
		title = "[Sample] Criticality: low"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		criticality = "low"
	case 9:
		title = "[Sample] Criticality: medium"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		criticality = "medium"
	case 10:
		title = "[Sample] Criticality: high"
		content = "<p>Brief alert.</p>"
		plainText = "Brief alert."
		criticality = "high"
	default:
		return NotificationRecord{}, "", "", fmt.Errorf("invalid sample variant: %d", variant)
	}

	record := NotificationRecord{
		Criticality: criticality,
		Title:       title,
		Messages: []NotificationMessage{
			{
				UUID:      messageUUID,
				Content:   content,
				PlainText: plainText,
				CreatedAt: createdAt,
			},
		},
		Action:    "",
		Type:      notificationType,
		ID:        parentUUID,
		Status:    "pending",
		CreatedAt: createdAt,
	}

	pointerKey := fmt.Sprintf("cl:%s:%s:%s:%s", "0", "sample", "", parentUUID)
	return record, pointerKey, parentUUID, nil
}
