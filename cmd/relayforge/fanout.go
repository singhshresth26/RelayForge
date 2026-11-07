package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/relayforge/relayforge/internal/broker"
	"github.com/relayforge/relayforge/internal/config"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/repository/postgres"
	"github.com/spf13/cobra"
)

var fanoutCmd = &cobra.Command{
	Use:   "fanout",
	Short: "Start RelayForge Event Fan-out Processor Daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)

		logger.Info("Starting Event Fan-out Processor Daemon")

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

		deliveryPub := fanout.NewAMQPDeliveryPublisher(rmq)
		processor := fanout.NewFanoutProcessor(nil, nil, deliveryPub, logger)

		if err := processor.StartConsumer(ctx, rmq.Channel()); err != nil {
			logger.Error("Failed to start consumer", slog.String("error", err.Error()))
			return err
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		logger.Info("Stopping fan-out processor...")
		processor.Stop()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fanoutCmd)
}

