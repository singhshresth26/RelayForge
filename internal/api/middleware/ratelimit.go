package middleware

import (
	"net/http"
	"strconv"

	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/ratelimit"
)

func RateLimit(limiter ratelimit.RateLimiter, capacity, rate int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Determine rate limit bucket key
			key := "ratelimit:ip:" + r.RemoteAddr
			if tenantID, ok := TenantIDFromContext(r.Context()); ok {
				key = "ratelimit:tenant:" + tenantID.String()
			}

			result, err := limiter.Allow(r.Context(), key, capacity, rate, 1)
			if err != nil {
				// Fail open on rate limiter technical failure to not break service
				next.ServeHTTP(w, r)
				return
			}

			// Set standard rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

			if !result.Allowed {
				retryAfterSec := int(result.RetryAfter.Seconds())
				if retryAfterSec < 1 {
					retryAfterSec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
				response.Error(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit quota exceeded. Slow down.", map[string]any{
					"retry_after_seconds": retryAfterSec,
					"limit":               result.Limit,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
