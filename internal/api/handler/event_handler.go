package handler

import (
	"encoding/json"
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

type EventHandler struct {
	eventService *service.EventService
}

func NewEventHandler(eventService *service.EventService) *EventHandler {
	return &EventHandler{eventService: eventService}
}

// IngestEvent handles POST /api/v1/apps/{app_id}/events
func (h *EventHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
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

	var input service.IngestEventInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body JSON", nil)
		return
	}

	// Extract Idempotency-Key header if provided
	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey != "" {
		input.IdempotencyKey = &idempKey
	}

	result, err := h.eventService.IngestEvent(r.Context(), tenantID, appID, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found or inactive", nil)
			return
		}
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			response.Error(w, http.StatusBadRequest, appErr.Code, appErr.Message, appErr.Details)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INGESTION_FAILED", err.Error(), nil)
		return
	}

	status := "ACCEPTED"
	statusCode := http.StatusAccepted
	if result.IsDuplicate {
		status = "ACCEPTED_DUPLICATE"
		statusCode = http.StatusOK
	}

	response.JSON(w, statusCode, map[string]any{
		"event_id":     result.Event.ID.String(),
		"status":       status,
		"is_duplicate": result.IsDuplicate,
		"created_at":   result.Event.CreatedAt,
	})
}

// GetEvent handles GET /api/v1/apps/{app_id}/events/{id}
func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Tenant context missing", nil)
		return
	}

	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "Invalid event UUID", nil)
		return
	}

	event, err := h.eventService.GetEvent(r.Context(), eventID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Event not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, event)
}

// ListEvents handles GET /api/v1/apps/{app_id}/events
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
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

	events, err := h.eventService.ListEvents(r.Context(), appID, tenantID, limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, events)
}

