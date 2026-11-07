package telemetry

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once sync.Once

	HttpRequestsTotal *prometheus.CounterVec
	HttpRequestDuration *prometheus.HistogramVec
	EventsIngestedTotal *prometheus.CounterVec
	DeliveriesTotal     *prometheus.CounterVec
	DeliveryDuration    *prometheus.HistogramVec
	OutboxPendingGauge  prometheus.Gauge
	DLQQueueDepthGauge  prometheus.Gauge
)

func InitMetrics() {
	once.Do(func() {
		HttpRequestsTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "relayforge",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed by RelayForge",
			},
			[]string{"method", "path", "status"},
		)

		HttpRequestDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "relayforge",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request latency in seconds",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"method", "path"},
		)

		EventsIngestedTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "relayforge",
				Subsystem: "ingestion",
				Name:      "events_ingested_total",
				Help:      "Total number of events ingested via API",
			},
			[]string{"app_id", "event_type", "status"},
		)

		DeliveriesTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "relayforge",
				Subsystem: "delivery",
				Name:      "attempts_total",
				Help:      "Total number of delivery attempts made to webhook destinations",
			},
			[]string{"endpoint_id", "status"},
		)

		DeliveryDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "relayforge",
				Subsystem: "delivery",
				Name:      "duration_seconds",
				Help:      "Outbound webhook delivery latency in seconds",
				Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 15},
			},
			[]string{"endpoint_id"},
		)

		OutboxPendingGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "relayforge",
				Subsystem: "outbox",
				Name:      "pending_records",
				Help:      "Current count of pending outbox records awaiting publishing",
			},
		)

		DLQQueueDepthGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "relayforge",
				Subsystem: "dlq",
				Name:      "queue_depth",
				Help:      "Total dead-lettered delivery jobs awaiting inspection or replay",
			},
		)
	})
}

