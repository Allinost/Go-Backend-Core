// Package shopchecker 商品价格监控模块，提供商品价格监测、降价通知、仪表盘等功能。
package shopchecker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/zzz/shopchecker/collector"
	"github.com/Allinost/go-backend-core/zzz/shopchecker/scheduler"
	"github.com/gin-gonic/gin"
)

// Module 商品价格监控模块，实现 modules.Module 接口
type Module struct {
	cfg         *config.Config
	h           *Handler
	svc         *Service
	repo        *Repository
	sched       *scheduler.Scheduler
	colRegistry *collector.Registry
	whSvc       *webhook.Service
}

// Name 返回模块名称，用于路由前缀 /api/v1/shopchecker/*
func (m *Module) Name() string {
	return "shopchecker"
}

// Init 初始化模块：获取 MySQL 连接，创建仓库/采集器注册表/Service/Scheduler/Handler，自动建表
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("shopchecker: MySQL 连接不可用")
	}

	m.repo = NewRepository(pool.DB)

	m.colRegistry = collector.NewRegistry()
	m.colRegistry.Register(string(PlatformJD), collector.NewJDCollector())
	m.colRegistry.Register(string(PlatformTaobao), collector.NewTaobaoCollector())
	m.colRegistry.Register(string(PlatformPDD), collector.NewPDDCollector())

	whMod := modules.Get("webhook")
	if whMod == nil {
		return fmt.Errorf("shopchecker: webhook 模块未注册")
	}
	m.whSvc = whMod.(*webhook.Module).GetService()

	m.svc = NewService(m.repo, m.whSvc, m.colRegistry)

	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("shopchecker: 初始化数据表失败: %w", err)
	}

	m.sched = scheduler.New(m.svc, m.svc.RunCheck)
	m.sched.Start()

	m.h = NewHandler(m.svc, m.colRegistry)
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	if m.sched != nil {
		m.sched.Stop()
	}
	return nil
}

// GetService 暴露 Service 实例，供其他模块调用价格检查能力
func (m *Module) GetService() *Service {
	return m.svc
}

// RegisterRoutes 注册模块路由到 /api/v1/shopchecker/* 下
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/products", m.h.ListProducts)
	r.POST("/products", m.h.CreateProduct)
	r.GET("/products/:id", m.h.GetProduct)
	r.PUT("/products/:id", m.h.UpdateProduct)
	r.DELETE("/products/:id", m.h.DeleteProduct)
	r.POST("/products/:id/check", m.h.ManualCheck)
	r.GET("/products/:id/price-records", m.h.ListPriceRecords)
	r.GET("/products/:id/notification-logs", m.h.ListNotificationLogs)
	r.GET("/dashboard", m.h.Dashboard)
}

// initTables 自动建表
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS zzz_shopchecker_products (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			platform VARCHAR(20) NOT NULL,
			url VARCHAR(2048) NOT NULL,
			image_url VARCHAR(1024) DEFAULT '',
			target_price DECIMAL(12,2) DEFAULT 0,
			price_drop_ratio DECIMAL(5,2) DEFAULT 0,
			last_price DECIMAL(12,2) DEFAULT 0,
			last_stock_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
			last_checked_at DATETIME NULL,
			next_check_at DATETIME NULL,
			check_interval VARCHAR(20) NOT NULL DEFAULT '1h',
			cron_expr VARCHAR(100) DEFAULT '',
			custom_webhook_ids JSON NULL,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			remark TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_enabled_next_check (enabled, next_check_at),
			INDEX idx_platform (platform)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_shopchecker_price_records (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			product_id VARCHAR(36) NOT NULL,
			price DECIMAL(12,2) NOT NULL,
			original_price DECIMAL(12,2) DEFAULT 0,
			title VARCHAR(255) DEFAULT '',
			stock_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
			is_notified TINYINT(1) NOT NULL DEFAULT 0,
			raw_data JSON NULL,
			error_msg VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_product_id (product_id),
			INDEX idx_created (created_at),
			INDEX idx_is_notified (is_notified)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_shopchecker_notification_logs (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			product_id VARCHAR(36) NOT NULL,
			price_record_id BIGINT NOT NULL,
			webhook_ids JSON NULL,
			old_price DECIMAL(12,2) DEFAULT 0,
			new_price DECIMAL(12,2) NOT NULL,
			old_stock_status VARCHAR(20) NOT NULL DEFAULT '',
			new_stock_status VARCHAR(20) NOT NULL DEFAULT '',
			notify_type VARCHAR(20) NOT NULL DEFAULT 'price_drop',
			result_summary VARCHAR(20) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_product_id (product_id),
			INDEX idx_price_record_id (price_record_id),
			INDEX idx_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}

	migrations := []string{
		"ALTER TABLE `zzz_shopchecker_products` ADD COLUMN `last_stock_status` VARCHAR(20) NOT NULL DEFAULT 'unknown' AFTER `last_price`",
		"ALTER TABLE `zzz_shopchecker_price_records` ADD COLUMN `stock_status` VARCHAR(20) NOT NULL DEFAULT 'unknown' AFTER `title`",
		"ALTER TABLE `zzz_shopchecker_notification_logs` ADD COLUMN `old_stock_status` VARCHAR(20) NOT NULL DEFAULT '' AFTER `new_price`",
		"ALTER TABLE `zzz_shopchecker_notification_logs` ADD COLUMN `new_stock_status` VARCHAR(20) NOT NULL DEFAULT '' AFTER `old_stock_status`",
		"ALTER TABLE `zzz_shopchecker_notification_logs` ADD COLUMN `notify_type` VARCHAR(20) NOT NULL DEFAULT 'price_drop' AFTER `new_stock_status`",
	}
	for _, m := range migrations {
		db.ExecContext(ctx, m)
	}

	logger.Info().Msg("shopchecker: 数据表初始化完成")
	return nil
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
