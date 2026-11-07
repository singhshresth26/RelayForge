package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/api"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/repository/memory"
	"github.com/relayforge/relayforge/internal/security"
	"github.com/relayforge/relayforge/internal/service"
)

func setupTestRouter(t *testing.T) (http.Handler, *memory.MemoryStore, *service.AuthService) {
	memStore := memory.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cryptor, err := security.NewSecretCryptor("01234567890123456789012345678901")
	if err != nil {
		t.Fatalf("failed to create cryptor: %v", err)
	}

	// In test, enforce HTTPS false, but keep allowPrivateIPs false to test SSRF blocking
	ssrf := security.NewSSRFValidator(false, false)

	authService := service.NewAuthService(memStore, memStore, memStore)
	appService := service.NewAppService(memStore)
	endpointService := service.NewEndpointService(memStore, memStore, cryptor, ssrf)
	eventService := service.NewEventService(memStore, memStore, memStore)
	deliveryService := service.NewDeliveryService(memStore, memStore, memStore, memStore, nil)

	router := api.NewRouter(api.RouterConfig{
		Logger:          logger,
		AuthService:     authService,
		AppService:      appService,
		EndpointService: endpointService,
		EventService:    eventService,
		DeliveryService: deliveryService,
		APIKeyRepo:      memStore,
		ReadyCheck:      func() bool { return true },
	})

	return router, memStore, authService
}

func doRequest(handler http.Handler, method, path string, body any, apiKey string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func parseResponse(t *testing.T, rec *httptest.ResponseRecorder) response.APIResponse {
	var resp response.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v, body was: %s", err, rec.Body.String())
	}
	return resp
}

func TestHealthCheck(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	rec := doRequest(router, "GET", "/healthz", nil, "")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	recReady := doRequest(router, "GET", "/readyz", nil, "")
	if recReady.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recReady.Code)
	}
}

