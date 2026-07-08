package monitor

import (
	"context"
	"sync"
	"time"
)

type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

type CheckResult struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Latency time.Duration `json:"latency_ms"`
	Error   string        `json:"error,omitempty"`
}

type CheckFunc func(ctx context.Context) CheckResult

type PollingConfig struct {
	Enabled  bool
	Interval time.Duration
}

type StatusChangeHandler func(name string, from, to Status)

type HealthChecker struct {
	mu        sync.RWMutex
	checks    map[string]CheckFunc
	cached    map[string]CheckResult
	previous  map[string]Status
	stopCh    chan struct{}
	polling   bool
	closeOnce sync.Once
	onChange  []StatusChangeHandler
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]CheckFunc),
	}
}

func (h *HealthChecker) Register(name string, fn CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = fn
}

func (h *HealthChecker) Remove(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checks, name)
	if h.cached != nil {
		delete(h.cached, name)
	}
}

func (h *HealthChecker) Check(ctx context.Context) map[string]CheckResult {
	h.mu.RLock()
	checks := make(map[string]CheckFunc, len(h.checks))
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	results := make(map[string]CheckResult, len(checks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, fn := range checks {
		wg.Add(1)
		go func(name string, fn CheckFunc) {
			defer wg.Done()
			start := time.Now()

			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			result := fn(checkCtx)
			result.Name = name
			result.Latency = time.Since(start)

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, fn)
	}

	wg.Wait()
	return results
}

func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	for _, result := range h.Check(ctx) {
		if result.Status == StatusDown {
			return false
		}
	}
	return true
}

func (h *HealthChecker) StartPolling(ctx context.Context, cfg PollingConfig) {
	if !cfg.Enabled {
		return
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}

	h.mu.Lock()
	if h.polling {
		h.mu.Unlock()
		return
	}
	h.polling = true
	h.stopCh = make(chan struct{})
	h.mu.Unlock()

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		h.pollOnce(ctx)

		for {
			select {
			case <-ticker.C:
				h.pollOnce(ctx)
			case <-h.stopCh:
				h.mu.Lock()
				h.polling = false
				h.mu.Unlock()
				return
			case <-ctx.Done():
				h.mu.Lock()
				h.polling = false
				h.mu.Unlock()
				return
			}
		}
	}()
}

func (h *HealthChecker) Stop() {
	h.mu.RLock()
	ch := h.stopCh
	h.mu.RUnlock()
	if ch != nil {
		h.closeOnce.Do(func() { close(ch) })
	}
}

func (h *HealthChecker) LastResult() map[string]CheckResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.cached == nil {
		return nil
	}

	result := make(map[string]CheckResult, len(h.cached))
	for k, v := range h.cached {
		result[k] = v
	}
	return result
}

func (h *HealthChecker) OnStatusChange(handler StatusChangeHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChange = append(h.onChange, handler)
}

func (h *HealthChecker) pollOnce(ctx context.Context) {
	results := h.Check(ctx)
	h.mu.Lock()
	h.cached = results

	if h.previous == nil {
		h.previous = make(map[string]Status)
	}
	for name, result := range results {
		prev, exists := h.previous[name]
		if exists && prev != result.Status {
			for _, handler := range h.onChange {
				handler(name, prev, result.Status)
			}
		}
		h.previous[name] = result.Status
	}
	h.mu.Unlock()
}
