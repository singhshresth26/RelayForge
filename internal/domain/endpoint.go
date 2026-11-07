package domain

import (
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID              uuid.UUID `json:"id"`
	AppID           uuid.UUID `json:"app_id"`
	URL             string    `json:"url"`
	SecretEncrypted string    `json:"-"`
	Description     string    `json:"description"`
	IsActive        bool      `json:"is_active"`
	MaxRetries      int32     `json:"max_retries"`
	TimeoutMS       int32     `json:"timeout_ms"`
	RateLimitRPS    int32     `json:"rate_limit_rps"`
	Subscriptions   []string  `json:"subscriptions,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type EndpointCreated struct {
	Endpoint  *Endpoint `json:"endpoint"`
	SecretKey string    `json:"secret_key"` // Revealed only upon creation or rotation
}

