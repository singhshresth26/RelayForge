package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "PENDING"
	DeliveryProcessing DeliveryStatus = "PROCESSING"
	DeliveryDelivered  DeliveryStatus = "DELIVERED"
	DeliveryFailed     DeliveryStatus = "FAILED"
	DeliveryRetrying   DeliveryStatus = "RETRYING"
)

type Delivery struct {
	ID             uuid.UUID       `json:"id"`
	EventID        uuid.UUID       `json:"event_id"`
	EndpointID     uuid.UUID       `json:"endpoint_id"`
	Status         DeliveryStatus  `json:"status"`
	CurrentAttempt int32           `json:"current_attempt"`
	MaxAttempts    int32           `json:"max_attempts"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty"`
	DeliveredAt    *time.Time      `json:"delivered_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	EndpointURL    string          `json:"endpoint_url,omitempty"`
	EventType      string          `json:"event_type,omitempty"`
}

type DeliveryAttempt struct {
	ID              int64           `json:"id"`
	DeliveryID      uuid.UUID       `json:"delivery_id"`
	AttemptNumber   int32           `json:"attempt_number"`
	StatusCode      *int32          `json:"status_code,omitempty"`
	LatencyMS       int32           `json:"latency_ms"`
	RequestHeaders  json.RawMessage `json:"request_headers"`
	ResponseHeaders json.RawMessage `json:"response_headers"`
	ResponseBody    *string         `json:"response_body,omitempty"`
	ErrorMessage    *string         `json:"error_message,omitempty"`
	ExecutedAt      time.Time       `json:"executed_at"`
}

