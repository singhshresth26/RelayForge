package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/api/middleware"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/service"
)

type EndpointHandler struct {
	endpointService *service.EndpointService
}

func NewEndpointHandler(endpointService *service.EndpointService) *EndpointHandler {
	return &EndpointHandler{endpointService: endpointService}
}

// CreateEndpoint handles POST /api/v1/apps/{app_id}/endpoints
func (h *EndpointHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "app_id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", nil)
		return
	}

	var input service.CreateEndpointInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	created, err := h.endpointService.CreateEndpoint(r.Context(), tenantID, appID, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		if errors.Is(err, domain.ErrSSRFBlocked) {
			response.Error(w, http.StatusForbidden, "SSRF_BLOCKED", err.Error(), nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "CREATE_ENDPOINT_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, created)
}

// GetEndpoint handles GET /api/v1/apps/{app_id}/endpoints/{id}
func (h *EndpointHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid endpoint UUID", nil)
		return
	}

	endpoint, err := h.endpointService.GetEndpoint(r.Context(), endpointID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, endpoint)
}

// ListEndpoints handles GET /api/v1/apps/{app_id}/endpoints
func (h *EndpointHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "app_id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", nil)
		return
	}

	endpoints, err := h.endpointService.ListEndpoints(r.Context(), appID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, endpoints)
}

// UpdateEndpoint handles PATCH /api/v1/apps/{app_id}/endpoints/{id}
func (h *EndpointHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid endpoint UUID", nil)
		return
	}

	var input service.UpdateEndpointInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	endpoint, err := h.endpointService.UpdateEndpoint(r.Context(), endpointID, tenantID, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		if errors.Is(err, domain.ErrSSRFBlocked) {
			response.Error(w, http.StatusForbidden, "SSRF_BLOCKED", err.Error(), nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, endpoint)
}

// RotateSecret handles POST /api/v1/apps/{app_id}/endpoints/{id}/rotate-secret
func (h *EndpointHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid endpoint UUID", nil)
		return
	}

	rotated, err := h.endpointService.RotateSecret(r.Context(), endpointID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "ROTATION_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, rotated)
}

// DeleteEndpoint handles DELETE /api/v1/apps/{app_id}/endpoints/{id}
func (h *EndpointHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid endpoint UUID", nil)
		return
	}

	if err := h.endpointService.DeleteEndpoint(r.Context(), endpointID, tenantID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
