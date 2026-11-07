package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/relayforge/relayforge/internal/config"
	"github.com/relayforge/relayforge/internal/outbox"
	"github.com/relayforge/relayforge/internal/repository/postgres"
	"github.com/spf13/cobra"
)

var outboxCmd = &cobra.Command{
	Use:   "outbox",
	Short: "Start RelayForge Transactional Outbox Publisher Daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)

		logger.Info("Starting Transactional Outbox Publisher",
			slog.Int("batch_size", cfg.OutboxBatchSize),
			slog.Duration("poll_interval", cfg.OutboxPollInterval),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("Failed to connect to PostgreSQL", slog.String("error", err.Error()))
			return err
		}
		defer db.Close()

		// Outbox Repo
		// When running with Postgres, the outbox repo can query directly
		// In Phase 3, RabbitMQ message broker publisher will be injected here
		publisher := outbox.NewOutboxPublisher(nil, nil, outbox.PublisherConfig{
			BatchSize:    int32(cfg.OutboxBatchSize),
			PollInterval: cfg.OutboxPollInterval,
			Logger:       logger,
		})

		publisher.Start(ctx)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		logger.Info("Stopping outbox publisher...")
		publisher.Stop()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(outboxCmd)
}

