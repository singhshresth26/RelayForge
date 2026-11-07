package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
)

type MemoryStore struct {
	mu           sync.RWMutex
	tenants      map[uuid.UUID]*domain.Tenant
	tenantsBySlug map[string]*domain.Tenant
	users        map[uuid.UUID]*domain.User
	usersByEmail map[string]*domain.User
	apiKeys      map[uuid.UUID]*domain.APIKey
	apiKeysByHash map[string]*domain.APIKey
	apps         map[uuid.UUID]*domain.Application
	endpoints    map[uuid.UUID]*domain.Endpoint
	events       map[uuid.UUID]*domain.Event
	eventsByIdemp map[string]*domain.Event
	outbox       []*domain.OutboxEvent
	deliveries   map[uuid.UUID]*domain.Delivery
	attempts     []*domain.DeliveryAttempt
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:       make(map[uuid.UUID]*domain.Tenant),
		tenantsBySlug: make(map[string]*domain.Tenant),
		users:         make(map[uuid.UUID]*domain.User),
		usersByEmail: make(map[string]*domain.User),
		apiKeys:       make(map[uuid.UUID]*domain.APIKey),
		apiKeysByHash: make(map[string]*domain.APIKey),
		apps:          make(map[uuid.UUID]*domain.Application),
		endpoints:     make(map[uuid.UUID]*domain.Endpoint),
		events:        make(map[uuid.UUID]*domain.Event),
		eventsByIdemp: make(map[string]*domain.Event),
		deliveries:    make(map[uuid.UUID]*domain.Delivery),
	}
}

// Ensure MemoryStore implements all repository interfaces
var _ repository.TenantRepository = (*MemoryStore)(nil)
var _ repository.UserRepository = (*MemoryStore)(nil)
var _ repository.APIKeyRepository = (*MemoryStore)(nil)
var _ repository.ApplicationRepository = (*MemoryStore)(nil)
var _ repository.EndpointRepository = (*MemoryStore)(nil)
var _ repository.EventRepository = (*MemoryStore)(nil)
var _ repository.OutboxRepository = (*MemoryStore)(nil)
var _ repository.DeliveryRepository = (*MemoryStore)(nil)

// TenantRepository
func (s *MemoryStore) CreateTenant(ctx context.Context, name, slug, plan string) (*domain.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenantsBySlug[slug]; exists {
		return nil, domain.ErrAlreadyExists
	}

	tenant := &domain.Tenant{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		Plan:      plan,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	s.tenants[tenant.ID] = tenant
	s.tenantsBySlug[slug] = tenant
	return tenant, nil
}

func (s *MemoryStore) GetTenantByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *MemoryStore) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenantsBySlug[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *MemoryStore) ListTenants(ctx context.Context, limit, offset int32) ([]*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Tenant
	for _, t := range s.tenants {
		result = append(result, t)
	}
	return result, nil
}

// UserRepository
func (s *MemoryStore) CreateUser(ctx context.Context, tenantID uuid.UUID, email, passwordHash string, role domain.UserRole) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usersByEmail[email]; exists {
		return nil, domain.ErrAlreadyExists
	}

	user := &domain.User{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	s.users[user.ID] = user
	s.usersByEmail[email] = user
	return user, nil
}

func (s *MemoryStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (s *MemoryStore) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (s *MemoryStore) ListUsersByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.User
	for _, u := range s.users {
		if u.TenantID == tenantID {
			result = append(result, u)
		}
	}
	return result, nil
}

// APIKeyRepository
func (s *MemoryStore) CreateAPIKey(ctx context.Context, tenantID uuid.UUID, name, prefix, hash string, scopes []string, expiresAt *time.Time) (*domain.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := &domain.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	s.apiKeys[key.ID] = key
	s.apiKeysByHash[hash] = key
	return key, nil
}

func (s *MemoryStore) GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.apiKeysByHash[hash]
	if !ok || k.RevokedAt != nil {
		return nil, domain.ErrNotFound
	}
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrNotFound
	}
	return k, nil
}

func (s *MemoryStore) ListAPIKeysByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.APIKey
	for _, k := range s.apiKeys {
		if k.TenantID == tenantID && k.RevokedAt == nil {
			result = append(result, k)
		}
	}
	return result, nil
}

func (s *MemoryStore) RevokeAPIKey(ctx context.Context, id, tenantID uuid.UUID) (*domain.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.apiKeys[id]
	if !ok || k.TenantID != tenantID || k.RevokedAt != nil {
		return nil, domain.ErrNotFound
	}
	now := time.Now().UTC()
	k.RevokedAt = &now
	return k, nil
}

func (s *MemoryStore) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k, ok := s.apiKeys[id]; ok {
		now := time.Now().UTC()
		k.LastUsedAt = &now
	}
	return nil
}

