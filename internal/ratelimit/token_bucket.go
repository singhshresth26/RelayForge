package ratelimit

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Limit      int
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, capacity, rate, cost int) (*RateLimitResult, error)
}

// Token Bucket atomic Lua script
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'last_updated')
local tokens = tonumber(data[1])
local last_updated = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_updated = now
else
    local elapsed = math.max(0, now - last_updated)
    tokens = math.min(capacity, tokens + (elapsed * rate))
    last_updated = now
end

if tokens >= cost then
    tokens = tokens - cost
    redis.call('HMSET', key, 'tokens', tokens, 'last_updated', last_updated)
    local ttl = math.ceil(capacity / rate) * 2
    redis.call('EXPIRE', key, math.max(60, ttl))
    return {1, math.floor(tokens), 0}
else
    local needed = cost - tokens
    local retry_after = math.ceil(needed / rate)
    return {0, math.floor(tokens), retry_after}
end
`

// RedisTokenBucketLimiter executes atomic token bucket checks in Redis
type RedisTokenBucketLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewRedisTokenBucketLimiter(client *redis.Client) *RedisTokenBucketLimiter {
	return &RedisTokenBucketLimiter{
		client: client,
		script: redis.NewScript(tokenBucketScript),
	}
}

func (l *RedisTokenBucketLimiter) Allow(ctx context.Context, key string, capacity, rate, cost int) (*RateLimitResult, error) {
	now := time.Now().Unix()
	res, err := l.script.Run(ctx, l.client, []string{key}, capacity, rate, now, cost).Result()
	if err != nil {
		return nil, fmt.Errorf("redis rate limiter script failed: %w", err)
	}

	values, ok := res.([]any)
	if !ok || len(values) < 3 {
		return nil, fmt.Errorf("unexpected script output: %v", res)
	}

	allowed := values[0].(int64) == 1
	remaining := int(values[1].(int64))
	retryAfterSec := values[2].(int64)

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryAfterSec) * time.Second,
		Limit:      capacity,
	}, nil
}

// MemoryTokenBucketLimiter provides local thread-safe rate limiting without Redis
type MemoryTokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens      float64
	lastUpdated time.Time
}

func NewMemoryTokenBucketLimiter() *MemoryTokenBucketLimiter {
	return &MemoryTokenBucketLimiter{
		buckets: make(map[string]*bucket),
	}
}

func (m *MemoryTokenBucketLimiter) Allow(ctx context.Context, key string, capacity, rate, cost int) (*RateLimitResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	b, exists := m.buckets[key]
	if !exists {
		b = &bucket{
			tokens:      float64(capacity),
			lastUpdated: now,
		}
		m.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastUpdated).Seconds()
		b.tokens = math.Min(float64(capacity), b.tokens+(elapsed*float64(rate)))
		b.lastUpdated = now
	}

	if b.tokens >= float64(cost) {
		b.tokens -= float64(cost)
		return &RateLimitResult{
			Allowed:    true,
			Remaining:  int(b.tokens),
			RetryAfter: 0,
			Limit:      capacity,
		}, nil
	}

	needed := float64(cost) - b.tokens
	retryAfter := time.Duration(math.Ceil(needed/float64(rate))) * time.Second
	return &RateLimitResult{
		Allowed:    false,
		Remaining:  int(b.tokens),
		RetryAfter: retryAfter,
		Limit:      capacity,
	}, nil
}

