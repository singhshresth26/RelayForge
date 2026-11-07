package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/relayforge/relayforge/internal/ratelimit"
)

func TestMemoryTokenBucketLimiter(t *testing.T) {
	ctx := context.Background()
	limiter := ratelimit.NewMemoryTokenBucketLimiter()

	key := "test-tenant-1"
	capacity := 5
	rate := 2 // 2 tokens per second

	// 1. Initial burst: consume all 5 tokens
	for i := 0; i < 5; i++ {
		res, err := limiter.Allow(ctx, key, capacity, rate, 1)
		if err != nil {
			t.Fatalf("unexpected error on token %d: %v", i, err)
		}
		if !res.Allowed {
			t.Errorf("expected token %d to be allowed", i)
		}
	}

	// 2. 6th request should be rejected (bucket empty)
	res, err := limiter.Allow(ctx, key, capacity, rate, 1)
	if err != nil {
		t.Fatalf("unexpected error on 6th token: %v", err)
	}
	if res.Allowed {
		t.Errorf("expected 6th request to be rejected")
	}
	if res.RetryAfter <= 0 {
		t.Errorf("expected retry after > 0, got %v", res.RetryAfter)
	}

	// 3. Sleep 600ms (refills ~1 token at 2 tokens/sec)
	time.Sleep(600 * time.Millisecond)

	resAfter, err := limiter.Allow(ctx, key, capacity, rate, 1)
	if err != nil {
		t.Fatalf("unexpected error after refill: %v", err)
	}
	if !resAfter.Allowed {
		t.Errorf("expected request to be allowed after token refill")
	}
}