// ApplicationRepository
func (s *MemoryStore) CreateApplication(ctx context.Context, tenantID uuid.UUID, name, description string, isActive bool) (*domain.Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.apps {
		if a.TenantID == tenantID && a.Name == name {
			return nil, domain.ErrAlreadyExists
		}
	}

	app := &domain.Application{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        name,
		Description: description,
		IsActive:    isActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	s.apps[app.ID] = app
	return app, nil
}

func (s *MemoryStore) GetApplicationByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[id]
	if !ok || app.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return app, nil
}

func (s *MemoryStore) ListApplicationsByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Application
	for _, a := range s.apps {
		if a.TenantID == tenantID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *MemoryStore) UpdateApplication(ctx context.Context, id, tenantID uuid.UUID, name, description string, isActive bool) (*domain.Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok || app.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	app.Name = name
	app.Description = description
	app.IsActive = isActive
	app.UpdatedAt = time.Now().UTC()
	return app, nil
}

func (s *MemoryStore) DeleteApplication(ctx context.Context, id, tenantID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok || app.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(s.apps, id)
	return nil
}

// EndpointRepository
func (s *MemoryStore) CreateEndpoint(
	ctx context.Context, appID uuid.UUID, url, secretEncrypted, description string,
	isActive bool, maxRetries, timeoutMS, rateLimitRPS int32, subscriptions []string,
) (*domain.Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ep := &domain.Endpoint{
		ID:              uuid.New(),
		AppID:           appID,
		URL:             url,
		SecretEncrypted: secretEncrypted,
		Description:     description,
		IsActive:        isActive,
		MaxRetries:      maxRetries,
		TimeoutMS:       timeoutMS,
		RateLimitRPS:    rateLimitRPS,
		Subscriptions:   subscriptions,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	s.endpoints[ep.ID] = ep
	return ep, nil
}

func (s *MemoryStore) GetEndpointByID(ctx context.Context, id uuid.UUID) (*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return ep, nil
}

func (s *MemoryStore) GetEndpointWithApp(ctx context.Context, id, tenantID uuid.UUID) (*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	app, ok := s.apps[ep.AppID]
	if !ok || app.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return ep, nil
}

func (s *MemoryStore) ListEndpointsByApp(ctx context.Context, appID uuid.UUID) ([]*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Endpoint
	for _, ep := range s.endpoints {
		if ep.AppID == appID {
			result = append(result, ep)
		}
	}
	return result, nil
}

func (s *MemoryStore) ListActiveEndpointsForEvent(ctx context.Context, appID uuid.UUID, eventType string) ([]*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Endpoint
	for _, ep := range s.endpoints {
		if ep.AppID == appID && ep.IsActive {
			for _, sub := range ep.Subscriptions {
				if sub == eventType || sub == "*" {
					result = append(result, ep)
					break
				}
			}
		}
	}
	return result, nil
}

func (s *MemoryStore) UpdateEndpoint(
	ctx context.Context, id uuid.UUID, url, description string,
	isActive bool, maxRetries, timeoutMS, rateLimitRPS int32, subscriptions []string,
) (*domain.Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	ep.URL = url
	ep.Description = description
	ep.IsActive = isActive
	ep.MaxRetries = maxRetries
	ep.TimeoutMS = timeoutMS
	ep.RateLimitRPS = rateLimitRPS
	ep.Subscriptions = subscriptions
	ep.UpdatedAt = time.Now().UTC()
	return ep, nil
}

func (s *MemoryStore) UpdateEndpointSecret(ctx context.Context, id uuid.UUID, secretEncrypted string) (*domain.Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	ep.SecretEncrypted = secretEncrypted
	ep.UpdatedAt = time.Now().UTC()
	return ep, nil
}

func (s *MemoryStore) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.endpoints[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.endpoints, id)
	return nil
}

// EventRepository
func (s *MemoryStore) CreateEvent(ctx context.Context, appID uuid.UUID, idempotencyKey *string, eventType string, payload, metadata []byte) (*domain.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if idempotencyKey != nil && *idempotencyKey != "" {
		composite := fmt.Sprintf("%s:%s", appID.String(), *idempotencyKey)
		if existing, ok := s.eventsByIdemp[composite]; ok {
			return existing, nil
		}
	}

	ev := &domain.Event{
		ID:             uuid.New(),
		AppID:          appID,
		IdempotencyKey: idempotencyKey,
		EventType:      eventType,
		Payload:        payload,
		Metadata:       metadata,
		CreatedAt:      time.Now().UTC(),
	}

	s.events[ev.ID] = ev
	if idempotencyKey != nil && *idempotencyKey != "" {
		composite := fmt.Sprintf("%s:%s", appID.String(), *idempotencyKey)
		s.eventsByIdemp[composite] = ev
	}
	return ev, nil
}

func (s *MemoryStore) GetEventByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.events[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return ev, nil
}

