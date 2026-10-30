// Package webhook 公共 Webhook 管理模块，提供统一的 Webhook 配置管理、多类型消息发送、限流去重能力。
package webhook

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/dedup"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/limiter"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/sender"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
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

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
