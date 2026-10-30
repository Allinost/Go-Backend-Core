// Package ia 个人投资分析模块（Investment Analytics, zzz-IA）。
//
// 提供多账户投资组合的资产标的管理、资金流水、交易记录、份额成本核算
// （加权平均/FIFO 双法）、费用与分红台账、持仓状态机、汇率折算、
// 资产配比目标与定投计划等能力。行情接入与定时快照在后续版本迭代。
package ia

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	centralscheduler "github.com/Allinost/go-backend-core/internal/services/scheduler"
	"github.com/Allinost/go-backend-core/internal/services/ws"
	pkgNet "github.com/Allinost/go-backend-core/pkg/net"

	iaquote "github.com/Allinost/go-backend-core/zzz/ia/quote"
)

// Module 投资分析模块，实现 modules.Module 接口
type Module struct {
	cfg    *config.Config
	h      *Handler
	svc    *Service
	repo   *Repository
	sched  *JobScheduler
	quotes *iaquote.Registry
	whSvc  *webhook.Service
}

// Name 返回模块名称，用于路由前缀 /api/v1/zzz-ia/*
func (m *Module) Name() string {
	return "zzz-ia"
}

// Init 初始化模块：获取 MySQL 连接，初始化仓库/行情源/Service/调度器并自动建表
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("zzz-ia: MySQL 连接不可用")
	}

	m.repo = NewRepository(pool.DB)
	m.svc = NewService(m.repo)

	// 行情数据源（HTTP 客户端带重试与熔断，支持 config.yaml 外部化配置）
	httpClient := pkgNet.NewHTTPClient(iaHTTPConfig(cfg))
	m.quotes = iaquote.NewRegistryWithConfig(httpClient, cfg)
	m.svc.AttachQuoteRegistry(m.quotes)

	// webhook 通知（可选：模块未注册时到期提醒仅记录日志）
	if whMod := modules.Get("webhook"); whMod != nil {
		m.whSvc = whMod.(*webhook.Module).GetService()
		m.svc.AttachWebhook(m.whSvc)
	} else {
		logger.Warn().Msg("zzz-ia: webhook 模块未注册，到期提醒将不推送通知")
	}

	m.sched = NewJobScheduler(m.svc)
	m.sched.Start()
	m.registerSchedulerTask()

	m.h = NewHandler(m.svc)

	// 预热快照缓存（后台异步，不阻塞启动）
	go warmScreenCache(context.Background(), m.quotes.ScreenHTTP())

	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	if m.sched != nil {
		m.sched.Stop()
	}
	return nil
}

// registerSchedulerTask 向 centralized scheduler 注册任务占位（管理可见性，实际由内部 ticker 驱动）
func (m *Module) registerSchedulerTask() {
	schedMod := modules.Get("scheduler")
	if schedMod == nil {
		return
	}
	s := schedMod.(*centralscheduler.Module).GetScheduler()
	if s == nil {
		return
	}
	tasks, err := s.ListTasks()
	if err != nil {
		return
	}
	const taskName = "zzz_ia_daily_maintenance"
	for _, t := range tasks {
		if t.Name == taskName {
			return
		}
	}
	s.AddTask(&centralscheduler.Task{
		Name:       taskName,
		Type:       centralscheduler.TaskTypeInterval,
		Expression: "30m",
		Handler:    taskName,
		Status:     centralscheduler.TaskStatusActive,
		Timeout:    300,
	})
	logger.Info().Msg("zzz-ia: 维护任务已注册到 centralized scheduler（管理可见性）")
}

// GetService 暴露 Service 实例，供其他模块调用投资分析能力
func (m *Module) GetService() *Service {
	return m.svc
}

