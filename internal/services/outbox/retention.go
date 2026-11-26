package outbox

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

// RetentionConfig 已投递事件保留配置。
type RetentionConfig struct {
	Days     int
	Interval time.Duration
}

// Retention 定期清理超过保留期的已投递事件。
type Retention struct {
	svc      *Service
	days     atomic.Int64
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRetention 创建清理器。days <= 0 表示禁用。
func NewRetention(svc *Service, cfg RetentionConfig) *Retention {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	r := &Retention{svc: svc, interval: interval}
	r.days.Store(int64(cfg.Days))
	return r
}

// SetDays 运行时调整保留天数（配置中心热更新）；<= 0 表示停止清理。
func (r *Retention) SetDays(days int) {
	if r == nil {
		return
	}
	r.days.Store(int64(days))
}

// Start 启动后台清理循环（循环常驻，保留天数为 0 时每轮跳过，便于运行时开启）。
func (r *Retention) Start(parent context.Context) {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	r.cancel = cancel
	r.done = done
	r.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if deleted, err := r.RunOnce(ctx); err != nil {
					if ctx.Err() == nil {
						logger.Warn().Err(err).Msg("outbox: 清理历史事件失败")
					}
				} else if deleted > 0 {
					logger.Info().Int64("deleted", deleted).Msg("outbox: 已清理历史事件")
				}
			}
		}
	}()
}

// Stop 停止后台清理循环。
func (r *Retention) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

// RunOnce 执行一次清理，返回删除条数。
func (r *Retention) RunOnce(ctx context.Context) (int64, error) {
	if r == nil || r.svc == nil {
		return 0, nil
	}
	days := int(r.days.Load())
	if days <= 0 {
		return 0, nil
	}
	before := time.Now().UTC().AddDate(0, 0, -days)
	return r.svc.DeleteDeliveredBefore(ctx, before)
}
