package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/outbox"
	"github.com/relayforge/relayforge/internal/repository/memory"
)

// MockBroker records published events
type MockBroker struct {
	mu        sync.Mutex
	published []*domain.OutboxEvent
	shouldFail bool
}

func (m *MockBroker) PublishEvent(ctx context.Context, event *domain.OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("simulated broker connection failure")
	}

	m.published = append(m.published, event)
	return nil
}

func TestOutboxPublisherProcessBatch(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()
	broker := &MockBroker{}

	publisher := outbox.NewOutboxPublisher(store, broker, outbox.PublisherConfig{
		BatchSize:    10,
		PollInterval: 10 * time.Millisecond,
		WorkerID:     "test-worker-1",
	})

	// 1. Create test outbox events
	appID := uuid.New()
	eventID1 := uuid.New()
	eventID2 := uuid.New()

	payload := json.RawMessage(`{"status":"ok"}`)
	_, _ = store.CreateOutboxEvent(ctx, eventID1, appID, "user.created", payload)
	_, _ = store.CreateOutboxEvent(ctx, eventID2, appID, "order.created", payload)

	// 2. Process batch
	count, err := publisher.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("unexpected error processing batch: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 events published, got %d", count)
	}

	if len(broker.published) != 2 {
		t.Errorf("expected broker to receive 2 events, got %d", len(broker.published))
	}

	// 3. Verify no pending outbox events remain
	pending, _ := store.FetchPendingOutboxBatch(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending outbox events after publishing, got %d", len(pending))
	}

	// 4. Test failure handling
	broker.shouldFail = true
	eventID3 := uuid.New()
	_, _ = store.CreateOutboxEvent(ctx, eventID3, appID, "payment.failed", payload)

	countFailed, err := publisher.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("unexpected error during failed batch run: %v", err)
	}
	if countFailed != 0 {
		t.Errorf("expected 0 successfully published events when broker fails, got %d", countFailed)
	}
}

