// Package login3rd 第三方应用登录管理模块
// 管理京东、淘宝、拼多多、1688、TJUPT 等平台登录信息与会话
// 提供自动登录、远程浏览器登录、session 验证/刷新、webhook 通知等功能
package login3rd

import (
	"context"
	"database/sql"
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

	// 注册 Provider
	m.svc.RegisterProvider(provider.NewJDCollector())

	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("login3rd: 初始化数据表失败: %w", err)
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

// registerSessionCheckTask 在 centralized scheduler 中注册 1h 间隔的 session 检查任务
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
			logger.Info().Msg("login3rd: session 检查任务已存在 scheduler 中")
			return
		}
	}

	if err := s.AddTask(&scheduler.Task{
		Name:       "login3rd_session_check",
		Type:       scheduler.TaskTypeInterval,
		Expression: "1h",
		Handler:    "login3rd_session_check",
		Status:     scheduler.TaskStatusActive,
		Timeout:    600,
	}); err != nil {
		logger.Warn().Err(err).Msg("login3rd: 添加 session 检查任务到 scheduler 失败")
		return
	}
	logger.Info().Msg("login3rd: session 检查任务已注册到 centralized scheduler（间隔 1h）")
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

// initTables 自动建表 + seed 数据
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS login3rd_platforms (
			id            VARCHAR(64)  PRIMARY KEY,
			name          VARCHAR(128) NOT NULL,
			category      VARCHAR(32)  NOT NULL,
			login_type    VARCHAR(32)  NOT NULL DEFAULT 'browser',
			has_auto      BOOLEAN      NOT NULL DEFAULT FALSE,
			login_url     TEXT,
			home_url      TEXT,
			form_config   JSON,
			icon_url      TEXT,
			created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS login3rd_accounts (
			id                  BIGINT       AUTO_INCREMENT PRIMARY KEY,
			platform_id         VARCHAR(64)  NOT NULL,
			label               VARCHAR(128) NOT NULL,
			username            VARCHAR(255),
			encrypted_password  TEXT,
			encrypted_token     TEXT,
			webhook_ids         JSON,
			session_data        JSON,
			login_method        VARCHAR(32),
			session_expires_at  TIMESTAMP    NULL,
			last_verified_at    TIMESTAMP    NULL,
			verified_status     VARCHAR(32)  DEFAULT 'unknown',
			status              VARCHAR(32)  NOT NULL DEFAULT 'active',
			last_login_at       TIMESTAMP    NULL,
			last_error          TEXT,
			created_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (platform_id) REFERENCES login3rd_platforms(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}

	// Seed 平台数据
	repo := NewRepository(db)
	if err := repo.SeedPlatforms(ctx); err != nil {
		return err
	}

	logger.Info().Msg("login3rd: 数据表初始化完成")
	return nil
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
