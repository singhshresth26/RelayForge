package fanout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/relayforge/relayforge/internal/broker"
	"github.com/relayforge/relayforge/internal/repository"
)

type IngestedEventMessage struct {
	EventID   uuid.UUID       `json:"event_id"`
	AppID     uuid.UUID       `json:"app_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"created_at"`
}

type DeliveryJob struct {
	DeliveryID      uuid.UUID       `json:"delivery_id"`
	EventID         uuid.UUID       `json:"event_id"`
	EndpointID      uuid.UUID       `json:"endpoint_id"`
	URL             string          `json:"url"`
	SecretEncrypted string          `json:"secret_encrypted"`
	EventType       string          `json:"event_type"`
	Payload         json.RawMessage `json:"payload"`
	CurrentAttempt  int32           `json:"current_attempt"`
	MaxAttempts     int32           `json:"max_attempts"`
	TimeoutMS       int32           `json:"timeout_ms"`
	RateLimitRPS    int32           `json:"rate_limit_rps"`
}

type DeliveryPublisher interface {
	PublishDelivery(ctx context.Context, job *DeliveryJob) error
}

type AMQPDeliveryPublisher struct {
	client *broker.RabbitMQClient
}

func NewAMQPDeliveryPublisher(client *broker.RabbitMQClient) *AMQPDeliveryPublisher {
	return &AMQPDeliveryPublisher{client: client}
}

func (p *AMQPDeliveryPublisher) PublishDelivery(ctx context.Context, job *DeliveryJob) error {
	ch := p.client.Channel()
	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("rabbitmq channel is closed")
	}

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery job: %w", err)
	}

	return ch.PublishWithContext(
		ctx,
		broker.ExchangeDeliveries,
		broker.RoutingKeyDeliveryProcess,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			MessageId:    job.DeliveryID.String(),
			Body:         body,
			Headers: amqp.Table{
				"delivery_id": job.DeliveryID.String(),
				"endpoint_id": job.EndpointID.String(),
				"attempt":     job.CurrentAttempt,
			},
		},
	)
}

type FanoutProcessor struct {
	endpointRepo repository.EndpointRepository
	deliveryRepo repository.DeliveryRepository
	publisher    DeliveryPublisher
	logger       *slog.Logger
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewFanoutProcessor(
	endpointRepo repository.EndpointRepository,
	deliveryRepo repository.DeliveryRepository,
	publisher DeliveryPublisher,
	logger *slog.Logger,
) *FanoutProcessor {
	if logger == nil {
		logger = slog.Default()
	}

	return &FanoutProcessor{
		endpointRepo: endpointRepo,
		deliveryRepo: deliveryRepo,
		publisher:    publisher,
		logger:       logger.With(slog.String("component", "fanout_processor")),
		stopCh:       make(chan struct{}),
	}
}

// ProcessEvent processes a single incoming event, finding matching endpoints and queueing deliveries
func (p *FanoutProcessor) ProcessEvent(ctx context.Context, msg IngestedEventMessage) (int, error) {
	endpoints, err := p.endpointRepo.ListActiveEndpointsForEvent(ctx, msg.AppID, msg.EventType)
	if err != nil {
		return 0, fmt.Errorf("failed to query active endpoints for app %s, event %s: %w", msg.AppID, msg.EventType, err)
	}

	if len(endpoints) == 0 {
		p.logger.Debug("No active endpoints registered for event",
			slog.String("app_id", msg.AppID.String()),
			slog.String("event_type", msg.EventType),
		)
		return 0, nil
	}

	queuedCount := 0
	for _, ep := range endpoints {
		// Create delivery record in database (deduplicated by UNIQUE(event_id, endpoint_id))
		delivery, err := p.deliveryRepo.CreateDelivery(ctx, msg.EventID, ep.ID, ep.MaxRetries)
		if err != nil {
			p.logger.Error("Failed to record delivery in DB",
				slog.String("event_id", msg.EventID.String()),
				slog.String("endpoint_id", ep.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		job := &DeliveryJob{
			DeliveryID:      delivery.ID,
			EventID:         msg.EventID,
			EndpointID:      ep.ID,
			URL:             ep.URL,
			SecretEncrypted: ep.SecretEncrypted,
			EventType:       msg.EventType,
			Payload:         msg.Payload,
			CurrentAttempt:  0,
			MaxAttempts:     ep.MaxRetries,
			TimeoutMS:       ep.TimeoutMS,
			RateLimitRPS:    ep.RateLimitRPS,
		}

		if p.publisher != nil {
			if err := p.publisher.PublishDelivery(ctx, job); err != nil {
				p.logger.Error("Failed to publish delivery job",
					slog.String("delivery_id", delivery.ID.String()),
					slog.String("error", err.Error()),
				)
				continue
			}
		}

		queuedCount++
	}

	p.logger.Info("Event successfully fanned out",
		slog.String("event_id", msg.EventID.String()),
		slog.String("event_type", msg.EventType),
		slog.Int("endpoints_matched", len(endpoints)),
		slog.Int("deliveries_queued", queuedCount),
	)

	return queuedCount, nil
}

// StartConsumer starts consuming from RabbitMQ queue
func (p *FanoutProcessor) StartConsumer(ctx context.Context, ch *amqp.Channel) error {
	// Set prefetch QoS
	if err := ch.Qos(50, 0, false); err != nil {
		return fmt.Errorf("failed to set prefetch QoS: %w", err)
	}

	deliveries, err := ch.Consume(
		broker.QueueEventsFanout,
		"fanout-worker",
		false, // auto-ack false (manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from %s: %w", broker.QueueEventsFanout, err)
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.logger.Info("Fan-out consumer running on " + broker.QueueEventsFanout)

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case d, ok := <-deliveries:
				if !ok {
					p.logger.Warn("RabbitMQ deliveries channel closed")
					return
				}

				var msg IngestedEventMessage
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					p.logger.Error("Failed to unmarshal event message", slog.String("error", err.Error()))
					_ = d.Nack(false, false) // dead-letter invalid message
					continue
				}

				_, err := p.ProcessEvent(ctx, msg)
				if err != nil {
					p.logger.Error("Fan-out processing error", slog.String("error", err.Error()))
					_ = d.Nack(false, true) // requeue transient errors
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	return nil
}

func (p *FanoutProcessor) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	p.logger.Info("Fan-out processor stopped cleanly")
}

