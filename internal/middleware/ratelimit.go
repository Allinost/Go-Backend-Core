package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit 简易内存滑动窗口限流中间件
// 限制每个 IP 在 window 时间内最多发起 maxRequests 次请求
type RateLimit struct {
	mu          sync.Mutex
	requests    map[string][]time.Time // IP → 请求时间戳列表
	maxRequests int                    // 窗口内最大请求数
	window      time.Duration          // 时间窗口
}

// NewRateLimit 创建限流器实例
func NewRateLimit(maxRequests int, window time.Duration) *RateLimit {
	rl := &RateLimit{
		requests:    make(map[string][]time.Time),
		maxRequests: maxRequests,
		window:      window,
	}
	// 定期清理过期记录，防止内存泄漏
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

func (rl *RateLimit) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[ip]

	// 移除窗口外的记录
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

// RateLimiter 返回 Gin 限流中间件处理函数
func (rl *RateLimit) RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
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
