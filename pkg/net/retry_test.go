package net

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), DefaultRetryConfig(), func(ctx context.Context) error {
		attempts++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), DefaultRetryConfig(), func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("attempt %d failed", attempts)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_AllFail(t *testing.T) {
	attempts := 0
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 3
	err := Retry(context.Background(), cfg, func(ctx context.Context) error {
		attempts++
		return errors.New("always fail")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "重试 3 次后失败")
	assert.Equal(t, 3, attempts)
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, DefaultRetryConfig(), func(ctx context.Context) error {
		return errors.New("should not reach here")
	})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetry_BackoffDuration(t *testing.T) {
	start := time.Now()
	cfg := RetryConfig{
		MaxAttempts: 3,
		Strategy:    BackoffExponential,
		BaseWait:    10 * time.Millisecond,
		MaxWait:     1 * time.Second,
	}

	attempts := 0
	Retry(context.Background(), cfg, func(ctx context.Context) error {
		attempts++
		return errors.New("fail")
	})
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)
	assert.Equal(t, 3, attempts)
}

func TestBackoff_Fixed(t *testing.T) {
	d := calculateBackoff(RetryConfig{
		Strategy: BackoffFixed,
		BaseWait: 100 * time.Millisecond,
	}, 5)
	assert.Equal(t, 100*time.Millisecond, d)
}

func TestBackoff_Exponential(t *testing.T) {
	d1 := calculateBackoff(RetryConfig{
		Strategy: BackoffExponential,
		BaseWait: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}, 1)
	assert.Equal(t, 100*time.Millisecond, d1)

	d2 := calculateBackoff(RetryConfig{
		Strategy: BackoffExponential,
		BaseWait: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}, 3)
	assert.Equal(t, 400*time.Millisecond, d2)
}

func TestBackoff_ExponentialWithJitter(t *testing.T) {
	cfg := RetryConfig{
		Strategy: BackoffExponentialWithJitter,
		BaseWait: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	d1 := calculateBackoff(cfg, 1)
	d2 := calculateBackoff(cfg, 1)
	assert.GreaterOrEqual(t, d1, 100*time.Millisecond)
	assert.GreaterOrEqual(t, d2, 100*time.Millisecond)
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, BackoffExponentialWithJitter, cfg.Strategy)
	assert.Equal(t, 500*time.Millisecond, cfg.BaseWait)
	assert.Equal(t, 30*time.Second, cfg.MaxWait)
}
