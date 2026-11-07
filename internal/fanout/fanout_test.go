package fanout_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository/memory"
)

type MockDeliveryPublisher struct {
	mu   sync.Mutex
	jobs []*fanout.DeliveryJob
}

func (m *MockDeliveryPublisher) PublishDelivery(ctx context.Context, job *fanout.DeliveryJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
	return nil
}

func TestFanoutProcessor(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()
	mockPub := &MockDeliveryPublisher{}

	appID := uuid.New()

	// 1. Register Endpoint A subscribed to "order.created"
	epA, err := store.CreateEndpoint(
		ctx, appID, "https://api.example.com/order-hook", "encrypted_secret_a",
		"Order Webhook", true, 5, 10000, 50, []string{"order.created"},
	)
	if err != nil {
		t.Fatalf("failed to create endpoint A: %v", err)
	}

	// 2. Register Endpoint B subscribed to "*" (wildcard)
	epB, err := store.CreateEndpoint(
		ctx, appID, "https://api.example.com/wildcard-hook", "encrypted_secret_b",
		"Audit Webhook", true, 3, 5000, 100, []string{"*"},
	)
	if err != nil {
		t.Fatalf("failed to create endpoint B: %v", err)
	}

	// 3. Register Endpoint C subscribed to "invoice.paid" (should NOT receive "order.created")
	_, err = store.CreateEndpoint(
		ctx, appID, "https://api.example.com/invoice-hook", "encrypted_secret_c",
		"Invoice Webhook", true, 5, 10000, 50, []string{"invoice.paid"},
	)
	if err != nil {
		t.Fatalf("failed to create endpoint C: %v", err)
	}

	processor := fanout.NewFanoutProcessor(store, store, mockPub, nil)

	// 4. Ingest event "order.created"
	eventID := uuid.New()
	payload := json.RawMessage(`{"order_id":"123","total":99.99}`)
	count, err := processor.ProcessEvent(ctx, fanout.IngestedEventMessage{
		EventID:   eventID,
		AppID:     appID,
		EventType: "order.created",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("failed to process event fanout: %v", err)
	}

	// Should match Endpoint A (exact) and Endpoint B (wildcard) = 2 deliveries
	if count != 2 {
		t.Errorf("expected 2 endpoints matched, got %d", count)
	}

	if len(mockPub.jobs) != 2 {
		t.Fatalf("expected 2 published delivery jobs, got %d", len(mockPub.jobs))
	}

	matchedEndpoints := make(map[uuid.UUID]bool)
	for _, job := range mockPub.jobs {
		matchedEndpoints[job.EndpointID] = true
		if job.EventID != eventID {
			t.Errorf("expected event_id %s, got %s", eventID, job.EventID)
		}
	}

	if !matchedEndpoints[epA.ID] {
		t.Errorf("expected Endpoint A to receive delivery job")
	}
	if !matchedEndpoints[epB.ID] {
		t.Errorf("expected Endpoint B to receive delivery job")
	}

	// 5. Test event with no matching endpoints
	countUnmatched, err := processor.ProcessEvent(ctx, fanout.IngestedEventMessage{
		EventID:   uuid.New(),
		AppID:     uuid.New(), // non-existent app
		EventType: "some.random.event",
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("unexpected error for unmatched event: %v", err)
	}
	if countUnmatched != 0 {
		t.Errorf("expected 0 deliveries for unmatched event, got %d", countUnmatched)
	}
}