func TestCompletePhase1Flow(t *testing.T) {
	router, memStore, authService := setupTestRouter(t)

	// 1. Register a Tenant
	tenantInput := map[string]string{
		"name": "Acme Corp",
		"slug": "acme-corp",
		"plan": "enterprise",
	}
	rec := doRequest(router, "POST", "/api/v1/auth/tenants", tenantInput, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	tenantResp := parseResponse(t, rec)
	if !tenantResp.Success {
		t.Fatalf("expected success true")
	}

	tenantData := tenantResp.Data.(map[string]any)
	tenantIDStr := tenantData["id"].(string)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		t.Fatalf("invalid tenant uuid: %v", err)
	}

	// 2. Generate API Key directly via AuthService for initial tenant authentication
	keyCreated, err := authService.GenerateAPIKey(nil, tenantID, service.CreateAPIKeyInput{
		Name:   "Production Ingestion Key",
		IsLive: true,
		Scopes: []string{"*"},
	})
	if err != nil {
		t.Fatalf("failed to generate initial api key: %v", err)
	}
	apiKey := keyCreated.RawToken

	// 3. Test Unauthorized access without key
	unauthRec := doRequest(router, "GET", "/api/v1/apps", nil, "")
	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", unauthRec.Code)
	}

	// 4. Test Create Application with valid API key
	appInput := map[string]string{
		"name":        "Billing Service",
		"description": "Processes subscription webhooks",
	}
	recApp := doRequest(router, "POST", "/api/v1/apps", appInput, apiKey)
	if recApp.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for app, got %d: %s", recApp.Code, recApp.Body.String())
	}
	appResp := parseResponse(t, recApp)
	appData := appResp.Data.(map[string]any)
	appID := appData["id"].(string)

	// 5. Test List Applications
	recListApps := doRequest(router, "GET", "/api/v1/apps", nil, apiKey)
	if recListApps.Code != http.StatusOK {
		t.Errorf("expected 200 OK for list apps, got %d", recListApps.Code)
	}

	// 6. Test Endpoint Creation - SSRF Violation Attempt
	ssrfInput := map[string]any{
		"url":           "http://169.254.169.254/latest/meta-data", // Cloud metadata SSRF attempt
		"description":   "Malicious endpoint",
		"subscriptions": []string{"order.completed"},
	}
	recSSRF := doRequest(router, "POST", "/api/v1/apps/"+appID+"/endpoints", ssrfInput, apiKey)
	if recSSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for SSRF attempt, got %d: %s", recSSRF.Code, recSSRF.Body.String())
	}

	// 7. Test Valid Endpoint Creation
	endpointInput := map[string]any{
		"url":           "https://webhook.site/test-hook-123",
		"description":   "Main customer webhook",
		"subscriptions": []string{"order.completed", "customer.created"},
		"max_retries":   5,
		"timeout_ms":    8000,
	}
	recEP := doRequest(router, "POST", "/api/v1/apps/"+appID+"/endpoints", endpointInput, apiKey)
	if recEP.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for endpoint, got %d: %s", recEP.Code, recEP.Body.String())
	}
	epResp := parseResponse(t, recEP)
	epData := epResp.Data.(map[string]any)
	secretKey := epData["secret_key"].(string)
	if secretKey == "" || len(secretKey) < 10 {
		t.Errorf("expected valid secret_key, got %s", secretKey)
	}
	endpointInner := epData["endpoint"].(map[string]any)
	endpointID := endpointInner["id"].(string)

	// 8. Test Rotate Secret
	recRotate := doRequest(router, "POST", "/api/v1/apps/"+appID+"/endpoints/"+endpointID+"/rotate-secret", nil, apiKey)
	if recRotate.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for secret rotate, got %d: %s", recRotate.Code, recRotate.Body.String())
	}
	rotateResp := parseResponse(t, recRotate)
	rotateData := rotateResp.Data.(map[string]any)
	newSecret := rotateData["secret_key"].(string)
	if newSecret == "" || newSecret == secretKey {
		t.Errorf("expected rotated secret to be new, got %s", newSecret)
	}

	// 9. Test List Endpoints
	recListEP := doRequest(router, "GET", "/api/v1/apps/"+appID+"/endpoints", nil, apiKey)
	if recListEP.Code != http.StatusOK {
		t.Errorf("expected 200 OK for list endpoints, got %d", recListEP.Code)
	}

	// 10. Test Create Secondary API Key via HTTP
	newKeyInput := map[string]any{
		"name":    "CI/CD Key",
		"is_live": false,
		"scopes":  []string{"endpoints:read"},
	}
	recKey := doRequest(router, "POST", "/api/v1/api-keys", newKeyInput, apiKey)
	if recKey.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for key, got %d: %s", recKey.Code, recKey.Body.String())
	}
	keyResp := parseResponse(t, recKey)
	createdKeyData := keyResp.Data.(map[string]any)
	newRawToken := createdKeyData["raw_token"].(string)

	// 11. Test Revoke API Key
	apiKeyObj := createdKeyData["api_key"].(map[string]any)
	newKeyID := apiKeyObj["id"].(string)
	recRevoke := doRequest(router, "DELETE", "/api/v1/api-keys/"+newKeyID, nil, apiKey)
	if recRevoke.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for revoke key, got %d: %s", recRevoke.Code, recRevoke.Body.String())
	}

	// 12. Test that revoked key is rejected
	recRejected := doRequest(router, "GET", "/api/v1/apps", nil, newRawToken)
	if recRejected.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for revoked key, got %d", recRejected.Code)
	}

	// 13. Test Event Ingestion (Phase 2)
	// First ingestion with idempotency key
	idempReq := httptest.NewRequest("POST", "/api/v1/apps/"+appID+"/events", bytes.NewReader([]byte(`{"event_type":"invoice.paid","payload":{"invoice_id":"inv_1001","amount":29900}}`)))
	idempReq.Header.Set("Content-Type", "application/json")
	idempReq.Header.Set("Authorization", "Bearer "+apiKey)
	idempReq.Header.Set("Idempotency-Key", "idemp_inv_1001")
	recEvent := httptest.NewRecorder()
	router.ServeHTTP(recEvent, idempReq)

	if recEvent.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for event ingestion, got %d: %s", recEvent.Code, recEvent.Body.String())
	}
	eventResp := parseResponse(t, recEvent)
	eventData := eventResp.Data.(map[string]any)
	eventID := eventData["event_id"].(string)
	if eventData["is_duplicate"] != false {
		t.Errorf("expected is_duplicate to be false")
	}

	// Second ingestion with SAME idempotency key -> must be duplicate
	idempReq2 := httptest.NewRequest("POST", "/api/v1/apps/"+appID+"/events", bytes.NewReader([]byte(`{"event_type":"invoice.paid","payload":{"invoice_id":"inv_1001","amount":29900}}`)))
	idempReq2.Header.Set("Content-Type", "application/json")
	idempReq2.Header.Set("Authorization", "Bearer "+apiKey)
	idempReq2.Header.Set("Idempotency-Key", "idemp_inv_1001")
	recEvent2 := httptest.NewRecorder()
	router.ServeHTTP(recEvent2, idempReq2)

	if recEvent2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for duplicate event ingestion, got %d: %s", recEvent2.Code, recEvent2.Body.String())
	}
	eventResp2 := parseResponse(t, recEvent2)
	eventData2 := eventResp2.Data.(map[string]any)
	if eventData2["event_id"] != eventID {
		t.Errorf("expected duplicate event to return same event_id %s, got %s", eventID, eventData2["event_id"])
	}
	if eventData2["is_duplicate"] != true {
		t.Errorf("expected is_duplicate to be true on duplicate request")
	}

	// 14. Test List Events
	recListEvents := doRequest(router, "GET", "/api/v1/apps/"+appID+"/events", nil, apiKey)
	if recListEvents.Code != http.StatusOK {
		t.Errorf("expected 200 OK for list events, got %d", recListEvents.Code)
	}

	// 15. Test Get Event by ID
	recGetEvent := doRequest(router, "GET", "/api/v1/apps/"+appID+"/events/"+eventID, nil, apiKey)
	if recGetEvent.Code != http.StatusOK {
		t.Errorf("expected 200 OK for get event, got %d", recGetEvent.Code)
	}

	// 16. Test Deliveries API (Phase 5)
	recListDelivs := doRequest(router, "GET", "/api/v1/apps/"+appID+"/deliveries", nil, apiKey)
	if recListDelivs.Code != http.StatusOK {
		t.Errorf("expected 200 OK for list deliveries, got %d", recListDelivs.Code)
	}

	// 17. Ensure memory store integrity
	tenants, _ := memStore.ListTenants(nil, 10, 0)
	if len(tenants) == 0 {
		t.Errorf("expected tenants in memory store")
	}
}
