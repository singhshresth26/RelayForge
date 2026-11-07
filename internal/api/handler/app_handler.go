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

type AppHandler struct {
	appService *service.AppService
}

func NewAppHandler(appService *service.AppService) *AppHandler {
	return &AppHandler{appService: appService}
}

// CreateApplication handles POST /api/v1/apps
func (h *AppHandler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	var input service.CreateAppInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	app, err := h.appService.CreateApplication(r.Context(), tenantID, input)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "CREATE_APP_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, app)
}

// GetApplication handles GET /api/v1/apps/{id}
func (h *AppHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", nil)
		return
	}

	app, err := h.appService.GetApplication(r.Context(), appID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if app == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
		return
	}

	response.JSON(w, http.StatusOK, app)
}

// ListApplications handles GET /api/v1/apps
func (h *AppHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	apps, err := h.appService.ListApplications(r.Context(), tenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, apps)
}

// UpdateApplication handles PATCH /api/v1/apps/{id}
func (h *AppHandler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", nil)
		return
	}

	var input service.UpdateAppInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	app, err := h.appService.UpdateApplication(r.Context(), appID, tenantID, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, app)
}

// DeleteApplication handles DELETE /api/v1/apps/{id}
func (h *AppHandler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", nil)
		return
	}

	if err := h.appService.DeleteApplication(r.Context(), appID, tenantID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
