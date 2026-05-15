package main

// NotificationMessage represents a single message entry inside a NotificationRecord.
type NotificationMessage struct {
	UUID      string `json:"uuid"`
	Content   string `json:"content"`
	PlainText string `json:"plain_text"`
	CreatedAt string `json:"created_at"`
}

// NotificationRecord represents the full notification payload stored in Redis.
type NotificationRecord struct {
	Criticality string                `json:"criticality"`
	Title       string                `json:"title"`
	Messages    []NotificationMessage `json:"messages"`
	Action      string                `json:"action"`
	Type        string                `json:"type"`
	ID          string                `json:"id"`
	Status      string                `json:"status"`
	CreatedAt   string                `json:"created_at"`
}