// RegisterRoutes 注册 zzz-ia 模块路由到 /api/v1/zzz-ia/* 下
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// 账户
	r.GET("/accounts", m.h.ListAccounts)
	r.POST("/accounts", m.h.CreateAccount)
	r.GET("/accounts/:id", m.h.GetAccount)
	r.PUT("/accounts/:id", m.h.UpdateAccount)
	r.DELETE("/accounts/:id", m.h.DeleteAccount)

	// 标的
	r.GET("/assets", m.h.ListAssets)
	r.GET("/assets/resolve", m.h.ResolveAssetCode)
	r.POST("/assets", m.h.CreateAsset)
	r.GET("/assets/:id", m.h.GetAsset)
	r.PUT("/assets/:id", m.h.UpdateAsset)
	r.DELETE("/assets/:id", m.h.DeleteAsset)

	// 资金流水
	r.POST("/cash-flows", m.h.CreateCashFlow)
	r.GET("/cash-flows", m.h.ListCashFlows)
	r.DELETE("/cash-flows/:id", m.h.DeleteCashFlow)

	// 交易
	r.POST("/transactions", m.h.CreateTransaction)
	r.GET("/transactions", m.h.ListTransactions)
	r.GET("/transactions/:id", m.h.GetTransaction)
	r.POST("/transactions/:id/confirm", m.h.ConfirmTransaction)
	r.DELETE("/transactions/:id", m.h.DeleteTransaction)

	// 持仓与总览
	r.GET("/positions", m.h.ListPositions)
	r.PUT("/positions/valuation", m.h.UpdateLastPrice)
	r.POST("/positions/target", m.h.SetPositionTarget)
	r.GET("/overview", m.h.Overview)

	// 买卖建议
	r.GET("/advice", m.h.TradeAdvice)

	// 行情与净值历史
	r.POST("/quotes/refresh", m.h.RefreshQuotes)
	r.GET("/price-history", m.h.ListPriceHistory)

	// 定时任务手动触发
	r.POST("/tasks/accrue", m.h.TriggerAccrue)
	r.POST("/tasks/snapshot", m.h.TriggerSnapshot)
	r.POST("/tasks/maturity-check", m.h.TriggerMaturityCheck)

	// 分析报表
	r.GET("/analytics/equity-curve", m.h.AnalyticsEquityCurve)
	r.GET("/analytics/returns", m.h.AnalyticsReturns)
	r.GET("/analytics/trades-review", m.h.AnalyticsTradesReview)
	r.GET("/analytics/risk", m.h.AnalyticsRisk)
	r.GET("/analytics/dca-simulate", m.h.AnalyticsDCASimulate)
	r.GET("/analytics/fees", m.h.AnalyticsFees)
	r.GET("/analytics/dividends", m.h.AnalyticsDividends)

	// 独立费用
	r.POST("/fees", m.h.CreateFee)
	r.GET("/fees", m.h.ListFees)
	r.DELETE("/fees/:id", m.h.DeleteFee)

	// 定投计划
	r.GET("/dca-plans", m.h.ListDCAPlans)
	r.POST("/dca-plans", m.h.CreateDCAPlan)
	r.GET("/dca-plans/:id", m.h.GetDCAPlan)
	r.PUT("/dca-plans/:id", m.h.UpdateDCAPlan)
	r.DELETE("/dca-plans/:id", m.h.DeleteDCAPlan)

	// 汇率
	r.PUT("/fx-rates", m.h.UpsertFxRate)
	r.GET("/fx-rates", m.h.ListFxRates)

	// 资产配比
	r.GET("/allocations", m.h.ListAllocations)
	r.POST("/allocations", m.h.CreateAllocation)
	r.PUT("/allocations/:id", m.h.UpdateAllocation)
	r.DELETE("/allocations/:id", m.h.DeleteAllocation)
	r.GET("/allocations/review", m.h.AllocationReview)

	// 价格告警
	r.GET("/alerts", m.h.ListAlerts)
	r.POST("/alerts", m.h.CreateAlert)
	r.GET("/alerts/:id", m.h.GetAlert)
	r.PUT("/alerts/:id", m.h.UpdateAlert)
	r.DELETE("/alerts/:id", m.h.DeleteAlert)

	// 告警与风险巡检手动触发
	r.POST("/tasks/check-alerts", m.h.TriggerAlertCheck)
	r.POST("/tasks/check-risk", m.h.TriggerRiskCheck)

	// 对账单导入
	r.POST("/import/transactions", m.h.ImportTransactions)

	// 个股筛选与指数分析
	r.GET("/screen/dynamic-potential", m.h.ScreenDynamicPotential)
	r.GET("/screen/quality-stocks", m.h.ScreenQualityStocks)
	r.GET("/analytics/index-analysis", m.h.AnalyticsIndexAnalysis)

	// 汇率自动刷新
	r.POST("/tasks/refresh-fx", m.h.TriggerFxRefresh)
}

