package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type OutboxStatus string

const (
	OutboxPending    OutboxStatus = "PENDING"
	OutboxProcessing OutboxStatus = "PROCESSING"
	OutboxPublished  OutboxStatus = "PUBLISHED"
	OutboxFailed     OutboxStatus = "FAILED"
)

type OutboxEvent struct {
	ID          int64           `json:"id"`
	EventID     uuid.UUID       `json:"event_id"`
	AppID       uuid.UUID       `json:"app_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      OutboxStatus    `json:"status"`
	RetryCount  int32           `json:"retry_count"`
	LastError   *string         `json:"last_error,omitempty"`
	LockedAt    *time.Time      `json:"locked_at,omitempty"`
	LockedBy    *string         `json:"locked_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
}

