package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
)

type AppService struct {
	appRepo repository.ApplicationRepository
}

func NewAppService(appRepo repository.ApplicationRepository) *AppService {
	return &AppService{appRepo: appRepo}
}

type CreateAppInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *AppService) CreateApplication(ctx context.Context, tenantID uuid.UUID, input CreateAppInput) (*domain.Application, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.NewAppError("INVALID_NAME", "application name cannot be empty", domain.ErrInvalidInput)
	}

	return s.appRepo.CreateApplication(ctx, tenantID, name, input.Description, true)
}

func (s *AppService) GetApplication(ctx context.Context, id, tenantID uuid.UUID) (*domain.Application, error) {
	return s.appRepo.GetApplicationByID(ctx, id, tenantID)
}

func (s *AppService) ListApplications(ctx context.Context, tenantID uuid.UUID) ([]*domain.Application, error) {
	return s.appRepo.ListApplicationsByTenant(ctx, tenantID)
}

type UpdateAppInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

func (s *AppService) UpdateApplication(ctx context.Context, id, tenantID uuid.UUID, input UpdateAppInput) (*domain.Application, error) {
	existing, err := s.appRepo.GetApplicationByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, domain.ErrNotFound
	}

	name := existing.Name
	if strings.TrimSpace(input.Name) != "" {
		name = strings.TrimSpace(input.Name)
	}

	desc := existing.Description
	if input.Description != "" {
		desc = input.Description
	}

	isActive := existing.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	return s.appRepo.UpdateApplication(ctx, id, tenantID, name, desc, isActive)
}

func (s *AppService) DeleteApplication(ctx context.Context, id, tenantID uuid.UUID) error {
	return s.appRepo.DeleteApplication(ctx, id, tenantID)
}