// SetWSHub 注入 WebSocket Hub（可选：hub 就绪后调用即可启用实时推送）
func (m *Module) SetWSHub(hub *ws.Hub) {
	if hub != nil {
		m.svc.SetNotifier(hubAdapter{h: hub})
	}
}

// hubAdapter 适配 ws.Hub 到 RoomNotifier 接口
type hubAdapter struct{ h *ws.Hub }

func (a hubAdapter) BroadcastToRoom(room, msgType string, payload []byte) {
	a.h.BroadcastToRoom(room, ws.Message{Room: room, Type: msgType, Payload: payload})
}

// initTables 自动建表（12 张）

// iaHTTPConfig 从 config 构建 IA 模块专用 HTTP 配置（含断路器）
func iaHTTPConfig(cfg *config.Config) pkgNet.HTTPConfig {
	httpCfg := pkgNet.DefaultHTTPConfig()

	// 从 config.yaml 覆盖基础字段
	if cfg.Net.HTTP.DefaultTimeout > 0 {
		httpCfg.Timeout = cfg.Net.HTTP.DefaultTimeout
	}
	if cfg.Net.HTTP.MaxRetries > 0 {
		httpCfg.RetryMax = cfg.Net.HTTP.MaxRetries
	}
	if cfg.Net.HTTP.RetryWaitMin > 0 {
		httpCfg.RetryWaitMin = cfg.Net.HTTP.RetryWaitMin
	}
	if cfg.Net.HTTP.RetryWaitMax > 0 {
		httpCfg.RetryWaitMax = cfg.Net.HTTP.RetryWaitMax
	}

	// 主断路器配置
	brk := config.DefaultBreakerConfig()
	if cfg.Net.HTTP.Breaker.FailureThreshold > 0 {
		brk.FailureThreshold = cfg.Net.HTTP.Breaker.FailureThreshold
	}
	if cfg.Net.HTTP.Breaker.SuccessThreshold > 0 {
		brk.SuccessThreshold = cfg.Net.HTTP.Breaker.SuccessThreshold
	}
	if cfg.Net.HTTP.Breaker.Timeout > 0 {
		brk.Timeout = cfg.Net.HTTP.Breaker.Timeout
	}
	httpCfg.Breaker = pkgNet.NewCircuitBreaker(pkgNet.CircuitBreakerConfig{
		FailureThreshold: brk.FailureThreshold,
		SuccessThreshold: brk.SuccessThreshold,
		Timeout:          brk.Timeout,
	})
	httpCfg.DefaultHeaders = map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Referer":    "https://quote.eastmoney.com/",
	}
	return httpCfg
}

// iaScreenHTTPConfig 构建筛选专用 HTTP 配置（独立断路器）
func iaScreenHTTPConfig(cfg *config.Config) pkgNet.HTTPConfig {
	httpCfg := pkgNet.DefaultHTTPConfig()
	httpCfg.Timeout = 30 * time.Second

	sbrk := config.DefaultScreenBreakerConfig()
	if cfg.Net.HTTP.ScreenBreaker.FailureThreshold > 0 {
		sbrk.FailureThreshold = cfg.Net.HTTP.ScreenBreaker.FailureThreshold
	}
	if cfg.Net.HTTP.ScreenBreaker.SuccessThreshold > 0 {
		sbrk.SuccessThreshold = cfg.Net.HTTP.ScreenBreaker.SuccessThreshold
	}
	if cfg.Net.HTTP.ScreenBreaker.Timeout > 0 {
		sbrk.Timeout = cfg.Net.HTTP.ScreenBreaker.Timeout
	}
	httpCfg.Breaker = pkgNet.NewCircuitBreaker(pkgNet.CircuitBreakerConfig{
		FailureThreshold: sbrk.FailureThreshold,
		SuccessThreshold: sbrk.SuccessThreshold,
		Timeout:          sbrk.Timeout,
	})
	httpCfg.DefaultHeaders = map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Referer":    "https://quote.eastmoney.com/",
	}
	return httpCfg
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
