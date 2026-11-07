package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
)

const (
	MaxPayloadBytes = 256 * 1024 // 256KB maximum payload
)

type EventService struct {
	eventRepo  repository.EventRepository
	outboxRepo repository.OutboxRepository
	appRepo    repository.ApplicationRepository
}

func NewEventService(
	eventRepo repository.EventRepository,
	outboxRepo repository.OutboxRepository,
	appRepo repository.ApplicationRepository,
) *EventService {
	return &EventService{
		eventRepo:  eventRepo,
		outboxRepo: outboxRepo,
		appRepo:    appRepo,
	}
}

type IngestEventInput struct {
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	IdempotencyKey *string         `json:"-"`
}

type IngestEventResult struct {
	Event       *domain.Event `json:"event"`
	IsDuplicate bool          `json:"is_duplicate"`
}

// IngestEvent processes incoming events with atomic outbox writes and idempotency control
func (s *EventService) IngestEvent(ctx context.Context, tenantID, appID uuid.UUID, input IngestEventInput) (*IngestEventResult, error) {
	// 1. Verify application ownership
	app, err := s.appRepo.GetApplicationByID(ctx, appID, tenantID)
	if err != nil {
		return nil, err
	}
	if app == nil || !app.IsActive {
		return nil, domain.ErrNotFound
	}

	// 2. Validate event type
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return nil, domain.NewAppError("INVALID_EVENT_TYPE", "event_type is required", domain.ErrInvalidInput)
	}

	// 3. Validate payload
	if len(input.Payload) == 0 {
		return nil, domain.NewAppError("EMPTY_PAYLOAD", "payload cannot be empty", domain.ErrInvalidInput)
	}
	if len(input.Payload) > MaxPayloadBytes {
		return nil, domain.NewAppError("PAYLOAD_TOO_LARGE", fmt.Sprintf("payload exceeds limit of %d bytes", MaxPayloadBytes), domain.ErrInvalidInput)
	}

	// Validate JSON syntax
	if !json.Valid(input.Payload) {
		return nil, domain.NewAppError("INVALID_JSON_PAYLOAD", "payload must be valid JSON", domain.ErrInvalidInput)
	}

	metadata := input.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	// 4. Check Idempotency Key
	if input.IdempotencyKey != nil && strings.TrimSpace(*input.IdempotencyKey) != "" {
		trimmedKey := strings.TrimSpace(*input.IdempotencyKey)
		existing, err := s.eventRepo.GetEventByIdempotencyKey(ctx, appID, trimmedKey)
		if err == nil && existing != nil {
			// Idempotent duplicate detected! Return existing event safely without re-inserting to outbox
			return &IngestEventResult{
				Event:       existing,
				IsDuplicate: true,
			}, nil
		}
		input.IdempotencyKey = &trimmedKey
	}

	// 5. Create Event & Outbox Record
	event, err := s.eventRepo.CreateEvent(ctx, appID, input.IdempotencyKey, eventType, input.Payload, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to persist event: %w", err)
	}

	// Transactional outbox entry
	_, err = s.outboxRepo.CreateOutboxEvent(ctx, event.ID, appID, eventType, input.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	return &IngestEventResult{
		Event:       event,
		IsDuplicate: false,
	}, nil
}

func (s *EventService) GetEvent(ctx context.Context, id, tenantID uuid.UUID) (*domain.Event, error) {
	ev, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, domain.ErrNotFound
	}

	// Verify app ownership
	app, err := s.appRepo.GetApplicationByID(ctx, ev.AppID, tenantID)
	if err != nil || app == nil {
		return nil, domain.ErrNotFound
	}

	return ev, nil
}

func (s *EventService) ListEvents(ctx context.Context, appID, tenantID uuid.UUID, limit, offset int32) ([]*domain.Event, error) {
	app, err := s.appRepo.GetApplicationByID(ctx, appID, tenantID)
	if err != nil || app == nil {
		return nil, domain.ErrNotFound
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	return s.eventRepo.ListEventsByApp(ctx, appID, limit, offset)
}
