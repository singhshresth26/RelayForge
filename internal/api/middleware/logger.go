package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func StructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				latency := time.Since(start)
				status := ww.Status()
				bytes := ww.BytesWritten()

				tenantID, _ := TenantIDFromContext(r.Context())

				fields := []any{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", bytes),
					slog.Duration("latency", latency),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("user_agent", r.UserAgent()),
				}

				if tenantID.String() != "00000000-0000-0000-0000-000000000000" {
					fields = append(fields, slog.String("tenant_id", tenantID.String()))
				}

				if status >= 500 {
					logger.Error("HTTP request server error", fields...)
				} else if status >= 400 {
					logger.Warn("HTTP request client error", fields...)
				} else {
					logger.Info("HTTP request processed", fields...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

