package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/security"
)

type AuthService struct {
	tenantRepo repository.TenantRepository
	apiKeyRepo repository.APIKeyRepository
	userRepo   repository.UserRepository
}

func NewAuthService(tenantRepo repository.TenantRepository, apiKeyRepo repository.APIKeyRepository, userRepo repository.UserRepository) *AuthService {
	return &AuthService{
		tenantRepo: tenantRepo,
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
	}
}

type CreateTenantInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

func (s *AuthService) RegisterTenant(ctx context.Context, input CreateTenantInput) (*domain.Tenant, error) {
	if input.Name == "" || input.Slug == "" {
		return nil, domain.NewAppError("INVALID_INPUT", "name and slug are required", domain.ErrInvalidInput)
	}
	if input.Plan == "" {
		input.Plan = "standard"
	}
	return s.tenantRepo.CreateTenant(ctx, input.Name, input.Slug, input.Plan)
}

type CreateAPIKeyInput struct {
	Name      string     `json:"name"`
	IsLive    bool       `json:"is_live"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (s *AuthService) GenerateAPIKey(ctx context.Context, tenantID uuid.UUID, input CreateAPIKeyInput) (*domain.APIKeyCreated, error) {
	if input.Name == "" {
		return nil, domain.NewAppError("INVALID_INPUT", "name is required for api key", domain.ErrInvalidInput)
	}
	if len(input.Scopes) == 0 {
		input.Scopes = []string{"events:publish", "endpoints:read"}
	}

	rawToken, prefix, hash, err := security.GenerateAPIKey(input.IsLive)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	apiKey, err := s.apiKeyRepo.CreateAPIKey(ctx, tenantID, input.Name, prefix, hash, input.Scopes, input.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &domain.APIKeyCreated{
		APIKey:   apiKey,
		RawToken: rawToken,
	}, nil
}

func (s *AuthService) ListAPIKeys(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.ListAPIKeysByTenant(ctx, tenantID)
}

func (s *AuthService) RevokeAPIKey(ctx context.Context, id, tenantID uuid.UUID) (*domain.APIKey, error) {
	return s.apiKeyRepo.RevokeAPIKey(ctx, id, tenantID)
}

