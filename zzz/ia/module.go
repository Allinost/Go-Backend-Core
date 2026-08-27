// Package ia 个人投资分析模块（Investment Analytics, zzz-IA）。
//
// 提供多账户投资组合的资产标的管理、资金流水、交易记录、份额成本核算
// （加权平均/FIFO 双法）、费用与分红台账、持仓状态机、汇率折算、
// 资产配比目标与定投计划等能力。行情接入与定时快照在后续版本迭代。
package ia

import (
	"context"
	"database/sql"
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

	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("zzz-ia: 初始化数据表失败: %w", err)
	}

	m.sched = NewJobScheduler(m.svc)
	m.sched.Start()
	m.registerSchedulerTask()

	m.h = NewHandler(m.svc)
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
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		// 账户表
		`CREATE TABLE IF NOT EXISTS zzz_ia_accounts (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			type VARCHAR(20) NOT NULL DEFAULT 'broker',
			broker VARCHAR(100) DEFAULT '',
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			cost_method VARCHAR(10) NOT NULL DEFAULT 'wa',
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_type_status (type, status),
			INDEX idx_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 投资账户'`,

		// 标的主数据表
		`CREATE TABLE IF NOT EXISTS zzz_ia_assets (
			id VARCHAR(36) PRIMARY KEY,
			code VARCHAR(32) NOT NULL,
			name VARCHAR(100) NOT NULL,
			asset_type VARCHAR(20) NOT NULL,
			sub_type VARCHAR(20) DEFAULT '',
			market VARCHAR(20) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			tracking_index VARCHAR(32) DEFAULT '',
			annual_rate DECIMAL(10,6) DEFAULT NULL,
			start_date DATE DEFAULT NULL,
			maturity_date DATE DEFAULT NULL,
			lock_days INT DEFAULT 0,
			risk_level VARCHAR(10) DEFAULT '',
			issuer VARCHAR(100) DEFAULT '',
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_code_market (code, market),
			INDEX idx_type_market (asset_type, market),
			INDEX idx_maturity (maturity_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 标的主数据'`,

		// 资金流水表
		`CREATE TABLE IF NOT EXISTS zzz_ia_cash_flows (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			flow_type VARCHAR(20) NOT NULL,
			amount DECIMAL(18,4) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			flow_date DATE NOT NULL,
			counter_account_id VARCHAR(36) DEFAULT NULL,
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_acct_date (account_id, flow_date),
			INDEX idx_type (flow_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 资金流水'`,

		// 交易流水表
		`CREATE TABLE IF NOT EXISTS zzz_ia_transactions (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) NOT NULL,
			trade_type VARCHAR(20) NOT NULL,
			shares DECIMAL(20,6) DEFAULT 0,
			price DECIMAL(18,6) DEFAULT 0,
			amount DECIMAL(18,4) DEFAULT 0,
			fee DECIMAL(18,4) DEFAULT 0,
			fee_detail JSON DEFAULT NULL,
			tax DECIMAL(18,4) DEFAULT 0,
			trade_at DATETIME NOT NULL,
			settle_date DATE DEFAULT NULL,
			status VARCHAR(10) NOT NULL DEFAULT 'confirmed',
			method VARCHAR(10) DEFAULT '',
			realized_pnl DECIMAL(18,4) DEFAULT NULL,
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_acct_asset_status (account_id, asset_id, status),
			INDEX idx_trade_at (trade_at),
			INDEX idx_type (trade_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 交易流水'`,

		// 独立费用表
		`CREATE TABLE IF NOT EXISTS zzz_ia_fees (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) DEFAULT NULL,
			fee_type VARCHAR(20) NOT NULL,
			amount DECIMAL(18,4) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			charge_date DATE NOT NULL,
			period VARCHAR(20) DEFAULT '',
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_acct_date (account_id, charge_date),
			INDEX idx_type (fee_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 独立费用'`,

		// 实时持仓表
		`CREATE TABLE IF NOT EXISTS zzz_ia_positions (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) NOT NULL,
			shares DECIMAL(20,6) NOT NULL DEFAULT 0,
			available_shares DECIMAL(20,6) NOT NULL DEFAULT 0,
			locked_shares DECIMAL(20,6) NOT NULL DEFAULT 0,
			avg_cost DECIMAL(18,6) NOT NULL DEFAULT 0,
			cost_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
			last_price DECIMAL(18,6) DEFAULT NULL,
			last_price_at DATETIME DEFAULT NULL,
			last_price_source VARCHAR(20) DEFAULT '',
			target_price DECIMAL(18,6) DEFAULT NULL,
			stop_loss_price DECIMAL(18,6) DEFAULT NULL,
			target_set_at DATETIME DEFAULT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			status VARCHAR(10) NOT NULL DEFAULT 'holding',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_acct_asset (account_id, asset_id),
			INDEX idx_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 实时持仓'`,

		// 价格/净值历史表
		`CREATE TABLE IF NOT EXISTS zzz_ia_price_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			asset_id VARCHAR(36) NOT NULL,
			nav_date DATE NOT NULL,
			nav DECIMAL(18,6) NOT NULL,
			source VARCHAR(20) NOT NULL DEFAULT 'official',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_asset_date (asset_id, nav_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 价格净值历史'`,

		// 组合每日快照表
		`CREATE TABLE IF NOT EXISTS zzz_ia_portfolio_snapshots (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			snap_date DATE NOT NULL,
			account_id VARCHAR(36) DEFAULT NULL,
			total_cost DECIMAL(20,4) DEFAULT 0,
			total_value DECIMAL(20,4) DEFAULT 0,
			total_pnl DECIMAL(20,4) DEFAULT 0,
			cash_amount DECIMAL(20,4) DEFAULT 0,
			by_type JSON DEFAULT NULL,
			by_currency JSON DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_date_acct (snap_date, account_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 组合快照'`,

		// 已实现收益台账
		`CREATE TABLE IF NOT EXISTS zzz_ia_realized_pnl (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) NOT NULL,
			transaction_id BIGINT NOT NULL,
			lot_transaction_id BIGINT DEFAULT NULL,
			method VARCHAR(10) NOT NULL DEFAULT 'wa',
			shares DECIMAL(20,6) NOT NULL DEFAULT 0,
			sell_price DECIMAL(18,6) NOT NULL DEFAULT 0,
			cost_price DECIMAL(18,6) NOT NULL DEFAULT 0,
			gross_pnl DECIMAL(18,4) NOT NULL DEFAULT 0,
			fee DECIMAL(18,4) NOT NULL DEFAULT 0,
			tax DECIMAL(18,4) NOT NULL DEFAULT 0,
			net_pnl DECIMAL(18,4) NOT NULL DEFAULT 0,
			realized_at DATE NOT NULL,
			INDEX idx_acct_asset (account_id, asset_id),
			INDEX idx_tx (transaction_id),
			INDEX idx_date (realized_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 已实现收益台账'`,

		// 定投计划表
		`CREATE TABLE IF NOT EXISTS zzz_ia_dca_plans (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) NOT NULL,
			amount DECIMAL(18,4) NOT NULL,
			cycle VARCHAR(10) NOT NULL,
			exec_day INT NOT NULL DEFAULT 1,
			start_date DATE NOT NULL,
			end_date DATE DEFAULT NULL,
			next_exec_date DATE NOT NULL,
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			auto_buy TINYINT(1) NOT NULL DEFAULT 0,
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_status_next (status, next_exec_date),
			INDEX idx_acct (account_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 定投计划'`,

		// 汇率表
		`CREATE TABLE IF NOT EXISTS zzz_ia_fx_rates (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			base_ccy VARCHAR(8) NOT NULL,
			quote_ccy VARCHAR(8) NOT NULL,
			rate DECIMAL(16,8) NOT NULL,
			rate_date DATE NOT NULL,
			source VARCHAR(10) NOT NULL DEFAULT 'manual',
			UNIQUE KEY uk_pair_date (base_ccy, quote_ccy, rate_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 汇率'`,

		// 资产配比目标表
		`CREATE TABLE IF NOT EXISTS zzz_ia_allocations (
			id VARCHAR(36) PRIMARY KEY,
			account_id VARCHAR(36) DEFAULT NULL,
			category VARCHAR(20) NOT NULL,
			target_ratio DECIMAL(7,6) NOT NULL,
			tolerance DECIMAL(7,6) NOT NULL DEFAULT 0.05,
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_scope_status (account_id, status),
			UNIQUE KEY uk_scope_category (account_id, category)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 资产配比目标'`,

		// 价格告警表
		`CREATE TABLE IF NOT EXISTS zzz_ia_price_alerts (
			id VARCHAR(36) PRIMARY KEY,
			account_id VARCHAR(36) NOT NULL,
			asset_id VARCHAR(36) NOT NULL,
			alert_type VARCHAR(20) NOT NULL,
			target_price DECIMAL(18,6) NOT NULL,
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			triggered_at DATETIME DEFAULT NULL,
			triggered_price DECIMAL(18,6) DEFAULT NULL,
			note VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_status_asset (status, asset_id),
			INDEX idx_acct (account_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 价格告警'`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	if err := migratePositionTargetCols(db); err != nil {
		return err
	}
	logger.Info().Msg("zzz-ia: 数据表初始化完成")
	return nil
}

// migratePositionTargetCols 存量 positions 表补齐追踪目标列（已存在则忽略）
func migratePositionTargetCols(db *sql.DB) error {
	compat := []string{
		"ALTER TABLE zzz_ia_positions ADD COLUMN target_price DECIMAL(18,6) DEFAULT NULL",
		"ALTER TABLE zzz_ia_positions ADD COLUMN stop_loss_price DECIMAL(18,6) DEFAULT NULL",
		"ALTER TABLE zzz_ia_positions ADD COLUMN target_set_at DATETIME DEFAULT NULL",
	}
	for _, ddl := range compat {
		// 列已存在时报 Duplicate column name，静默忽略
		_, _ = db.ExecContext(context.Background(), ddl)
	}
	return nil
}

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
	return httpCfg
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
