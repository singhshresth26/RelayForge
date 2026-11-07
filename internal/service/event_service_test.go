package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/repository/memory"
	"github.com/relayforge/relayforge/internal/service"
)

func TestEventIngestionAndIdempotency(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	// Setup tenant and app
	tenant, err := store.CreateTenant(ctx, "Test Corp", "test-corp", "standard")
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	app, err := store.CreateApplication(ctx, tenant.ID, "Order Service", "Handles orders", true)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	eventSvc := service.NewEventService(store, store, store)

	// 1. Ingest valid event
	idempKey := "order-12345"
	payload := json.RawMessage(`{"order_id":"12345","amount":500}`)
	result1, err := eventSvc.IngestEvent(ctx, tenant.ID, app.ID, service.IngestEventInput{
		EventType:      "order.placed",
		Payload:        payload,
		IdempotencyKey: &idempKey,
	})
	if err != nil {
		t.Fatalf("expected successful ingestion, got %v", err)
	}
	if result1.IsDuplicate {
		t.Errorf("expected is_duplicate to be false on first call")
	}
	if result1.Event == nil || result1.Event.ID == uuid.Nil {
		t.Fatalf("expected valid event with generated uuid")
	}

	// Verify outbox entry was created
	outboxBatch, err := store.FetchPendingOutboxBatch(ctx, 10)
	if err != nil {
		t.Fatalf("failed to fetch outbox: %v", err)
	}
	if len(outboxBatch) != 1 {
		t.Fatalf("expected 1 outbox record, got %d", len(outboxBatch))
	}
	if outboxBatch[0].EventID != result1.Event.ID {
		t.Errorf("expected outbox event_id %s, got %s", result1.Event.ID, outboxBatch[0].EventID)
	}

	// 2. Ingest duplicate event with same idempotency key
	result2, err := eventSvc.IngestEvent(ctx, tenant.ID, app.ID, service.IngestEventInput{
		EventType:      "order.placed",
		Payload:        payload,
		IdempotencyKey: &idempKey,
	})
	if err != nil {
		t.Fatalf("expected duplicate request to succeed idempotently, got %v", err)
	}
	if !result2.IsDuplicate {
		t.Errorf("expected is_duplicate to be true on duplicate call")
	}
	if result2.Event.ID != result1.Event.ID {
		t.Errorf("expected same event ID %s, got %s", result1.Event.ID, result2.Event.ID)
	}

	// Ensure NO new outbox entry was created for the duplicate!
	// (outboxBatch previously fetched is now in PROCESSING or we check total outbox records)
	allPending, _ := store.FetchPendingOutboxBatch(ctx, 10)
	if len(allPending) != 0 {
		t.Errorf("expected no additional outbox record for duplicate request, got %d", len(allPending))
	}

	// 3. Test Invalid JSON payload rejected
	_, err = eventSvc.IngestEvent(ctx, tenant.ID, app.ID, service.IngestEventInput{
		EventType: "test.invalid",
		Payload:   json.RawMessage(`{invalid_json`),
	})
	if err == nil {
		t.Errorf("expected error for invalid JSON payload")
	}

	// 4. Test Oversized payload rejected (>256KB)
	largeData := strings.Repeat("A", service.MaxPayloadBytes+100)
	largePayload, _ := json.Marshal(map[string]string{"data": largeData})
	_, err = eventSvc.IngestEvent(ctx, tenant.ID, app.ID, service.IngestEventInput{
		EventType: "test.oversized",
		Payload:   largePayload,
	})
	if err == nil {
		t.Errorf("expected error for oversized payload")
	}

	// 5. Test Empty EventType rejected
	_, err = eventSvc.IngestEvent(ctx, tenant.ID, app.ID, service.IngestEventInput{
		EventType: "",
		Payload:   payload,
	})
	if err == nil {
		t.Errorf("expected error for empty event type")
	}
}
