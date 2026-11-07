package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/security"
)

type EndpointService struct {
	endpointRepo repository.EndpointRepository
	appRepo      repository.ApplicationRepository
	cryptor      *security.SecretCryptor
	ssrf         *security.SSRFValidator
}

func NewEndpointService(
	endpointRepo repository.EndpointRepository,
	appRepo repository.ApplicationRepository,
	cryptor *security.SecretCryptor,
	ssrf *security.SSRFValidator,
) *EndpointService {
	return &EndpointService{
		endpointRepo: endpointRepo,
		appRepo:      appRepo,
		cryptor:      cryptor,
		ssrf:         ssrf,
	}
}

type CreateEndpointInput struct {
	URL           string   `json:"url"`
	Description   string   `json:"description"`
	Subscriptions []string `json:"subscriptions"`
	MaxRetries    *int32   `json:"max_retries,omitempty"`
	TimeoutMS     *int32   `json:"timeout_ms,omitempty"`
	RateLimitRPS  *int32   `json:"rate_limit_rps,omitempty"`
}

func (s *EndpointService) CreateEndpoint(ctx context.Context, tenantID, appID uuid.UUID, input CreateEndpointInput) (*domain.EndpointCreated, error) {
	// Verify application ownership
	app, err := s.appRepo.GetApplicationByID(ctx, appID, tenantID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, domain.ErrNotFound
	}

	targetURL := strings.TrimSpace(input.URL)
	if targetURL == "" {
		return nil, domain.NewAppError("INVALID_URL", "webhook url is required", domain.ErrInvalidInput)
	}

	// Validate SSRF
	if err := s.ssrf.ValidateURL(targetURL); err != nil {
		return nil, domain.NewAppError("SSRF_VIOLATION", err.Error(), domain.ErrSSRFBlocked)
	}

	// Generate signing secret
	rawSecret, err := security.GenerateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	encryptedSecret, err := s.cryptor.Encrypt(rawSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	maxRetries := int32(5)
	if input.MaxRetries != nil {
		if *input.MaxRetries < 0 || *input.MaxRetries > 20 {
			return nil, domain.NewAppError("INVALID_MAX_RETRIES", "max_retries must be between 0 and 20", domain.ErrInvalidInput)
		}
		maxRetries = *input.MaxRetries
	}

	timeoutMS := int32(10000)
	if input.TimeoutMS != nil {
		if *input.TimeoutMS < 1000 || *input.TimeoutMS > 30000 {
			return nil, domain.NewAppError("INVALID_TIMEOUT", "timeout_ms must be between 1000 and 30000", domain.ErrInvalidInput)
		}
		timeoutMS = *input.TimeoutMS
	}

	rateLimitRPS := int32(50)
	if input.RateLimitRPS != nil {
		if *input.RateLimitRPS <= 0 {
			return nil, domain.NewAppError("INVALID_RATE_LIMIT", "rate_limit_rps must be greater than 0", domain.ErrInvalidInput)
		}
		rateLimitRPS = *input.RateLimitRPS
	}

	subs := input.Subscriptions
	if len(subs) == 0 {
		subs = []string{"*"} // Default subscribe to all events
	}

	endpoint, err := s.endpointRepo.CreateEndpoint(
		ctx, appID, targetURL, encryptedSecret, input.Description,
		true, maxRetries, timeoutMS, rateLimitRPS, subs,
	)
	if err != nil {
		return nil, err
	}

	return &domain.EndpointCreated{
		Endpoint:  endpoint,
		SecretKey: rawSecret,
	}, nil
}

func (s *EndpointService) GetEndpoint(ctx context.Context, id, tenantID uuid.UUID) (*domain.Endpoint, error) {
	ep, err := s.endpointRepo.GetEndpointWithApp(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, domain.ErrNotFound
	}
	return ep, nil
}

func (s *EndpointService) ListEndpoints(ctx context.Context, appID, tenantID uuid.UUID) ([]*domain.Endpoint, error) {
	app, err := s.appRepo.GetApplicationByID(ctx, appID, tenantID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, domain.ErrNotFound
	}
	return s.endpointRepo.ListEndpointsByApp(ctx, appID)
}

type UpdateEndpointInput struct {
	URL           *string  `json:"url,omitempty"`
	Description   *string  `json:"description,omitempty"`
	IsActive      *bool    `json:"is_active,omitempty"`
	MaxRetries    *int32   `json:"max_retries,omitempty"`
	TimeoutMS     *int32   `json:"timeout_ms,omitempty"`
	RateLimitRPS  *int32   `json:"rate_limit_rps,omitempty"`
	Subscriptions []string `json:"subscriptions,omitempty"`
}

func (s *EndpointService) UpdateEndpoint(ctx context.Context, id, tenantID uuid.UUID, input UpdateEndpointInput) (*domain.Endpoint, error) {
	existing, err := s.endpointRepo.GetEndpointWithApp(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, domain.ErrNotFound
	}

	targetURL := existing.URL
	if input.URL != nil && strings.TrimSpace(*input.URL) != "" {
		targetURL = strings.TrimSpace(*input.URL)
		if err := s.ssrf.ValidateURL(targetURL); err != nil {
			return nil, domain.NewAppError("SSRF_VIOLATION", err.Error(), domain.ErrSSRFBlocked)
		}
	}

	desc := existing.Description
	if input.Description != nil {
		desc = *input.Description
	}

	isActive := existing.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	maxRetries := existing.MaxRetries
	if input.MaxRetries != nil {
		maxRetries = *input.MaxRetries
	}

	timeoutMS := existing.TimeoutMS
	if input.TimeoutMS != nil {
		timeoutMS = *input.TimeoutMS
	}

	rateLimitRPS := existing.RateLimitRPS
	if input.RateLimitRPS != nil {
		rateLimitRPS = *input.RateLimitRPS
	}

	subs := existing.Subscriptions
	if input.Subscriptions != nil {
		subs = input.Subscriptions
	}

	return s.endpointRepo.UpdateEndpoint(
		ctx, id, targetURL, desc, isActive, maxRetries, timeoutMS, rateLimitRPS, subs,
	)
}

func (s *EndpointService) RotateSecret(ctx context.Context, id, tenantID uuid.UUID) (*domain.EndpointCreated, error) {
	existing, err := s.endpointRepo.GetEndpointWithApp(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, domain.ErrNotFound
	}

	newSecret, err := security.GenerateWebhookSecret()
	if err != nil {
		return nil, err
	}

	encrypted, err := s.cryptor.Encrypt(newSecret)
	if err != nil {
		return nil, err
	}

	updated, err := s.endpointRepo.UpdateEndpointSecret(ctx, id, encrypted)
	if err != nil {
		return nil, err
	}

	return &domain.EndpointCreated{
		Endpoint:  updated,
		SecretKey: newSecret,
	}, nil
}

func (s *EndpointService) DeleteEndpoint(ctx context.Context, id, tenantID uuid.UUID) error {
	existing, err := s.endpointRepo.GetEndpointWithApp(ctx, id, tenantID)
	if err != nil {
		return err
	}
	if existing == nil {
		return domain.ErrNotFound
	}
	return s.endpointRepo.DeleteEndpoint(ctx, id)
}

