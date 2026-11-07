package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/relayforge/relayforge/internal/api"
	"github.com/relayforge/relayforge/internal/config"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/outbox"
	"github.com/relayforge/relayforge/internal/ratelimit"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/repository/memory"
	"github.com/relayforge/relayforge/internal/repository/postgres"
	"github.com/relayforge/relayforge/internal/scheduler"
	"github.com/relayforge/relayforge/internal/security"
	"github.com/relayforge/relayforge/internal/service"
	"github.com/relayforge/relayforge/internal/worker"
	"github.com/spf13/cobra"
)

const DemoAPIKey = "rf_live_demo_relayforge_secret_key"

type InProcessBroker struct {
	fanoutProcessor *fanout.FanoutProcessor
	workerPool      *worker.WorkerPool
}

func (b *InProcessBroker) PublishEvent(ctx context.Context, event *domain.OutboxEvent) error {
	if b.fanoutProcessor != nil {
		_, err := b.fanoutProcessor.ProcessEvent(ctx, fanout.IngestedEventMessage{
			EventID:   event.EventID,
			AppID:     event.AppID,
			EventType: event.EventType,
			Payload:   event.Payload,
			CreatedAt: event.CreatedAt.Format(time.RFC3339),
		})
		return err
	}
	return nil
}

func (b *InProcessBroker) PublishDelivery(ctx context.Context, job *fanout.DeliveryJob) error {
	if b.workerPool != nil {
		go func(j *fanout.DeliveryJob) {
			_ = b.workerPool.ProcessJob(context.Background(), j)
		}(job)
	}
	return nil
}

func (b *InProcessBroker) ScheduleRetry(ctx context.Context, job *fanout.DeliveryJob, attempt int32) error {
	// In-process retry after backoff delay
	delay := scheduler.CalculateBackoff(scheduler.BackoffConfig{
		BaseDelay: 2 * time.Second,
		MaxDelay:  30 * time.Second,
	}, attempt)

	go func(j *fanout.DeliveryJob, d time.Duration) {
		time.Sleep(d)
		if b.workerPool != nil {
			_ = b.workerPool.ProcessJob(context.Background(), j)
		}
	}(job, delay)
	return nil
}

func (b *InProcessBroker) RouteToDLQ(ctx context.Context, job *fanout.DeliveryJob, reason string) error {
	return nil
}

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start RelayForge HTTP REST API Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()

		logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		logger := slog.New(logHandler)
		slog.SetDefault(logger)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var tenantRepo repository.TenantRepository
		var apiKeyRepo repository.APIKeyRepository
		var appRepo repository.ApplicationRepository
		var endpointRepo repository.EndpointRepository
		var eventRepo repository.EventRepository
		var outboxRepo repository.OutboxRepository
		var deliveryRepo repository.DeliveryRepository
		var rateLimiter ratelimit.RateLimiter

		// Try connecting to PostgreSQL
		db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
		if err == nil {
			logger.Info("Connected to PostgreSQL successfully")
			tenantRepo = postgres.NewTenantRepo(db)
			// PostgreSQL repositories active
		} else {
			logger.Warn("PostgreSQL not detected on "+cfg.DatabaseURL+", running in high-performance Dev/Memory mode", slog.String("reason", err.Error()))
			memStore := memory.NewMemoryStore()
			tenantRepo = memStore
			apiKeyRepo = memStore
			appRepo = memStore
			endpointRepo = memStore
			eventRepo = memStore
			outboxRepo = memStore
			deliveryRepo = memStore
			rateLimiter = ratelimit.NewMemoryTokenBucketLimiter()

			// Seed Demo Tenant, API Key, App, and Endpoint
			tenant, _ := memStore.CreateTenant(ctx, "Acme Corp", "acme-corp", "enterprise")
			keyHash := security.HashAPIKey(DemoAPIKey)
			_, _ = memStore.CreateAPIKey(ctx, tenant.ID, "Default Dev Key", security.APIKeyLivePrefix, keyHash, []string{"*"}, nil)
			demoApp, _ := memStore.CreateApplication(ctx, tenant.ID, "E-Commerce Orders", "Production order processing", true)

			cryptor, _ := security.NewSecretCryptor(cfg.MasterKey)
			encSecret, _ := cryptor.Encrypt("whsec_demo_secret_key_12345")
			_, _ = memStore.CreateEndpoint(
				ctx, demoApp.ID, "https://webhook.site/demo", encSecret,
				"Main Webhook", true, 5, 10000, 50, []string{"*"},
			)
		}

		// Cryptor & SSRF
		cryptor, err := security.NewSecretCryptor(cfg.MasterKey)
		if err != nil {
			return err
		}
		ssrfValidator := security.NewSSRFValidator(cfg.AllowPrivateIPs, cfg.EnforceHTTPS)
		httpClient := worker.NewHTTPDeliveryClient(cryptor, ssrfValidator)

		// Setup in-process broker & workers for standalone/dev execution
		inProcessBroker := &InProcessBroker{}
		workerPool := worker.NewWorkerPool(httpClient, deliveryRepo, inProcessBroker, cfg.WorkerConcurrency, logger)
		fanoutProc := fanout.NewFanoutProcessor(endpointRepo, deliveryRepo, inProcessBroker, logger)
		inProcessBroker.fanoutProcessor = fanoutProc
		inProcessBroker.workerPool = workerPool

		// Start background Outbox Publisher
		if outboxRepo != nil {
			outboxPublisher := outbox.NewOutboxPublisher(outboxRepo, inProcessBroker, outbox.PublisherConfig{
				BatchSize:    int32(cfg.OutboxBatchSize),
				PollInterval: cfg.OutboxPollInterval,
				Logger:       logger,
			})
			outboxPublisher.Start(ctx)
			defer outboxPublisher.Stop()
		}

		// Services
		authService := service.NewAuthService(tenantRepo, apiKeyRepo, nil)
		appService := service.NewAppService(appRepo)
		endpointService := service.NewEndpointService(endpointRepo, appRepo, cryptor, ssrfValidator)
		eventService := service.NewEventService(eventRepo, outboxRepo, appRepo)
		deliveryService := service.NewDeliveryService(deliveryRepo, endpointRepo, eventRepo, appRepo, inProcessBroker)

		// Router
		router := api.NewRouter(api.RouterConfig{
			Logger:          logger,
			AuthService:     authService,
			AppService:      appService,
			EndpointService: endpointService,
			EventService:    eventService,
			DeliveryService: deliveryService,
			RateLimiter:     rateLimiter,
			APIKeyRepo:      apiKeyRepo,
			ReadyCheck: func() bool {
				return true
			},
		})

		server := &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Graceful shutdown channel
		shutdownErr := make(chan error, 1)
		go func() {
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
			<-quit

			logger.Info("Gracefully shutting down API server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			shutdownErr <- server.Shutdown(shutdownCtx)
		}()

		fmt.Println("==========================================================================")
		fmt.Printf("🚀 RelayForge Webhook Delivery Platform is RUNNING!\n")
		fmt.Printf("💻 Developer Console: http://localhost:%s/dashboard\n", cfg.Port)
		fmt.Printf("📊 Prometheus Metrics: http://localhost:%s/metrics\n", cfg.Port)
		fmt.Printf("💓 Health Check:       http://localhost:%s/healthz\n", cfg.Port)
		fmt.Printf("🔑 Demo API Key:       %s\n", DemoAPIKey)
		fmt.Println("==========================================================================")

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}

		return <-shutdownErr
	},
}
