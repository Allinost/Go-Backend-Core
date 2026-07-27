// Package limiter Webhook 限流器
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 限流器接口
type Limiter interface {
	Allow(ctx context.Context, key string, maxPerMinute, maxPerHour int) (bool, error)
}

// RedisLimiter Redis 滑动窗口限流实现，按分钟和小时两个粒度分别计数
type RedisLimiter struct {
	client *redis.Client
}

// New 创建限流器，client 为 nil 时返回 NoopLimiter（不限流）
func New(client *redis.Client) Limiter {
	if client == nil {
		return &NoopLimiter{}
	}
	return &RedisLimiter{client: client}
}

// Allow 检查是否允许通过：超过分钟或小时限制时拒绝
// Redis 不可用时降级为允许通过
func (l *RedisLimiter) Allow(ctx context.Context, key string, maxPerMinute, maxPerHour int) (bool, error) {
	now := time.Now()
	minuteKey := fmt.Sprintf("webhook:ratelimit:%s:minute:%d", key, now.Unix()/60)
	hourKey := fmt.Sprintf("webhook:ratelimit:%s:hour:%d", key, now.Unix()/3600)

	pipe := l.client.Pipeline()
	minuteCount := pipe.Incr(ctx, minuteKey)
	pipe.Expire(ctx, minuteKey, 70*time.Second)
	hourCount := pipe.Incr(ctx, hourKey)
	pipe.Expire(ctx, hourKey, 3700*time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, nil
	}

	if maxPerMinute > 0 && minuteCount.Val() > int64(maxPerMinute) {
		return false, nil
	}
	if maxPerHour > 0 && hourCount.Val() > int64(maxPerHour) {
		return false, nil
	}
	return true, nil
}

// NoopLimiter 空实现，Redis 不可用时可安全降级
type NoopLimiter struct{}

func (l *NoopLimiter) Allow(_ context.Context, _ string, _, _ int) (bool, error) {
	return true, nil
}