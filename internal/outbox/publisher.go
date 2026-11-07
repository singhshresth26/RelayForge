package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
)

// MessagePublisher abstracts the message broker (RabbitMQ) publish mechanism
type MessagePublisher interface {
	PublishEvent(ctx context.Context, event *domain.OutboxEvent) error
}

type PublisherConfig struct {
	BatchSize    int32
	PollInterval time.Duration
	WorkerID     string
	Logger       *slog.Logger
}

type OutboxPublisher struct {
	outboxRepo repository.OutboxRepository
	broker     MessagePublisher
	cfg        PublisherConfig
	logger     *slog.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewOutboxPublisher(
	outboxRepo repository.OutboxRepository,
	broker MessagePublisher,
	cfg PublisherConfig,
) *OutboxPublisher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 50 * time.Millisecond
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &OutboxPublisher{
		outboxRepo: outboxRepo,
		broker:     broker,
		cfg:        cfg,
		logger:     cfg.Logger.With(slog.String("component", "outbox_publisher")),
		stopCh:     make(chan struct{}),
	}
}

// Start begins background polling loop
func (p *OutboxPublisher) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.logger.Info("Outbox publisher started",
			slog.Int("batch_size", int(p.cfg.BatchSize)),
			slog.Duration("poll_interval", p.cfg.PollInterval),
			slog.String("worker_id", p.cfg.WorkerID),
		)

		ticker := time.NewTicker(p.cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Info("Context cancelled, stopping outbox publisher")
				return
			case <-p.stopCh:
				p.logger.Info("Stop signal received, stopping outbox publisher")
				return
			case <-ticker.C:
				count, err := p.ProcessBatch(ctx)
				if err != nil {
					p.logger.Error("Failed to process outbox batch", slog.String("error", err.Error()))
				} else if count > 0 {
					p.logger.Debug("Processed outbox batch", slog.Int("count", count))
				}
			}
		}
	}()
}

// ProcessBatch claims and publishes a single batch using FOR UPDATE SKIP LOCKED
func (p *OutboxPublisher) ProcessBatch(ctx context.Context) (int, error) {
	events, err := p.outboxRepo.FetchPendingOutboxBatch(ctx, p.cfg.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending outbox batch: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	var publishedIDs []int64

	for _, event := range events {
		if p.broker != nil {
			err := p.broker.PublishEvent(ctx, event)
			if err != nil {
				p.logger.Error("Failed to publish outbox event to broker",
					slog.Int64("outbox_id", event.ID),
					slog.String("event_id", event.EventID.String()),
					slog.String("error", err.Error()),
				)
				_ = p.outboxRepo.MarkOutboxFailed(ctx, event.ID, err.Error())
				continue
			}
		}

		publishedIDs = append(publishedIDs, event.ID)
	}

	if len(publishedIDs) > 0 {
		if err := p.outboxRepo.MarkOutboxPublished(ctx, publishedIDs); err != nil {
			return len(publishedIDs), fmt.Errorf("failed to mark outbox events published: %w", err)
		}
	}

	return len(publishedIDs), nil
}

// Stop gracefully shuts down the publisher
func (p *OutboxPublisher) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	p.logger.Info("Outbox publisher stopped cleanly")
}

