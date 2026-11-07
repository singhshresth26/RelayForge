package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/relayforge/relayforge/internal/api/handler"
	"github.com/relayforge/relayforge/internal/api/middleware"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/ratelimit"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/service"
)

type RouterConfig struct {
	Logger          *slog.Logger
	AuthService     *service.AuthService
	AppService      *service.AppService
	EndpointService *service.EndpointService
	EventService    *service.EventService
	DeliveryService *service.DeliveryService
	RateLimiter     ratelimit.RateLimiter
	APIKeyRepo      repository.APIKeyRepository
	ReadyCheck      func() bool
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.StructuredLogger(cfg.Logger))
	r.Use(middleware.PrometheusMetrics())
	r.Use(chimiddleware.Recoverer)

	// Max 1MB request body size limiter
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
			next.ServeHTTP(w, r)
		})
	})

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key", "X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health probes & metrics
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "healthy"})
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ReadyCheck != nil && !cfg.ReadyCheck() {
			response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "dependency check failed", nil)
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/dashboard", handler.ServeDashboard)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	// Handlers
	authH := handler.NewAuthHandler(cfg.AuthService)
	appH := handler.NewAppHandler(cfg.AppService)
	endpointH := handler.NewEndpointHandler(cfg.EndpointService)
	eventH := handler.NewEventHandler(cfg.EventService)
	deliveryH := handler.NewDeliveryHandler(cfg.DeliveryService)

	// API v1 routes
	r.Route("/api/v1", func(v1 chi.Router) {
		// Public registration endpoint
		v1.Post("/auth/tenants", authH.RegisterTenant)

		// Protected endpoints
		v1.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAPIKey(cfg.APIKeyRepo))

			// Optional Rate Limiting Middleware
			if cfg.RateLimiter != nil {
				protected.Use(middleware.RateLimit(cfg.RateLimiter, 100, 20))
			}

			// API Keys management
			protected.Route("/api-keys", func(ak chi.Router) {
				ak.Post("/", authH.CreateAPIKey)
				ak.Get("/", authH.ListAPIKeys)
				ak.Delete("/{id}", authH.RevokeAPIKey)
			})

			// Applications
			protected.Route("/apps", func(apps chi.Router) {
				apps.Post("/", appH.CreateApplication)
				apps.Get("/", appH.ListApplications)
				apps.Get("/{id}", appH.GetApplication)
				apps.Patch("/{id}", appH.UpdateApplication)
				apps.Delete("/{id}", appH.DeleteApplication)

				// Webhook Endpoints under Application
				apps.Route("/{app_id}/endpoints", func(eps chi.Router) {
					eps.Post("/", endpointH.CreateEndpoint)
					eps.Get("/", endpointH.ListEndpoints)
					eps.Get("/{id}", endpointH.GetEndpoint)
					eps.Patch("/{id}", endpointH.UpdateEndpoint)
					eps.Delete("/{id}", endpointH.DeleteEndpoint)
					eps.Post("/{id}/rotate-secret", endpointH.RotateSecret)
				})

				// Events Ingestion & History under Application
				apps.Route("/{app_id}/events", func(events chi.Router) {
					events.Post("/", eventH.IngestEvent)
					events.Get("/", eventH.ListEvents)
					events.Get("/{id}", eventH.GetEvent)
				})

				// Deliveries & Attempts under Application
				apps.Route("/{app_id}/deliveries", func(delivs chi.Router) {
					delivs.Get("/", deliveryH.ListDeliveries)
					delivs.Get("/{id}", deliveryH.GetDelivery)
					delivs.Get("/{id}/attempts", deliveryH.ListAttempts)
					delivs.Post("/{id}/replay", deliveryH.ReplayDelivery)
				})
			})
		})
	})

	return r
}
