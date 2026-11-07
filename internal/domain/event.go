package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID             uuid.UUID       `json:"id"`
	AppID          uuid.UUID       `json:"app_id"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
}

