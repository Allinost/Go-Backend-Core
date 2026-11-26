package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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

// SetMaxRequests 运行时调整窗口内最大请求数（配置中心热更新）
func (rl *RateLimit) SetMaxRequests(n int) {
	if n <= 0 {
		return
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maxRequests = n
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
	maxRequests atomic.Int64
	window      time.Duration
}

// NewRedisRateLimit 创建 Redis 分布式限流器（生产多实例场景）
func NewRedisRateLimit(client redis.UniversalClient, maxRequests int, window time.Duration) *RedisRateLimit {
	rl := &RedisRateLimit{
		client: client,
		prefix: "ratelimit",
		window: window,
	}
	rl.maxRequests.Store(int64(maxRequests))
	return rl
}

// SetMaxRequests 运行时调整窗口内最大请求数（配置中心热更新）
func (rl *RedisRateLimit) SetMaxRequests(n int) {
	if n > 0 {
		rl.maxRequests.Store(int64(n))
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

	return count <= rl.maxRequests.Load()
}

// RateLimiter 返回 Gin 限流中间件处理函数（按客户端 IP）
func RateLimiter(l Limiter) gin.HandlerFunc {
	return RateLimiterByKey(l, func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// RateLimiterByKey 按自定义 key 限流，用于 API Key / 用户级别的配额。
func RateLimiterByKey(l Limiter, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if keyFunc != nil {
			if custom := strings.TrimSpace(keyFunc(c)); custom != "" {
				key = custom
			}
		}
		if !l.Allow(key) {
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
