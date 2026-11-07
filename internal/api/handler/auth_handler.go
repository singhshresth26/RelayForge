package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/api/middleware"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterTenant handles POST /api/v1/auth/tenants
func (h *AuthHandler) RegisterTenant(w http.ResponseWriter, r *http.Request) {
	var input service.CreateTenantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	tenant, err := h.authService.RegisterTenant(r.Context(), input)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, tenant)
}

// CreateAPIKey handles POST /api/v1/api-keys
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	var input service.CreateAPIKeyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	apiKeyCreated, err := h.authService.GenerateAPIKey(r.Context(), tenantID, input)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "KEY_CREATION_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, apiKeyCreated)
}

// ListAPIKeys handles GET /api/v1/api-keys
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	keys, err := h.authService.ListAPIKeys(r.Context(), tenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, keys)
}

// RevokeAPIKey handles DELETE /api/v1/api-keys/{id}
func (h *AuthHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid API key UUID", nil)
		return
	}

	revoked, err := h.authService.RevokeAPIKey(r.Context(), keyID, tenantID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "REVOKE_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, revoked)
}
