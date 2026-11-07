package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/relayforge/relayforge/internal/broker"
	"github.com/relayforge/relayforge/internal/config"
	"github.com/relayforge/relayforge/internal/repository/postgres"
	"github.com/relayforge/relayforge/internal/security"
	"github.com/relayforge/relayforge/internal/worker"
	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start RelayForge Concurrent Webhook Delivery Worker Pool",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)

		logger.Info("Starting Webhook Delivery Worker Pool",
			slog.Int("concurrency", cfg.WorkerConcurrency),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("Failed to connect to PostgreSQL", slog.String("error", err.Error()))
			return err
		}
		defer db.Close()

		rmq, err := broker.NewRabbitMQClient(cfg.RabbitMQURL, logger)
		if err != nil {
			logger.Error("Failed to connect to RabbitMQ", slog.String("error", err.Error()))
			return err
		}
		defer rmq.Close()

		if err := broker.SetupTopology(rmq.Channel()); err != nil {
			logger.Error("Failed to setup RabbitMQ topology", slog.String("error", err.Error()))
			return err
		}

		cryptor, err := security.NewSecretCryptor(cfg.MasterKey)
		if err != nil {
			return err
		}
		ssrf := security.NewSSRFValidator(cfg.AllowPrivateIPs, cfg.EnforceHTTPS)
		httpClient := worker.NewHTTPDeliveryClient(cryptor, ssrf)

		pool := worker.NewWorkerPool(httpClient, nil, nil, cfg.WorkerConcurrency, logger)

		if err := pool.StartConsumer(ctx, rmq.Channel()); err != nil {
			logger.Error("Failed to start worker pool consumer", slog.String("error", err.Error()))
			return err
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		logger.Info("Stopping worker pool...")
		pool.Stop()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}

