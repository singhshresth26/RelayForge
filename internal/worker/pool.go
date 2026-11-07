package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/relayforge/relayforge/internal/broker"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository"
)

type RetryDispatcher interface {
	ScheduleRetry(ctx context.Context, job *fanout.DeliveryJob, attempt int32) error
	RouteToDLQ(ctx context.Context, job *fanout.DeliveryJob, reason string) error
}

type WorkerPool struct {
	client       *HTTPDeliveryClient
	deliveryRepo repository.DeliveryRepository
	retryRouter  RetryDispatcher
	concurrency  int
	logger       *slog.Logger
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewWorkerPool(
	client *HTTPDeliveryClient,
	deliveryRepo repository.DeliveryRepository,
	retryRouter RetryDispatcher,
	concurrency int,
	logger *slog.Logger,
) *WorkerPool {
	if concurrency <= 0 {
		concurrency = 20
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &WorkerPool{
		client:       client,
		deliveryRepo: deliveryRepo,
		retryRouter:  retryRouter,
		concurrency:  concurrency,
		logger:       logger.With(slog.String("component", "delivery_worker_pool")),
		stopCh:       make(chan struct{}),
	}
}

// ProcessJob executes a single delivery job and records results
func (p *WorkerPool) ProcessJob(ctx context.Context, job *fanout.DeliveryJob) error {
	attemptNum := job.CurrentAttempt + 1

	// Update delivery to PROCESSING
	_, _ = p.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryProcessing, attemptNum, nil, nil)

	// Execute HTTP dispatch
	result := p.client.ExecuteDelivery(ctx, job)

	// Record delivery attempt log in database
	_, err := p.deliveryRepo.CreateDeliveryAttempt(
		ctx,
		job.DeliveryID,
		attemptNum,
		result.StatusCode,
		result.LatencyMS,
		result.RequestHeaders,
		result.ResponseHeaders,
		result.ResponseBody,
		result.ErrorMessage,
	)
	if err != nil {
		p.logger.Error("Failed to save delivery attempt",
			slog.String("delivery_id", job.DeliveryID.String()),
			slog.String("error", err.Error()),
		)
	}

	if result.IsSuccess {
		// Delivery succeeded! Terminal state DELIVERED
		now := time.Now().UTC()
		_, err = p.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryDelivered, attemptNum, nil, &now)
		p.logger.Info("Delivery successful",
			slog.String("delivery_id", job.DeliveryID.String()),
			slog.Int("attempt", int(attemptNum)),
			slog.Int("latency_ms", int(result.LatencyMS)),
		)
		return err
	}

	// Delivery failed! Check retry availability
	if attemptNum < job.MaxAttempts {
		// Retries remaining: transition to RETRYING
		_, _ = p.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryRetrying, attemptNum, nil, nil)
		p.logger.Warn("Delivery failed, scheduling retry",
			slog.String("delivery_id", job.DeliveryID.String()),
			slog.Int("attempt", int(attemptNum)),
			slog.Int("max_attempts", int(job.MaxAttempts)),
		)

		if p.retryRouter != nil {
			return p.retryRouter.ScheduleRetry(ctx, job, attemptNum)
		}
		return nil
	}

	// Retries exhausted: transition to FAILED (terminal) and route to DLQ
	_, _ = p.deliveryRepo.UpdateDeliveryStatus(ctx, job.DeliveryID, domain.DeliveryFailed, attemptNum, nil, nil)
	p.logger.Error("Delivery permanently failed, routing to DLQ",
		slog.String("delivery_id", job.DeliveryID.String()),
		slog.Int("final_attempt", int(attemptNum)),
	)

	if p.retryRouter != nil {
		reason := "Max retries exceeded"
		if result.ErrorMessage != nil {
			reason = *result.ErrorMessage
		}
		return p.retryRouter.RouteToDLQ(ctx, job, reason)
	}

	return nil
}

// StartConsumer launches concurrent workers listening on RabbitMQ
func (p *WorkerPool) StartConsumer(ctx context.Context, ch *amqp.Channel) error {
	// Set prefetch QoS equal to concurrency * 2
	if err := ch.Qos(p.concurrency*2, 0, false); err != nil {
		return fmt.Errorf("failed to set prefetch QoS: %w", err)
	}

	deliveries, err := ch.Consume(
		broker.QueueDeliveriesProcess,
		"delivery-worker",
		false, // manual ACK
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from %s: %w", broker.QueueDeliveriesProcess, err)
	}

	p.logger.Info("Delivery Worker Pool started", slog.Int("concurrency", p.concurrency))

	// Launch worker goroutines
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-p.stopCh:
					return
				case d, ok := <-deliveries:
					if !ok {
						return
					}

					var job fanout.DeliveryJob
					if err := json.Unmarshal(d.Body, &job); err != nil {
						p.logger.Error("Failed to unmarshal delivery job", slog.String("error", err.Error()))
						_ = d.Nack(false, false) // discard corrupt message
						continue
					}

					if err := p.ProcessJob(ctx, &job); err != nil {
						p.logger.Error("Job processing error", slog.String("error", err.Error()))
					}

					_ = d.Ack(false)
				}
			}
		}(i)
	}

	return nil
}

func (p *WorkerPool) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	p.logger.Info("Delivery Worker Pool stopped cleanly")
}

