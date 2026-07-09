package net

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

type BackoffStrategy int

const (
	BackoffFixed BackoffStrategy = iota
	BackoffExponential
	BackoffExponentialWithJitter
)

type RetryConfig struct {
	MaxAttempts int
	Strategy    BackoffStrategy
	BaseWait    time.Duration
	MaxWait     time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Strategy:    BackoffExponentialWithJitter,
		BaseWait:    500 * time.Millisecond,
		MaxWait:     30 * time.Second,
	}
}

type RetryFunc func(ctx context.Context) error

func Retry(ctx context.Context, config RetryConfig, fn RetryFunc) error {
	var lastErr error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			wait := calculateBackoff(config, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		if err := fn(ctx); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("net: 重试 %d 次后失败: %w", config.MaxAttempts, lastErr)
}

func calculateBackoff(config RetryConfig, attempt int) time.Duration {
	switch config.Strategy {
	case BackoffFixed:
		return config.BaseWait
	case BackoffExponential:
		wait := config.BaseWait * (1 << uint(attempt-1))
		if wait > config.MaxWait {
			wait = config.MaxWait
		}
		return wait
	case BackoffExponentialWithJitter:
		wait := config.BaseWait * (1 << uint(attempt-1))
		if wait > config.MaxWait {
			wait = config.MaxWait
		}
		jitter := time.Duration(rand.Float64() * float64(wait) * 0.5)
		return wait + jitter
	default:
		return config.BaseWait
	}
}
