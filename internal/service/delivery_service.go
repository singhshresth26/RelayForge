package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository"
)

type DeliveryService struct {
	deliveryRepo repository.DeliveryRepository
	endpointRepo repository.EndpointRepository
	eventRepo    repository.EventRepository
	appRepo      repository.ApplicationRepository
	publisher    fanout.DeliveryPublisher
}

func NewDeliveryService(
	deliveryRepo repository.DeliveryRepository,
	endpointRepo repository.EndpointRepository,
	eventRepo repository.EventRepository,
	appRepo repository.ApplicationRepository,
	publisher fanout.DeliveryPublisher,
) *DeliveryService {
	return &DeliveryService{
		deliveryRepo: deliveryRepo,
		endpointRepo: endpointRepo,
		eventRepo:    eventRepo,
		appRepo:      appRepo,
		publisher:    publisher,
	}
}

func (s *DeliveryService) GetDelivery(ctx context.Context, id, tenantID uuid.UUID) (*domain.Delivery, error) {
	d, err := s.deliveryRepo.GetDeliveryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, domain.ErrNotFound
	}

	// Verify tenant ownership via endpoint
	ep, err := s.endpointRepo.GetEndpointWithApp(ctx, d.EndpointID, tenantID)
	if err != nil || ep == nil {
		return nil, domain.ErrNotFound
	}

	return d, nil
}

func (s *DeliveryService) ListDeliveries(ctx context.Context, appID, tenantID uuid.UUID, limit, offset int32) ([]*domain.Delivery, error) {
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

	return s.deliveryRepo.ListDeliveriesByApp(ctx, appID, limit, offset)
}

func (s *DeliveryService) ListAttempts(ctx context.Context, deliveryID, tenantID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	d, err := s.deliveryRepo.GetDeliveryByID(ctx, deliveryID)
	if err != nil || d == nil {
		return nil, domain.ErrNotFound
	}

	ep, err := s.endpointRepo.GetEndpointWithApp(ctx, d.EndpointID, tenantID)
	if err != nil || ep == nil {
		return nil, domain.ErrNotFound
	}

	return s.deliveryRepo.ListAttemptsByDeliveryID(ctx, deliveryID)
}

// ReplayDelivery manually resets a delivery attempt counter and re-enqueues it
func (s *DeliveryService) ReplayDelivery(ctx context.Context, deliveryID, tenantID uuid.UUID) (*domain.Delivery, error) {
	d, err := s.deliveryRepo.GetDeliveryByID(ctx, deliveryID)
	if err != nil || d == nil {
		return nil, domain.ErrNotFound
	}

	ep, err := s.endpointRepo.GetEndpointWithApp(ctx, d.EndpointID, tenantID)
	if err != nil || ep == nil {
		return nil, domain.ErrNotFound
	}

	event, err := s.eventRepo.GetEventByID(ctx, d.EventID)
	if err != nil || event == nil {
		return nil, fmt.Errorf("underlying event not found: %w", err)
	}

	// Reset delivery in DB
	updated, err := s.deliveryRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.DeliveryPending, 0, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to reset delivery status: %w", err)
	}

	// Re-enqueue delivery job
	job := &fanout.DeliveryJob{
		DeliveryID:      deliveryID,
		EventID:         event.ID,
		EndpointID:      ep.ID,
		URL:             ep.URL,
		SecretEncrypted: ep.SecretEncrypted,
		EventType:       event.EventType,
		Payload:         event.Payload,
		CurrentAttempt:  0,
		MaxAttempts:     ep.MaxRetries,
		TimeoutMS:       ep.TimeoutMS,
		RateLimitRPS:    ep.RateLimitRPS,
	}

	if s.publisher != nil {
		if err := s.publisher.PublishDelivery(ctx, job); err != nil {
			return nil, fmt.Errorf("failed to republish delivery job: %w", err)
		}
	}

	return updated, nil
}

