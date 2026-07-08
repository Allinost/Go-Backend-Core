package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Limiter 限流器接口，支持内存和 Redis 两种实现
type Limiter interface {
	Allow(ip string) bool
}

// RateLimit 内存滑动窗口限流器（进程内，重启丢失）
type RateLimit struct {
	mu          sync.Mutex
	requests    map[string][]time.Time
	maxRequests int
	window      time.Duration
}

// NewRateLimit 创建内存限流器（开发/单实例场景）
func NewRateLimit(maxRequests int, window time.Duration) *RateLimit {
	rl := &RateLimit{
		requests:    make(map[string][]time.Time),
		maxRequests: maxRequests,
		window:      window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimit) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if now.Sub(t) < rl.window {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimit) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[ip]

	var valid []time.Time
	for _, t := range times {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxRequests {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

// RedisRateLimit 分布式限流器（基于 Redis 固定窗口）
type RedisRateLimit struct {
	client      redis.UniversalClient
	prefix      string
	maxRequests int
	window      time.Duration
}

// NewRedisRateLimit 创建 Redis 分布式限流器（生产多实例场景）
func NewRedisRateLimit(client redis.UniversalClient, maxRequests int, window time.Duration) *RedisRateLimit {
	return &RedisRateLimit{
		client:      client,
		prefix:      "ratelimit",
		maxRequests: maxRequests,
		window:      window,
	}
}

func (rl *RedisRateLimit) Allow(ip string) bool {
	key := rl.prefix + ":" + ip
	ctx := context.Background()

	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return true
	}

	if count == 1 {
		rl.client.Expire(ctx, key, rl.window)
	}

	return count <= int64(rl.maxRequests)
}

// RateLimiter 返回 Gin 限流中间件处理函数（适配 Limiter 接口）
func RateLimiter(l Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !l.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
