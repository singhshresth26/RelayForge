package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/relayforge/relayforge/internal/broker"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/worker"
)

// Ensure AMQPRetryDispatcher implements worker.RetryDispatcher
var _ worker.RetryDispatcher = (*AMQPRetryDispatcher)(nil)

type AMQPRetryDispatcher struct {
	client       *broker.RabbitMQClient
	deliveryRepo repository.DeliveryRepository
	backoffCfg   BackoffConfig
	logger       *slog.Logger
}

func NewAMQPRetryDispatcher(
	client *broker.RabbitMQClient,
	deliveryRepo repository.DeliveryRepository,
	backoffCfg BackoffConfig,
	logger *slog.Logger,
) *AMQPRetryDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &AMQPRetryDispatcher{
		client:       client,
		deliveryRepo: deliveryRepo,
		backoffCfg:   backoffCfg,
		logger:       logger.With(slog.String("component", "retry_dispatcher")),
	}
}

func (d *AMQPRetryDispatcher) ScheduleRetry(ctx context.Context, job *fanout.DeliveryJob, attempt int32) error {
	delay := CalculateBackoff(d.backoffCfg, attempt)
	nextRetryAt := time.Now().UTC().Add(delay)

	// 1. Update database state
	if d.deliveryRepo != nil {
		_, err := d.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryRetrying, attempt, &nextRetryAt, nil)
		if err != nil {
			d.logger.Error("Failed to update delivery retry state in DB",
				slog.String("delivery_id", job.DeliveryID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	// 2. Publish to RabbitMQ Retries Exchange with per-message TTL
	if d.client == nil {
		return nil
	}

	ch := d.client.Channel()
	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("rabbitmq channel closed")
	}

	// Increment attempt counter in job payload for next round
	job.CurrentAttempt = attempt
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal retry job: %w", err)
	}

	ttlMS := delay.Milliseconds()
	if ttlMS < 1000 {
		ttlMS = 1000
	}

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    job.DeliveryID.String(),
		Expiration:   strconv.FormatInt(ttlMS, 10), // Per-message TTL in milliseconds
		Body:         body,
		Headers: amqp.Table{
			"delivery_id": job.DeliveryID.String(),
			"attempt":     attempt,
			"retry_delay": delay.String(),
		},
	}

	err = ch.PublishWithContext(
		ctx,
		broker.ExchangeRetries,
		broker.RoutingKeyDeliveryRetry,
		false,
		false,
		msg,
	)
	if err != nil {
		return fmt.Errorf("failed to publish retry to %s: %w", broker.ExchangeRetries, err)
	}

	d.logger.Info("Delivery retry scheduled",
		slog.String("delivery_id", job.DeliveryID.String()),
		slog.Int("attempt", int(attempt)),
		slog.Duration("delay", delay),
		slog.Time("next_retry_at", nextRetryAt),
	)

	return nil
}

func (d *AMQPRetryDispatcher) RouteToDLQ(ctx context.Context, job *fanout.DeliveryJob, reason string) error {
	// 1. Update database state to FAILED
	if d.deliveryRepo != nil {
		_, err := d.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryFailed, job.CurrentAttempt, nil, nil)
		if err != nil {
			d.logger.Error("Failed to update delivery failed state in DB",
				slog.String("delivery_id", job.DeliveryID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	// 2. Publish to RabbitMQ DLQ
	if d.client == nil {
		return nil
	}

	ch := d.client.Channel()
	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("rabbitmq channel closed")
	}

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal dlq job: %w", err)
	}

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    job.DeliveryID.String(),
		Body:         body,
		Headers: amqp.Table{
			"delivery_id": job.DeliveryID.String(),
			"dlq_reason":  reason,
			"failed_at":   time.Now().UTC().Format(time.RFC3339),
		},
	}

	err = ch.PublishWithContext(
		ctx,
		broker.ExchangeDLQ,
		broker.RoutingKeyDeliveryDLQ,
		false,
		false,
		msg,
	)
	if err != nil {
		return fmt.Errorf("failed to route to DLQ %s: %w", broker.ExchangeDLQ, err)
	}

	d.logger.Warn("Delivery routed to DLQ",
		slog.String("delivery_id", job.DeliveryID.String()),
		slog.String("reason", reason),
	)

	return nil
}