func (s *MemoryStore) GetEventByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*domain.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	composite := fmt.Sprintf("%s:%s", appID.String(), key)
	ev, ok := s.eventsByIdemp[composite]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return ev, nil
}

func (s *MemoryStore) ListEventsByApp(ctx context.Context, appID uuid.UUID, limit, offset int32) ([]*domain.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Event
	for _, ev := range s.events {
		if ev.AppID == appID {
			result = append(result, ev)
		}
	}
	return result, nil
}

// OutboxRepository
func (s *MemoryStore) CreateOutboxEvent(ctx context.Context, eventID, appID uuid.UUID, eventType string, payload []byte) (*domain.OutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	outbox := &domain.OutboxEvent{
		ID:        int64(len(s.outbox) + 1),
		EventID:   eventID,
		AppID:     appID,
		EventType: eventType,
		Payload:   payload,
		Status:    domain.OutboxPending,
		CreatedAt: time.Now().UTC(),
	}
	s.outbox = append(s.outbox, outbox)
	return outbox, nil
}

func (s *MemoryStore) FetchPendingOutboxBatch(ctx context.Context, limit int32) ([]*domain.OutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var batch []*domain.OutboxEvent
	for _, o := range s.outbox {
		if o.Status == domain.OutboxPending {
			o.Status = domain.OutboxProcessing
			now := time.Now().UTC()
			o.LockedAt = &now
			batch = append(batch, o)
			if int32(len(batch)) >= limit {
				break
			}
		}
	}
	return batch, nil
}

func (s *MemoryStore) MarkOutboxPublished(ctx context.Context, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idMap := make(map[int64]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	now := time.Now().UTC()
	for _, o := range s.outbox {
		if idMap[o.ID] {
			o.Status = domain.OutboxPublished
			o.PublishedAt = &now
		}
	}
	return nil
}

func (s *MemoryStore) MarkOutboxFailed(ctx context.Context, id int64, lastError string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, o := range s.outbox {
		if o.ID == id {
			o.Status = domain.OutboxFailed
			o.RetryCount++
			o.LastError = &lastError
			break
		}
	}
	return nil
}

// DeliveryRepository
func (s *MemoryStore) CreateDelivery(ctx context.Context, eventID, endpointID uuid.UUID, maxAttempts int32) (*domain.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range s.deliveries {
		if d.EventID == eventID && d.EndpointID == endpointID {
			return d, nil
		}
	}

	d := &domain.Delivery{
		ID:             uuid.New(),
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         domain.DeliveryPending,
		CurrentAttempt: 0,
		MaxAttempts:    maxAttempts,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	s.deliveries[d.ID] = d
	return d, nil
}

func (s *MemoryStore) GetDeliveryByID(ctx context.Context, id uuid.UUID) (*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deliveries[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

func (s *MemoryStore) ListDeliveriesByApp(ctx context.Context, appID uuid.UUID, limit, offset int32) ([]*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Delivery
	for _, d := range s.deliveries {
		if ep, ok := s.endpoints[d.EndpointID]; ok && ep.AppID == appID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (s *MemoryStore) ListDeliveriesByEvent(ctx context.Context, eventID uuid.UUID) ([]*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Delivery
	for _, d := range s.deliveries {
		if d.EventID == eventID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (s *MemoryStore) UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, status domain.DeliveryStatus, currentAttempt int32, nextRetryAt, deliveredAt *time.Time) (*domain.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deliveries[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	d.Status = status
	d.CurrentAttempt = currentAttempt
	d.NextRetryAt = nextRetryAt
	d.DeliveredAt = deliveredAt
	d.UpdatedAt = time.Now().UTC()
	return d, nil
}

func (s *MemoryStore) CreateDeliveryAttempt(
	ctx context.Context, deliveryID uuid.UUID, attemptNum int32, statusCode *int32,
	latencyMS int32, reqHeaders, respHeaders []byte, respBody, errMsg *string,
) (*domain.DeliveryAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	attempt := &domain.DeliveryAttempt{
		ID:              int64(len(s.attempts) + 1),
		DeliveryID:      deliveryID,
		AttemptNumber:   attemptNum,
		StatusCode:      statusCode,
		LatencyMS:       latencyMS,
		RequestHeaders:  reqHeaders,
		ResponseHeaders: respHeaders,
		ResponseBody:    respBody,
		ErrorMessage:    errMsg,
		ExecutedAt:      time.Now().UTC(),
	}

	s.attempts = append(s.attempts, attempt)
	return attempt, nil
}

func (s *MemoryStore) ListAttemptsByDeliveryID(ctx context.Context, deliveryID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.DeliveryAttempt
	for _, a := range s.attempts {
		if a.DeliveryID == deliveryID {
			result = append(result, a)
		}
	}
	return result, nil
}

