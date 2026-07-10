package net

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// BackoffStrategy 退避策略类型
type BackoffStrategy int

const (
	BackoffFixed                 BackoffStrategy = iota // 固定等待时间
	BackoffExponential                                  // 指数退避
	BackoffExponentialWithJitter                        // 指数退避 + 随机抖动
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts int
	Strategy    BackoffStrategy
	BaseWait    time.Duration
	MaxWait     time.Duration
}

// DefaultRetryConfig 返回默认重试配置（3 次，指数退避+抖动，基值 500ms，最大 30s）
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Strategy:    BackoffExponentialWithJitter,
		BaseWait:    500 * time.Millisecond,
		MaxWait:     30 * time.Second,
	}
}

// RetryFunc 可重试的函数签名
type RetryFunc func(ctx context.Context) error

// Retry 按指定配置执行带重试的函数，支持上下文取消
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

// calculateBackoff 根据退避策略计算第 attempt 次的等待时间
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
