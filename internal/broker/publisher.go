package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/outbox"
)

// Ensure EventPublisher implements outbox.MessagePublisher
var _ outbox.MessagePublisher = (*EventPublisher)(nil)

type EventPublisher struct {
	client  *RabbitMQClient
	logger  *slog.Logger
	mu      sync.Mutex
	confirms chan amqp.Confirmation
}

func NewEventPublisher(client *RabbitMQClient, logger *slog.Logger) (*EventPublisher, error) {
	if logger == nil {
		logger = slog.Default()
	}

	pub := &EventPublisher{
		client: client,
		logger: logger.With(slog.String("component", "event_publisher")),
	}

	ch := client.Channel()
	if ch != nil {
		if err := ch.Confirm(false); err != nil {
			return nil, fmt.Errorf("failed to put channel into confirm mode: %w", err)
		}
		pub.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 100))
	}

	return pub, nil
}

func (p *EventPublisher) PublishEvent(ctx context.Context, event *domain.OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := p.client.Channel()
	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("rabbitmq channel is closed")
	}

	routingKey := fmt.Sprintf("event.%s.%s", event.AppID.String(), event.EventType)

	body, err := json.Marshal(map[string]any{
		"event_id":   event.EventID.String(),
		"app_id":     event.AppID.String(),
		"event_type": event.EventType,
		"payload":    event.Payload,
		"created_at": event.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal event message: %w", err)
	}

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Timestamp:    time.Now().UTC(),
		MessageId:    event.EventID.String(),
		Type:         event.EventType,
		AppId:        event.AppID.String(),
		Body:         body,
		Headers: amqp.Table{
			"outbox_id":  event.ID,
			"event_id":   event.EventID.String(),
			"app_id":     event.AppID.String(),
			"event_type": event.EventType,
		},
	}

	err = ch.PublishWithContext(
		ctx,
		ExchangeEvents,
		routingKey,
		false, // mandatory
		false, // immediate
		msg,
	)
	if err != nil {
		return fmt.Errorf("failed to publish to %s (%s): %w", ExchangeEvents, routingKey, err)
	}

	// If publisher confirms enabled, wait for ACK or timeout
	if p.confirms != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case confirm, ok := <-p.confirms:
			if !ok || !confirm.Ack {
				return fmt.Errorf("rabbitmq publisher confirm rejected or channel closed")
			}
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timed out waiting for rabbitmq publisher confirmation")
		}
	}

	return nil
}

