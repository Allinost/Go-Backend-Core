// Package shopchecker 商品价格监控模块，提供商品价格监测、降价通知、仪表盘等功能。
package shopchecker

import (
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	centralscheduler "github.com/Allinost/go-backend-core/internal/services/scheduler"
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
	m.colRegistry.Register(string(PlatformTmall), collector.NewTmallCollector())
	m.colRegistry.Register(string(PlatformPDD), collector.NewPDDCollector())
	m.colRegistry.Register(string(PlatformQixian), collector.NewQixianCollector())
	m.colRegistry.Register(string(PlatformJDMobile), collector.NewJDMobileCollector())

	whMod := modules.Get("webhook")
	if whMod == nil {
		return fmt.Errorf("shopchecker: webhook 模块未注册")
	}
	m.whSvc = whMod.(*webhook.Module).GetService()

	m.svc = NewService(m.repo, m.whSvc, m.colRegistry)

	m.sched = scheduler.New(m.svc, m.svc.RunCheck)
	m.sched.Start()

	m.registerSchedulerTask()

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

// registerSchedulerTask 在 centralized scheduler 中注册价格检查任务（仅用于管理可见性）
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
	for _, t := range tasks {
		if t.Name == "shopchecker_price_scan" {
			if t.Expression != "1h" {
				t.Expression = "1h"
				if err := s.UpdateTask(&t); err == nil {
					logger.Info().Msg("shopchecker: 价格扫描任务间隔已更新为 1h")
				}
			}
			return
		}
	}

	s.AddTask(&centralscheduler.Task{
		Name:       "shopchecker_price_scan",
		Type:       centralscheduler.TaskTypeInterval,
		Expression: "1h",
		Handler:    "shopchecker_price_scan",
		Status:     centralscheduler.TaskStatusActive,
		Timeout:    300,
	})
	logger.Info().Msg("shopchecker: 价格扫描任务已注册到 centralized scheduler（管理可见性，实际由独立调度器驱动）")
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

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
