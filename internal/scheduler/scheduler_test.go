package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository/memory"
	"github.com/relayforge/relayforge/internal/scheduler"
)

func TestCalculateBackoff(t *testing.T) {
	cfg := scheduler.BackoffConfig{
		BaseDelay: 2 * time.Second,
		MaxDelay:  30 * time.Second,
	}

	d0 := scheduler.CalculateBackoff(cfg, 0)
	d1 := scheduler.CalculateBackoff(cfg, 1)
	d2 := scheduler.CalculateBackoff(cfg, 2)
	d3 := scheduler.CalculateBackoff(cfg, 3)

	if d0 < 2*time.Second || d0 > 4*time.Second {
		t.Errorf("attempt 0 backoff %v out of expected range [2s, 4s]", d0)
	}

	if d1 < 4*time.Second || d1 > 6*time.Second {
		t.Errorf("attempt 1 backoff %v out of expected range [4s, 6s]", d1)
	}

	if d2 < 8*time.Second || d2 > 10*time.Second {
		t.Errorf("attempt 2 backoff %v out of expected range [8s, 10s]", d2)
	}

	// High attempt number should cap at MaxDelay
	d10 := scheduler.CalculateBackoff(cfg, 10)
	if d10 > cfg.MaxDelay {
		t.Errorf("expected backoff to be capped at %v, got %v", cfg.MaxDelay, d10)
	}

	t.Logf("Calculated backoffs: att0=%v, att1=%v, att2=%v, att3=%v, att10=%v", d0, d1, d2, d3, d10)
}

func TestAMQPRetryDispatcher(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	cfg := scheduler.BackoffConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  10 * time.Second,
	}
	dispatcher := scheduler.NewAMQPRetryDispatcher(nil, store, cfg, nil)

	eventID := uuid.New()
	endpointID := uuid.New()

	delivery, err := store.CreateDelivery(ctx, eventID, endpointID, 5)
	if err != nil {
		t.Fatalf("failed to create delivery: %v", err)
	}

	job := &fanout.DeliveryJob{
		DeliveryID:  delivery.ID,
		EventID:     eventID,
		EndpointID:  endpointID,
		MaxAttempts: 5,
	}

	// 1. Schedule Retry
	if err := dispatcher.ScheduleRetry(ctx, job, 1); err != nil {
		t.Fatalf("unexpected error scheduling retry: %v", err)
	}

	updated, _ := store.GetDeliveryByID(ctx, delivery.ID)
	if updated == nil {
		t.Fatalf("expected delivery in store")
	}
	if updated.Status != domain.DeliveryRetrying {
		t.Errorf("expected status RETRYING, got %s", updated.Status)
	}
	if updated.NextRetryAt == nil {
		t.Errorf("expected next_retry_at to be populated")
	}

	// 2. Route to DLQ
	if err := dispatcher.RouteToDLQ(ctx, job, "Max retry limit reached"); err != nil {
		t.Fatalf("unexpected error routing to DLQ: %v", err)
	}

	updatedDLQ, _ := store.GetDeliveryByID(ctx, delivery.ID)
	if updatedDLQ == nil {
		t.Fatalf("expected delivery in store after DLQ")
	}
	if updatedDLQ.Status != domain.DeliveryFailed {
		t.Errorf("expected status FAILED, got %s", updatedDLQ.Status)
	}
}
