// Package webhook 公共 Webhook 管理模块，提供统一的 Webhook 配置管理、多类型消息发送、限流去重能力。
package webhook

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/dedup"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/limiter"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/sender"
	"github.com/gin-gonic/gin"
)

// Module Webhook 模块，实现 modules.Module 接口
type Module struct {
	cfg  *config.Config
	h    *Handler
	svc  *Service
	repo *Repository
}

// Name 返回模块名称，用于路由前缀 /api/v1/webhook/*
func (m *Module) Name() string {
	return "webhook"
}

// Init 初始化模块：获取 MySQL 连接和 Redis（可选），初始化仓库/发送器工厂/限流器/去重器，自动建表并注册路由
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("webhook: MySQL 连接不可用")
	}

	var rdb *redis.Client
	if c, ok := database.DB.Redis["main"]; ok {
		rdb = c.Client
	} else {
		logger.Warn().Msg("webhook: Redis 不可用，限流和去重功能将禁用")
	}

	m.repo = NewRepository(pool.DB, rdb)

	senderFactory := sender.NewFactory()
	rateLimiter := limiter.New(rdb)
	deduper := dedup.New(rdb)

	m.svc = NewService(m.repo, senderFactory, rateLimiter, deduper)

	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("webhook: 初始化数据表失败: %w", err)
	}

	m.h = NewHandler(m.svc)
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	return nil
}

// GetService 暴露 Service 实例，供其他模块调用 Webhook 发送能力
func (m *Module) GetService() *Service {
	return m.svc
}

// RegisterRoutes 注册 webhook 模块路由到 /api/v1/webhook/* 下
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/configs", m.h.ListConfigs)
	r.POST("/configs", m.h.CreateConfig)
	r.GET("/configs/:id", m.h.GetConfig)
	r.PUT("/configs/:id", m.h.UpdateConfig)
	r.DELETE("/configs/:id", m.h.DeleteConfig)
	r.POST("/configs/:id/status", m.h.UpdateStatus)
	r.POST("/configs/:id/test", m.h.TestSend)

	r.POST("/send", m.h.Send)

	r.GET("/delivery-logs", m.h.ListDeliveryLogs)
	r.DELETE("/delivery-logs/:id", m.h.DeleteDeliveryLog)
	r.POST("/delivery-logs/batch-delete", m.h.BatchDeleteDeliveryLogs)

	r.GET("/stats", m.h.Stats)
	r.GET("/configs/:id/stats", m.h.ConfigStats)
}

// initTables 自动建表：zzz_webhook_configs（配置表）和 zzz_webhook_delivery_logs（投递日志表）
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS webhook_configs (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			type VARCHAR(20) NOT NULL,
			url VARCHAR(2048) NOT NULL,
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			tags JSON DEFAULT NULL,
			description VARCHAR(500) DEFAULT '',
			source VARCHAR(50) NOT NULL DEFAULT 'system',
			app VARCHAR(50) DEFAULT '',
			signing_key VARCHAR(255) DEFAULT '',
			access_token VARCHAR(255) DEFAULT '',
			auth_type VARCHAR(20) DEFAULT '',
			auth_value TEXT DEFAULT NULL,
			custom_headers JSON DEFAULT NULL,
			whitelist_keywords JSON DEFAULT NULL,
			whitelist_mode VARCHAR(10) DEFAULT 'any',
			max_per_minute INT DEFAULT 60,
			max_per_hour INT DEFAULT 1000,
			dedup_window INT DEFAULT 0,
			max_retries INT DEFAULT 3,
			retry_interval INT DEFAULT 30,
			timeout_ms INT DEFAULT 10000,
			last_sent_at DATETIME DEFAULT NULL,
			last_success_at DATETIME DEFAULT NULL,
			last_error VARCHAR(500) DEFAULT '',
			total_sent INT DEFAULT 0,
			total_failed INT DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_status (status),
			INDEX idx_type (type),
			INDEX idx_source_app (source, app)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS webhook_delivery_logs (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			webhook_id VARCHAR(36) NOT NULL,
			source VARCHAR(50) NOT NULL,
			title VARCHAR(255) DEFAULT '',
			content_hash VARCHAR(64) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			request_body TEXT DEFAULT NULL,
			response_code INT DEFAULT 0,
			response_body TEXT DEFAULT NULL,
			error_msg VARCHAR(500) DEFAULT '',
			retry_count INT DEFAULT 0,
			duration_ms INT DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_webhook_status (webhook_id, status),
			INDEX idx_content_hash (content_hash, created_at),
			INDEX idx_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	logger.Info().Msg("webhook: 数据表初始化完成")
	return nil
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)