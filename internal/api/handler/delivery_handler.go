package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/api/middleware"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/service"
)

type DeliveryHandler struct {
	deliveryService *service.DeliveryService
}

func NewDeliveryHandler(deliveryService *service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{deliveryService: deliveryService}
}

// ListDeliveries handles GET /api/v1/apps/{app_id}/deliveries
func (h *DeliveryHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
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

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = int32(val)
		}
	}

	offset := int32(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = int32(val)
		}
	}

	deliveries, err := h.deliveryService.ListDeliveries(r.Context(), appID, tenantID, limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, deliveries)
}

// GetDelivery handles GET /api/v1/apps/{app_id}/deliveries/{id}
func (h *DeliveryHandler) GetDelivery(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	deliveryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid delivery UUID", nil)
		return
	}

	delivery, err := h.deliveryService.GetDelivery(r.Context(), deliveryID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Delivery not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, delivery)
}

// ListAttempts handles GET /api/v1/apps/{app_id}/deliveries/{id}/attempts
func (h *DeliveryHandler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	deliveryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid delivery UUID", nil)
		return
	}

	attempts, err := h.deliveryService.ListAttempts(r.Context(), deliveryID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Delivery not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, attempts)
}

// ReplayDelivery handles POST /api/v1/apps/{app_id}/deliveries/{id}/replay
func (h *DeliveryHandler) ReplayDelivery(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	deliveryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid delivery UUID", nil)
		return
	}

	replayed, err := h.deliveryService.ReplayDelivery(r.Context(), deliveryID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Delivery not found", nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "REPLAY_FAILED", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, replayed)
}

