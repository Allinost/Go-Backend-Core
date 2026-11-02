// Package integration 集成模块：统一动作网关、外盒事件与连接器注册表。
//
// n8n、OpenClaw、LangBot 等外部系统使用 API Key 调用本模块动作，
// API Key 的作用域决定其可读写的业务范围。
package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/action"
	"github.com/Allinost/go-backend-core/internal/services/connector"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/gin-gonic/gin"
)

// Module 集成模块，实现 modules.Module 接口
type Module struct {
	cfg        *config.Config
	outbox     *outbox.Service
	gateway    *action.Gateway
	connectors *connector.Registry
	h          *Handler
	authMW     gin.HandlerFunc
}

// Name 返回模块名称，路由挂载至 /api/v1/integration
func (m *Module) Name() string {
	return "integration"
}

// Init 初始化集成模块
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("integration: MySQL main 连接不可用")
	}

	m.outbox = outbox.NewService(outbox.NewMySQLStore(pool.DB))
	m.connectors = connector.NewRegistry()

	registry := action.NewRegistry()
	registry.MustRegister(action.Definition{
		Name:        "integration.ping",
		Description: "集成连通性测试，返回服务时间",
		Scopes:      []string{authz.ScopeIntegrationManage},
		Handler: func(_ context.Context, _ action.Request) (any, error) {
			return map[string]any{
				"ok":          true,
				"server_time": time.Now().UTC(),
			}, nil
		},
	})
	m.gateway = action.NewGateway(registry)

	authMod := modules.Get("auth")
	if authMod == nil {
		return fmt.Errorf("integration: auth 模块未注册")
	}
	am, ok := authMod.(*coreAuth.Module)
	if !ok {
		return fmt.Errorf("integration: auth 模块类型异常")
	}
	m.authMW = am.APIKeyOrJWTMiddleware()

	m.h = NewHandler(m.gateway, m.outbox, m.connectors, NewMySQLActionLogger(pool.DB))

	logger.Info().Msg("integration: 模块初始化完成")
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册集成路由
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(m.authMW)
	read := middleware.RequireScope(authz.ScopeOwnerRead)
	manage := middleware.RequireScope(authz.ScopeIntegrationManage)

	r.GET("/ping", read, m.h.Ping)
	r.GET("/actions", read, m.h.ListActions)
	r.POST("/actions/:name", m.h.InvokeAction)
	r.GET("/events", read, m.h.ListEvents)
	r.POST("/events", manage, m.h.EmitEvent)
	r.GET("/connectors", manage, m.h.ListConnectors)
}

// Outbox 暴露外盒服务，供业务模块写入事件
func (m *Module) Outbox() *outbox.Service {
	return m.outbox
}

// Actions 暴露动作注册表，供业务模块注册动作
func (m *Module) Actions() *action.Registry {
	if m.gateway == nil {
		return nil
	}
	return m.gateway.Registry()
}

// Connectors 暴露连接器注册表
func (m *Module) Connectors() *connector.Registry {
	return m.connectors
}
