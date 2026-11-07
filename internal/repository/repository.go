package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
)

type TenantRepository interface {
	CreateTenant(ctx context.Context, name, slug, plan string) (*domain.Tenant, error)
	GetTenantByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	ListTenants(ctx context.Context, limit, offset int32) ([]*domain.Tenant, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, tenantID uuid.UUID, email, passwordHash string, role domain.UserRole) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	ListUsersByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.User, error)
}

type APIKeyRepository interface {
	CreateAPIKey(ctx context.Context, tenantID uuid.UUID, name, prefix, hash string, scopes []string, expiresAt *time.Time) (*domain.APIKey, error)
	GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	ListAPIKeysByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error)
	RevokeAPIKey(ctx context.Context, id, tenantID uuid.UUID) (*domain.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error
}

type ApplicationRepository interface {
	CreateApplication(ctx context.Context, tenantID uuid.UUID, name, description string, isActive bool) (*domain.Application, error)
	GetApplicationByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Application, error)
	ListApplicationsByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.Application, error)
	UpdateApplication(ctx context.Context, id, tenantID uuid.UUID, name, description string, isActive bool) (*domain.Application, error)
	DeleteApplication(ctx context.Context, id, tenantID uuid.UUID) error
}

type EndpointRepository interface {
	CreateEndpoint(ctx context.Context, appID uuid.UUID, url, secretEncrypted, description string, isActive bool, maxRetries, timeoutMS, rateLimitRPS int32, subscriptions []string) (*domain.Endpoint, error)
	GetEndpointByID(ctx context.Context, id uuid.UUID) (*domain.Endpoint, error)
	GetEndpointWithApp(ctx context.Context, id, tenantID uuid.UUID) (*domain.Endpoint, error)
	ListEndpointsByApp(ctx context.Context, appID uuid.UUID) ([]*domain.Endpoint, error)
	ListActiveEndpointsForEvent(ctx context.Context, appID uuid.UUID, eventType string) ([]*domain.Endpoint, error)
	UpdateEndpoint(ctx context.Context, id uuid.UUID, url, description string, isActive bool, maxRetries, timeoutMS, rateLimitRPS int32, subscriptions []string) (*domain.Endpoint, error)
	UpdateEndpointSecret(ctx context.Context, id uuid.UUID, secretEncrypted string) (*domain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error
}

type EventRepository interface {
	CreateEvent(ctx context.Context, appID uuid.UUID, idempotencyKey *string, eventType string, payload, metadata []byte) (*domain.Event, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	GetEventByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*domain.Event, error)
	ListEventsByApp(ctx context.Context, appID uuid.UUID, limit, offset int32) ([]*domain.Event, error)
}

type OutboxRepository interface {
	CreateOutboxEvent(ctx context.Context, eventID, appID uuid.UUID, eventType string, payload []byte) (*domain.OutboxEvent, error)
	FetchPendingOutboxBatch(ctx context.Context, limit int32) ([]*domain.OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, ids []int64) error
	MarkOutboxFailed(ctx context.Context, id int64, lastError string) error
}

type DeliveryRepository interface {
	CreateDelivery(ctx context.Context, eventID, endpointID uuid.UUID, maxAttempts int32) (*domain.Delivery, error)
	GetDeliveryByID(ctx context.Context, id uuid.UUID) (*domain.Delivery, error)
	ListDeliveriesByApp(ctx context.Context, appID uuid.UUID, limit, offset int32) ([]*domain.Delivery, error)
	ListDeliveriesByEvent(ctx context.Context, eventID uuid.UUID) ([]*domain.Delivery, error)
	UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, status domain.DeliveryStatus, currentAttempt int32, nextRetryAt, deliveredAt *time.Time) (*domain.Delivery, error)
	CreateDeliveryAttempt(ctx context.Context, deliveryID uuid.UUID, attemptNum int32, statusCode *int32, latencyMS int32, reqHeaders, respHeaders []byte, respBody, errMsg *string) (*domain.DeliveryAttempt, error)
	ListAttemptsByDeliveryID(ctx context.Context, deliveryID uuid.UUID) ([]*domain.DeliveryAttempt, error)
}

