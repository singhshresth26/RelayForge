package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/relayforge/relayforge/internal/api/response"
	"github.com/relayforge/relayforge/internal/repository"
	"github.com/relayforge/relayforge/internal/security"
)

type contextKey string

const (
	TenantIDKey contextKey = "tenant_id"
	APIKeyIDKey contextKey = "api_key_id"
	ScopesKey   contextKey = "scopes"
)

func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(TenantIDKey).(uuid.UUID)
	return val, ok
}

func ScopesFromContext(ctx context.Context) ([]string, bool) {
	val, ok := ctx.Value(ScopesKey).([]string)
	return val, ok
}

// RequireAPIKey validates the Authorization: Bearer <key> header
func RequireAPIKey(apiKeyRepo repository.APIKeyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header", nil)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Error(w, http.StatusUnauthorized, "INVALID_AUTH_HEADER", "Authorization header must be Bearer <token>", nil)
				return
			}

			rawToken := strings.TrimSpace(parts[1])
			if _, err := security.ExtractPrefix(rawToken); err != nil {
				response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY_FORMAT", "API key has invalid format", nil)
				return
			}

			keyHash := security.HashAPIKey(rawToken)
			apiKey, err := apiKeyRepo.GetAPIKeyByHash(r.Context(), keyHash)
			if err != nil || apiKey == nil {
				response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY", "API key is invalid or revoked", nil)
				return
			}

			// Fire-and-forget update last used timestamp
			go func(id uuid.UUID) {
				_ = apiKeyRepo.UpdateAPIKeyLastUsed(context.Background(), id)
			}(apiKey.ID)

			ctx := context.WithValue(r.Context(), TenantIDKey, apiKey.TenantID)
			ctx = context.WithValue(ctx, APIKeyIDKey, apiKey.ID)
			ctx = context.WithValue(ctx, ScopesKey, apiKey.Scopes)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope verifies that the authenticated API key contains the required scope
func RequireScope(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scopes, ok := ScopesFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusForbidden, "FORBIDDEN", "No scopes found in context", nil)
				return
			}

			for _, s := range scopes {
				if s == requiredScope || s == "*" || s == "admin" {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.Error(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions for this operation", map[string]any{
				"required_scope": requiredScope,
			})
		})
	}
}
