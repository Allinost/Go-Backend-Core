package integration

import (
	"sync"

	"github.com/Allinost/go-backend-core/internal/services/outbox"
)

// 运行时可热更新的组件引用，由 Init 注册，配置中心保存后通过 Set* 生效。
var (
	runtimeMu       sync.Mutex
	activeRateLimit interface{ SetMaxRequests(int) }
	activeRetention *outbox.Retention
)

// SetRateLimitPerMinute 热更新集成动作/MCP 每 Key 每分钟限流。限流未启用时仅记录值。
func SetRateLimitPerMinute(n int) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if activeRateLimit != nil {
		activeRateLimit.SetMaxRequests(n)
	}
}

// SetRetentionDays 热更新已投递事件保留天数，0 表示停止清理。
func SetRetentionDays(n int) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if activeRetention != nil {
		activeRetention.SetDays(n)
	}
}
