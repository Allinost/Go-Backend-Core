package s0

import (
	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Module S0 调试服务模块
type Module struct {
	cfg *config.Config
}

// Name 模块标识
func (m *Module) Name() string {
	return "s0"
}

// Init 模块初始化
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	return nil
}

// Close 关闭模块
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ping", m.Ping)
	r.GET("/health", m.Health)
	r.GET("/echo", m.Echo)
}

// Ping 存活检查
func (m *Module) Ping(c *gin.Context) {
	response.Success(c, gin.H{"message": "pong"})
}

// Health 各依赖服务健康状态聚合
func (m *Module) Health(c *gin.Context) {
	dbHealth := database.Health()

	overall := "ok"
	for _, h := range dbHealth {
		if h.Status == "error" {
			overall = "degraded"
			break
		}
	}

	response.Success(c, gin.H{
		"status":   overall,
		"version":  m.cfg.Server.Version,
		"database": dbHealth,
	})
}

// Echo 请求回显（调试用）
func (m *Module) Echo(c *gin.Context) {
	response.Success(c, gin.H{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"headers":   c.Request.Header,
		"client_ip": c.ClientIP(),
	})
}

// 编译期检查确保实现 modules.Module 接口
var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
