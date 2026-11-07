package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository/memory"
	"github.com/relayforge/relayforge/internal/security"
	"github.com/relayforge/relayforge/internal/worker"
)

type MockRetryDispatcher struct {
	mu          sync.Mutex
	retries     []*fanout.DeliveryJob
	dlqJobs     []*fanout.DeliveryJob
	dlqReasons  []string
}

func (m *MockRetryDispatcher) ScheduleRetry(ctx context.Context, job *fanout.DeliveryJob, attempt int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retries = append(m.retries, job)
	return nil
}

func (m *MockRetryDispatcher) RouteToDLQ(ctx context.Context, job *fanout.DeliveryJob, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dlqJobs = append(m.dlqJobs, job)
	m.dlqReasons = append(m.dlqReasons, reason)
	return nil
}

func TestWorkerPoolExecution(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()
	mockRetry := &MockRetryDispatcher{}

	masterKey := "01234567890123456789012345678901"
	cryptor, _ := security.NewSecretCryptor(masterKey)
	rawSecret := "whsec_test_signing_secret_999"
	encryptedSecret, _ := cryptor.Encrypt(rawSecret)

	// 1. Mock Webhook Server that verifies HMAC-SHA256 signature
	var receivedHeaders http.Header
	var receivedBody []byte
	var serverResponseCode = http.StatusOK

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		receivedBody, _ = io.ReadAll(r.Body)

		// Validate HMAC signature
		sigHeader := r.Header.Get("X-RelayForge-Signature")
		tsStr := r.Header.Get("X-RelayForge-Timestamp")
		ts, _ := strconv.ParseInt(tsStr, 10, 64)

		if !security.VerifySignature(rawSecret, sigHeader, ts, receivedBody, 0) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"invalid_signature"}`))
			return
		}

		w.WriteHeader(serverResponseCode)
		if serverResponseCode == http.StatusOK {
			_, _ = w.Write([]byte(`{"received":true}`))
		} else {
			_, _ = w.Write([]byte(`{"error":"internal_server_error"}`))
		}
	}))
	defer server.Close()

	// In test, allow private/loopback so httptest.Server is accessible
	ssrf := security.NewSSRFValidator(true, false)
	client := worker.NewHTTPDeliveryClient(cryptor, ssrf)
	pool := worker.NewWorkerPool(client, store, mockRetry, 5, nil)

	eventID := uuid.New()
	endpointID := uuid.New()
	delivery, err := store.CreateDelivery(ctx, eventID, endpointID, 3)
	if err != nil {
		t.Fatalf("failed to create delivery record: %v", err)
	}

	payload := json.RawMessage(`{"event":"order.completed","amount":5000}`)
	job := &fanout.DeliveryJob{
		DeliveryID:      delivery.ID,
		EventID:         eventID,
		EndpointID:      endpointID,
		URL:             server.URL,
		SecretEncrypted: encryptedSecret,
		EventType:       "order.completed",
		Payload:         payload,
		CurrentAttempt:  0,
		MaxAttempts:     3,
		TimeoutMS:       5000,
	}

	// 2. Test Successful Delivery (200 OK)
	serverResponseCode = http.StatusOK
	if err := pool.ProcessJob(ctx, job); err != nil {
		t.Fatalf("unexpected error processing job: %v", err)
	}

	// Verify Delivery status in database transitioned to DELIVERED
	updatedDeliv, _ := store.GetDeliveryByID(ctx, delivery.ID)
	if updatedDeliv.Status != domain.DeliveryDelivered {
		t.Errorf("expected delivery status DELIVERED, got %s", updatedDeliv.Status)
	}
	if updatedDeliv.DeliveredAt == nil {
		t.Errorf("expected delivered_at to be recorded")
	}

	// Verify attempt logged in database
	if receivedHeaders.Get("User-Agent") != worker.DefaultUserAgent {
		t.Errorf("expected User-Agent %s, got %s", worker.DefaultUserAgent, receivedHeaders.Get("User-Agent"))
	}
	attempts, _ := store.ListAttemptsByDeliveryID(ctx, delivery.ID)
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt logged, got %d", len(attempts))
	}
	if *attempts[0].StatusCode != 200 {
		t.Errorf("expected attempt status code 200, got %d", *attempts[0].StatusCode)
	}
	if *attempts[0].ResponseBody != `{"received":true}` {
		t.Errorf("unexpected response body log: %s", *attempts[0].ResponseBody)
	}

	// 3. Test Failure with Retry (500 Internal Server Error)
	serverResponseCode = http.StatusInternalServerError
	delivery2, _ := store.CreateDelivery(ctx, uuid.New(), endpointID, 3)
	job2 := &fanout.DeliveryJob{
		DeliveryID:      delivery2.ID,
		EventID:         uuid.New(),
		EndpointID:      endpointID,
		URL:             server.URL,
		SecretEncrypted: encryptedSecret,
		EventType:       "order.completed",
		Payload:         payload,
		CurrentAttempt:  0,
		MaxAttempts:     3,
		TimeoutMS:       5000,
	}

	_ = pool.ProcessJob(ctx, job2)
	updatedDeliv2, _ := store.GetDeliveryByID(ctx, delivery2.ID)
	if updatedDeliv2.Status != domain.DeliveryRetrying {
		t.Errorf("expected delivery status RETRYING, got %s", updatedDeliv2.Status)
	}
	if len(mockRetry.retries) != 1 {
		t.Errorf("expected 1 retry scheduled, got %d", len(mockRetry.retries))
	}

	// 4. Test Final Failure -> DLQ Routing (CurrentAttempt = 2 of MaxAttempts = 3)
	delivery3, _ := store.CreateDelivery(ctx, uuid.New(), endpointID, 3)
	job3 := &fanout.DeliveryJob{
		DeliveryID:      delivery3.ID,
		EventID:         uuid.New(),
		EndpointID:      endpointID,
		URL:             server.URL,
		SecretEncrypted: encryptedSecret,
		EventType:       "order.completed",
		Payload:         payload,
		CurrentAttempt:  2, // This is attempt 3 of 3 (final attempt)
		MaxAttempts:     3,
		TimeoutMS:       5000,
	}

	_ = pool.ProcessJob(ctx, job3)
	updatedDeliv3, _ := store.GetDeliveryByID(ctx, delivery3.ID)
	if updatedDeliv3.Status != domain.DeliveryFailed {
		t.Errorf("expected delivery status FAILED, got %s", updatedDeliv3.Status)
	}
	if len(mockRetry.dlqJobs) != 1 {
		t.Errorf("expected 1 job routed to DLQ, got %d", len(mockRetry.dlqJobs))
	}
}
