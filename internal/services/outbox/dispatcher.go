package outbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/connector"
)

// DispatcherConfig 外盒事件分发器配置。
type DispatcherConfig struct {
	Enabled     bool
	Interval    time.Duration
	BatchSize   int
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	SendTimeout time.Duration
}

func (c DispatcherConfig) normalized() DispatcherConfig {
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.BatchSize > 500 {
		c.BatchSize = 500
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 30 * time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Minute
	}
	if c.MaxBackoff < c.BaseBackoff {
		c.MaxBackoff = c.BaseBackoff
	}
	if c.SendTimeout <= 0 {
		c.SendTimeout = 10 * time.Second
	}
	return c
}

// Dispatcher 轮询待投递事件并推送到匹配的连接器。
type Dispatcher struct {
	svc        *Service
	connectors *connector.Registry
	cfg        DispatcherConfig

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewDispatcher 创建分发器。
func NewDispatcher(svc *Service, connectors *connector.Registry, cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{svc: svc, connectors: connectors, cfg: cfg.normalized()}
}

// Start 启动后台轮询，重复调用不会创建多个循环。
func (d *Dispatcher) Start(parent context.Context) {
	if d == nil || !d.cfg.Enabled || d.svc == nil || d.connectors == nil {
		return
	}
	d.mu.Lock()
	if d.cancel != nil {
		d.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	d.cancel = cancel
	d.done = done
	d.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(d.cfg.Interval)
		defer ticker.Stop()
		d.runOnceSafe(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.runOnceSafe(ctx)
			}
		}
	}()
}

// Stop 停止后台轮询并等待当前轮次结束。
func (d *Dispatcher) Stop() {
	if d == nil {
		return
	}
	d.mu.Lock()
	cancel := d.cancel
	done := d.done
	d.cancel = nil
	d.done = nil
	d.mu.Unlock()
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

// RunOnce 投递一轮事件，返回成功与失败事件数。
func (d *Dispatcher) RunOnce(ctx context.Context) (int, int, error) {
	if d == nil || d.svc == nil || d.connectors == nil {
		return 0, 0, fmt.Errorf("outbox: dispatcher 未初始化")
	}
	now := time.Now().UTC()
	events, err := d.collect(ctx, now)
	if err != nil {
		return 0, 0, err
	}

	delivered, failed := 0, 0
	for _, event := range events {
		targets := d.targets(event.Topic)
		if len(targets) == 0 {
			if err := d.svc.MarkDelivered(ctx, event.ID); err != nil {
				failed++
				logger.Warn().Err(err).Str("event_id", event.ID).Msg("outbox: 无订阅连接器，标记已投递失败")
				continue
			}
			delivered++
			continue
		}

		var errs []string
		for _, target := range targets {
			sendCtx, cancel := context.WithTimeout(ctx, d.cfg.SendTimeout)
			sendErr := target.Send(sendCtx, connector.Message{
				EventID:    event.ID,
				Event:      event.Topic,
				OccurredAt: event.CreatedAt,
				Payload:    event.Payload,
				Metadata: map[string]string{
					"source":         event.Source,
					"aggregate_type": event.AggregateType,
					"aggregate_id":   event.AggregateID,
				},
			})
			cancel()
			if sendErr != nil {
				errs = append(errs, target.ID()+": "+sendErr.Error())
			}
		}

		if len(errs) > 0 {
			failed++
			reason := truncateReason(strings.Join(errs, "; "))
			if err := d.svc.MarkFailed(ctx, event.ID, reason); err != nil {
				logger.Warn().Err(err).Str("event_id", event.ID).Msg("outbox: 标记投递失败异常")
			}
			continue
		}
		delivered++
		if err := d.svc.MarkDelivered(ctx, event.ID); err != nil {
			failed++
			logger.Warn().Err(err).Str("event_id", event.ID).Msg("outbox: 标记已投递失败")
		}
	}
	return delivered, failed, nil
}

func (d *Dispatcher) collect(ctx context.Context, now time.Time) ([]Event, error) {
	pending, err := d.svc.List(ctx, Filter{Status: string(StatusPending), Limit: d.cfg.BatchSize})
	if err != nil {
		return nil, err
	}
	failed, err := d.svc.List(ctx, Filter{Status: string(StatusFailed), Limit: d.cfg.BatchSize})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(pending)+len(failed))
	result := make([]Event, 0, len(pending)+len(failed))
	for _, event := range pending {
		if _, ok := seen[event.ID]; ok {
			continue
		}
		seen[event.ID] = struct{}{}
		result = append(result, event)
	}
	for _, event := range failed {
		if event.Attempts >= d.cfg.MaxAttempts {
			continue
		}
		if now.Sub(event.UpdatedAt) < d.backoff(event.Attempts) {
			continue
		}
		if _, ok := seen[event.ID]; ok {
			continue
		}
		seen[event.ID] = struct{}{}
		result = append(result, event)
	}
	return result, nil
}

func (d *Dispatcher) targets(topic string) []connector.Connector {
	all := d.connectors.All()
	result := make([]connector.Connector, 0, len(all))
	for _, item := range all {
		if connector.MatchesTopic(item, topic) {
			result = append(result, item)
		}
	}
	return result
}

func (d *Dispatcher) backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	wait := d.cfg.BaseBackoff
	for i := 1; i < attempts; i++ {
		wait *= 2
		if wait >= d.cfg.MaxBackoff {
			return d.cfg.MaxBackoff
		}
	}
	if wait > d.cfg.MaxBackoff {
		return d.cfg.MaxBackoff
	}
	return wait
}

func (d *Dispatcher) runOnceSafe(ctx context.Context) {
	delivered, failed, err := d.RunOnce(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Warn().Err(err).Msg("outbox: 分发轮询失败")
		}
		return
	}
	if delivered > 0 || failed > 0 {
		logger.Info().Int("delivered", delivered).Int("failed", failed).Msg("outbox: 分发完成")
	}
}

func truncateReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= 900 {
		return reason
	}
	return reason[:900]
}
