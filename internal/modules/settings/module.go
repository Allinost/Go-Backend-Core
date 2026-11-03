// Package settings 应用设置模块：管理员可在前端配置公开地址、前端参数与外部工具地址，
// 连接类敏感配置（数据库/Redis/JWT）仍由 .env 管理，本模块只读展示。
package settings

import (
	"fmt"
	"os"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	svcAuth "github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

// Module 设置模块，实现 modules.Module 接口
type Module struct {
	cfg    *config.Config
	store  *Store
	h      *Handler
	authMW gin.HandlerFunc
	rbac   *svcAuth.RBACService
}

// Name 返回模块名称，用于路由前缀 /api/v1/settings/*
func (m *Module) Name() string {
	return "settings"
}

// Init 初始化设置模块：读取设置文件、装配管理员鉴权
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	m.store = NewStore(settingsFilePath())

	authMod := modules.Get("auth")
	if authMod == nil {
		return fmt.Errorf("settings: auth 模块未注册")
	}
	am, ok := authMod.(*coreAuth.Module)
	if !ok {
		return fmt.Errorf("settings: auth 模块类型异常")
	}
	m.authMW = am.AuthMiddleware()
	m.rbac = am.RBAC()

	m.h = NewHandler(m.store, cfg)
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册设置路由（仅管理员：resource=* action=*）
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(m.authMW)
	admin := middleware.RequirePermission(m.rbac, "*", "*")
	r.GET("", admin, m.h.Get)
	r.PUT("", admin, m.h.Update)
}

// settingsFilePath 设置文件路径，可用 APP_SETTINGS_FILE 覆盖
func settingsFilePath() string {
	if path := os.Getenv("APP_SETTINGS_FILE"); path != "" {
		return path
	}
	return "data/settings.json"
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
