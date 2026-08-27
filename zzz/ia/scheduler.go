// Package ia 内部调度器 — 驱动行情刷新、自动计息、每日快照与到期检测
package ia

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

// 调度时间配置
const (
	tickInterval     = 30 * time.Second // 调度器心跳
	quoteEvery       = 30 * time.Minute // 行情刷新间隔（交易时段内）
	accrueAt         = "07:30"          // 每日计息触发时刻
	snapshotAt       = "16:35"          // 每日快照触发时刻（收盘后）
	maturityCheckAt  = "09:05"          // 每日到期检测触发时刻
	riskCheckAt      = "16:45"          // 每日风险巡检触发时刻
	quoteWindowStart = 9                // 行情窗口起始小时（含）
	quoteWindowEnd   = 16               // 行情窗口结束小时（不含）
)

// JobScheduler 投资模块内部调度器。
// 单实例内存标记任务当日是否已执行（at-most-once，失败次日重试或手动触发）；
// 进程重启后由幂等逻辑兜底（计息/快照均按日去重）。
type JobScheduler struct {
	svc          *Service
	ticker       *time.Ticker
	stopCh       chan struct{}
	once         sync.Once
	mu           sync.Mutex
	lastQuoteRun time.Time
	dailyDone    map[string]string // 任务名 → 最近执行日期 YYYY-MM-DD
}

// NewJobScheduler 创建调度器
func NewJobScheduler(svc *Service) *JobScheduler {
	return &JobScheduler{
		svc:       svc,
		stopCh:    make(chan struct{}),
		dailyDone: make(map[string]string),
	}
}

// Start 启动调度循环
func (s *JobScheduler) Start() {
	s.once.Do(func() {
		s.ticker = time.NewTicker(tickInterval)
		go func() {
			for {
				select {
				case <-s.ticker.C:
					s.tick()
				case <-s.stopCh:
					return
				}
			}
		}()
		logger.Info().Msg("zzz-ia: 调度器已启动")
	})
}

// Stop 停止调度循环
func (s *JobScheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// tick 单次调度检查：行情窗口内按间隔刷新；每日任务按时刻触发一次
func (s *JobScheduler) tick() {
	now := time.Now()

	// 行情刷新：工作日交易时段窗口内按间隔执行
	if s.inQuoteWindow(now) && now.Sub(s.lastQuoteRun) >= quoteEvery {
		s.lastQuoteRun = now
		s.runAsync("quotes", func(ctx context.Context) (string, error) {
			res, err := s.svc.RefreshQuotes(ctx)
			if res != nil {
				return summaryf("扫描 %d 只标的，更新 %d，失败 %d", res.Scanned, res.Updated, res.Failed), err
			}
			return "", err
		})
	}

	// 每日任务
	s.runDailyOnce(accrueAt, "accrue", func(ctx context.Context) (string, error) {
		res, err := s.svc.AccrueInterest(ctx)
		if res != nil {
			return summaryf("计息 %d 笔共 %.2f，跳过 %d", res.Accrued, res.TotalInterest, res.Skipped), err
		}
		return "", err
	})
	s.runDailyOnce(snapshotAt, "snapshot", func(ctx context.Context) (string, error) {
		res, err := s.svc.GenerateSnapshots(ctx)
		if res != nil {
			return summaryf("生成 %d 份快照，失败 %d", res.Accounts, res.Failed), err
		}
		return "", err
	})
	s.runDailyOnce(maturityCheckAt, "maturity", func(ctx context.Context) (string, error) {
		res, err := s.svc.CheckMaturities(ctx)
		if res != nil {
			return summaryf("发现 %d 笔临近到期资产", res.Count), err
		}
		return "", err
	})
	s.runDailyOnce(riskCheckAt, "risk", func(ctx context.Context) (string, error) {
		res, err := s.svc.CheckRiskAlerts(ctx)
		if res != nil {
			return summaryf("集中度 Top1 %.0f%%，HHI %.3f", res.Top1Weight*100, res.HHI), err
		}
		return "", err
	})
}

// inQuoteWindow 是否处于行情刷新窗口（周一至周五 9:00-16:00）
func (s *JobScheduler) inQuoteWindow(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}
	return t.Hour() >= quoteWindowStart && t.Hour() < quoteWindowEnd
}

// runDailyOnce 当日指定时刻后仅执行一次的任务
func (s *JobScheduler) runDailyOnce(atTime, jobName string, job func(ctx context.Context) (string, error)) {
	now := time.Now()
	if hm := now.Format("15:04"); hm < atTime {
		return
	}
	today := now.Format(dateLayout)
	s.mu.Lock()
	done := s.dailyDone[jobName] == today
	s.mu.Unlock()
	if done {
		return
	}
	s.mu.Lock()
	s.dailyDone[jobName] = today
	s.mu.Unlock()
	s.runAsync(jobName, job)
}

// runAsync 异步执行任务并记录日志
func (s *JobScheduler) runAsync(name string, job func(ctx context.Context) (string, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Msgf("zzz-ia: 任务 %s panic: %v", name, r)
			}
		}()
		start := time.Now()
		summary, err := job(ctx)
		entry := logger.Info().Str("job", name).Dur("elapsed", time.Since(start))
		if err != nil {
			entry.Err(err).Str("summary", summary).Msg("zzz-ia: 定时任务执行失败")
			return
		}
		entry.Str("summary", summary).Msg("zzz-ia: 定时任务执行完成")
	}()
}

// summaryf 生成任务摘要文本
func summaryf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
