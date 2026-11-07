package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/relayforge/relayforge/internal/domain"
)

type CacheService interface {
	GetEndpoint(ctx context.Context, id uuid.UUID) (*domain.Endpoint, error)
	SetEndpoint(ctx context.Context, ep *domain.Endpoint, ttl time.Duration) error
	InvalidateEndpoint(ctx context.Context, id, appID uuid.UUID) error
	GetSubscribedEndpoints(ctx context.Context, appID uuid.UUID, eventType string) ([]*domain.Endpoint, error)
	SetSubscribedEndpoints(ctx context.Context, appID uuid.UUID, eventType string, eps []*domain.Endpoint, ttl time.Duration) error
}

type RedisCache struct {
	client *redis.Client
	logger *slog.Logger
}

func NewRedisCache(redisURL string, logger *slog.Logger) (*RedisCache, error) {
	if logger == nil {
		logger = slog.Default()
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	logger.Info("Connected to Redis successfully")
	return &RedisCache{
		client: client,
		logger: logger.With(slog.String("component", "redis_cache")),
	}, nil
}

func (c *RedisCache) Client() *redis.Client {
	return c.client
}

func (c *RedisCache) GetEndpoint(ctx context.Context, id uuid.UUID) (*domain.Endpoint, error) {
	key := fmt.Sprintf("rf:ep:%s", id.String())
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var ep domain.Endpoint
	if err := json.Unmarshal(val, &ep); err != nil {
		return nil, err
	}

	return &ep, nil
}

func (c *RedisCache) SetEndpoint(ctx context.Context, ep *domain.Endpoint, ttl time.Duration) error {
	key := fmt.Sprintf("rf:ep:%s", ep.ID.String())
	val, err := json.Marshal(ep)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, val, ttl).Err()
}

func (c *RedisCache) InvalidateEndpoint(ctx context.Context, id, appID uuid.UUID) error {
	key := fmt.Sprintf("rf:ep:%s", id.String())
	_ = c.client.Del(ctx, key).Err()

	// Invalidate subscription wildcards for app
	pattern := fmt.Sprintf("rf:subs:%s:*", appID.String())
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		_ = c.client.Del(ctx, iter.Val()).Err()
	}

	return nil
}

func (c *RedisCache) GetSubscribedEndpoints(ctx context.Context, appID uuid.UUID, eventType string) ([]*domain.Endpoint, error) {
	key := fmt.Sprintf("rf:subs:%s:%s", appID.String(), eventType)
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var eps []*domain.Endpoint
	if err := json.Unmarshal(val, &eps); err != nil {
		return nil, err
	}

	return eps, nil
}

func (c *RedisCache) SetSubscribedEndpoints(ctx context.Context, appID uuid.UUID, eventType string, eps []*domain.Endpoint, ttl time.Duration) error {
	key := fmt.Sprintf("rf:subs:%s:%s", appID.String(), eventType)
	val, err := json.Marshal(eps)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, val, ttl).Err()
}

