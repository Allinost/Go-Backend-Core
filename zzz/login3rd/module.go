// Package login3rd 第三方应用登录管理模块
// 管理京东、淘宝、拼多多、1688、TJUPT 等平台登录信息与会话
// 提供自动登录、远程浏览器登录、session 验证/刷新、webhook 通知等功能
package login3rd

import (
	"context"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/scheduler"
	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/gin-gonic/gin"
)

// Module Login3rd 模块
type Module struct {
	cfg   *config.Config
	h     *Handler
	svc   *Service
	repo  *Repository
	whSvc *webhook.Service
	sch   *SessionChecker
}

// Name 返回模块名称，路由挂载至 /api/v1/login3rd
func (m *Module) Name() string {
	return "login3rd"
}

// Init 初始化模块
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("login3rd: MySQL 连接不可用")
	}

	m.repo = NewRepository(pool.DB)

	whMod := modules.Get("webhook")
	if whMod == nil {
		return fmt.Errorf("login3rd: webhook 模块未注册")
	}
	m.whSvc = whMod.(*webhook.Module).GetService()

	m.svc = NewService(m.repo, m.whSvc)

	// 注册 Provider（七鲜/京东移动版使用京东移动版登录，复用京东 Provider 做 session 验证/刷新）
	jdProv := provider.NewJDCollector()
	m.svc.RegisterProvider(jdProv)
	m.svc.RegisterProviderAs("qixian", jdProv)
	m.svc.RegisterProviderAs("jdm", jdProv)

	// 平台 seed（表由 migrations 管理，未应用迁移时仅告警，不阻断启动）
	if err := m.repo.SeedPlatforms(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("login3rd: 平台 seed 未就绪（请先应用 migrations）")
	}

	m.h = NewHandler(m.svc)

	// 设置全局服务引用（供 centralized scheduler handler 使用）
	sessionCheckSvc = m.svc

	// 在 centralized scheduler 中注册 session 检查任务
	m.registerSessionCheckTask()

	return nil
}

// Close 关闭模块
func (m *Module) Close() error {
	if m.sch != nil {
		m.sch.Stop()
	}
	return nil
}

// GetService 暴露 Service 实例
func (m *Module) GetService() *Service {
	return m.svc
}

// registerSessionCheckTask 在 centralized scheduler 中注册 24h 间隔的 session 检查任务
func (m *Module) registerSessionCheckTask() {
	schedMod := modules.Get("scheduler")
	if schedMod == nil {
		logger.Warn().Msg("login3rd: scheduler 模块未注册，session 检查任务未添加（非致命，可通过 API 手动触发）")
		return
	}
	s := schedMod.(*scheduler.Module).GetScheduler()
	if s == nil {
		return
	}

	tasks, err := s.ListTasks()
	if err != nil {
		logger.Warn().Err(err).Msg("login3rd: 获取 scheduler 任务列表失败")
		return
	}
	for _, t := range tasks {
		if t.Name == "login3rd_session_check" {
			if t.Expression != "24h" {
				t.Expression = "24h"
				if err := s.UpdateTask(&t); err == nil {
					logger.Info().Msg("login3rd: session 检查任务间隔已更新为 24h")
				}
			}
			return
		}
	}

	if err := s.AddTask(&scheduler.Task{
		Name:       "login3rd_session_check",
		Type:       scheduler.TaskTypeInterval,
		Expression: "24h",
		Handler:    "login3rd_session_check",
		Status:     scheduler.TaskStatusActive,
		Timeout:    600,
	}); err != nil {
		logger.Warn().Err(err).Msg("login3rd: 添加 session 检查任务到 scheduler 失败")
		return
	}
	logger.Info().Msg("login3rd: session 检查任务已注册到 centralized scheduler（间隔 24h）")
}

// RegisterRoutes 注册路由到 /api/v1/login3rd/*
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// 平台
	r.GET("/platforms", m.h.ListPlatforms)

	// 账号 CRUD
	r.POST("/accounts", m.h.CreateAccount)
	r.GET("/accounts", m.h.ListAccounts)
	r.GET("/accounts/:id", m.h.GetAccount)
	r.PUT("/accounts/:id", m.h.UpdateAccount)
	r.DELETE("/accounts/:id", m.h.DeleteAccount)

	// 登录操作
	r.POST("/accounts/:id/login", m.h.AutoLogin)
	r.POST("/accounts/:id/remote-login", m.h.CreateRemoteSession)
	r.GET("/accounts/:id/status", m.h.GetAccountStatus)
	r.POST("/accounts/:id/refresh", m.h.RefreshSession)
	r.POST("/accounts/:id/verify", m.h.VerifySession)
	r.POST("/accounts/:id/logout", m.h.Logout)

	// 远程浏览器
	r.GET("/remote/session/:token", m.h.RemoteSessionPage)
	r.GET("/remote/session/:token/status", m.h.RemoteSessionStatus)
	r.GET("/remote/session/:token/ws", m.h.RemoteSessionWS)
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
