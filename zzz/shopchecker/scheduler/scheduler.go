// Package scheduler 提供定时检查调度器
package scheduler

import (
	"context"
	"sync"
	"time"
)

// DueProductFinder 查找需要检查的商品 ID，避免与 shopchecker 包循环依赖
type DueProductFinder interface {
	FindDueProductIDs(ctx context.Context, limit int) ([]string, error)
}

// CheckRunner 对指定商品 ID 执行一次价格检查
type CheckRunner func(ctx context.Context, productID string) error

// Scheduler 定时价格检查调度器，周期性地扫描到期商品并执行价格抓取
type Scheduler struct {
	finder       DueProductFinder
	runner       CheckRunner
	pollInterval time.Duration
	ticker       *time.Ticker
	stopCh       chan struct{}
	sem          chan struct{}
	once         sync.Once
}

// New 创建 Scheduler
func New(finder DueProductFinder, runner CheckRunner) *Scheduler {
	return &Scheduler{
		finder:       finder,
		runner:       runner,
		pollInterval: 1 * time.Hour,
		stopCh:       make(chan struct{}),
		sem:          make(chan struct{}, 5),
	}
}

// SetPollInterval 设置到期商品轮询间隔（允许到期时间出现最多一个轮询周期的偏差）
func (s *Scheduler) SetPollInterval(d time.Duration) {
	if d > 0 {
		s.pollInterval = d
	}
}

// Start 启动调度器，周期性扫描到期商品
func (s *Scheduler) Start() {
	s.once.Do(func() {
		s.ticker = time.NewTicker(s.pollInterval)
		go func() {
			for {
				select {
				case <-s.ticker.C:
					s.processDueProducts()
				case <-s.stopCh:
					return
				}
			}
		}()
	})
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
}

// processDueProducts 扫描到期商品并并发执行价格检查
func (s *Scheduler) processDueProducts() {
	// 休眠时段（1:00-9:00）跳过采集
	now := time.Now()
	h := now.Hour()
	if h >= 1 && h < 9 {
		return
	}

	ids, err := s.finder.FindDueProductIDs(context.Background(), 20)
	if err != nil || len(ids) == 0 {
		return
	}

	var wg sync.WaitGroup

	for _, id := range ids {
		wg.Add(1)

		go func(productID string) {
			defer wg.Done()
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			runnerCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			s.runner(runnerCtx, productID)
		}(id)
	}

	wg.Wait()
}
