package middleware

import (
	"net/http"
	"strconv"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/relayforge/relayforge/internal/telemetry"
)

func PrometheusMetrics() func(http.Handler) http.Handler {
	telemetry.InitMetrics()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				duration := time.Since(start).Seconds()
				status := strconv.Itoa(ww.Status())

				if telemetry.HttpRequestsTotal != nil {
					telemetry.HttpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
				}
				if telemetry.HttpRequestDuration != nil {
					telemetry.HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

