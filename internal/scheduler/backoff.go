package scheduler

import (
	"math"
	"math/rand"
	"time"
)

type BackoffConfig struct {
	BaseDelay time.Duration // e.g. 5 seconds
	MaxDelay  time.Duration // e.g. 1 hour
}

func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		BaseDelay: 5 * time.Second,
		MaxDelay:  1 * time.Hour,
	}
}

// CalculateBackoff computes exponential backoff with full jitter:
// delay = min(MaxDelay, BaseDelay * 2^attempt) + rand(0, BaseDelay)
func CalculateBackoff(cfg BackoffConfig, attempt int32) time.Duration {
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 5 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 1 * time.Hour
	}

	if attempt < 0 {
		attempt = 0
	}

	// Exponential factor: 2^attempt
	multiplier := math.Pow(2, float64(attempt))
	rawDelay := float64(cfg.BaseDelay) * multiplier

	if rawDelay > float64(cfg.MaxDelay) {
		rawDelay = float64(cfg.MaxDelay)
	}

	// Add full randomized jitter: between 0 and BaseDelay
	jitter := float64(rand.Int63n(int64(cfg.BaseDelay)))
	totalDelay := time.Duration(rawDelay + jitter)

	if totalDelay > cfg.MaxDelay {
		totalDelay = cfg.MaxDelay
	}

	return totalDelay
}

