package modules

import (
	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/gin-gonic/gin"
)

// Module 模块接口，所有业务模块必须实现
type Module interface {
	Name() string                          // 模块唯一标识（用于路由前缀 /api/v1/{name}）
	RegisterRoutes(r *gin.RouterGroup)     // 注册模块路由
	Init(cfg *config.Config) error         // 模块初始化
	Close() error                          // 模块关闭（资源清理）
}

var registry = make(map[string]Module) // 全局模块注册表

// Register 注册模块到全局注册表
func Register(m Module) {
	name := m.Name()
	if _, exists := registry[name]; exists {
		panic("模块 " + name + " 重复注册")
	}
	registry[name] = m
}

// InitAll 初始化所有已注册模块（调用 Init），不自动挂载路由
func InitAll(cfg *config.Config) {
	for _, m := range registry {
		if err := m.Init(cfg); err != nil {
			panic("模块 " + m.Name() + " 初始化失败: " + err.Error())
		}
	}
}

// RegisterAllRoutes 将所有模块路由挂载到指定引擎
func RegisterAllRoutes(r *gin.Engine) {
	for _, m := range registry {
		m.RegisterRoutes(r.Group("/api/v1/" + m.Name()))
	}
}

// RegisterRoutesTo 将指定模块挂载到指定引擎
func RegisterRoutesTo(r *gin.Engine, names ...string) {
	for _, name := range names {
		if m, ok := registry[name]; ok {
			m.RegisterRoutes(r.Group("/api/v1/" + name))
		}
	}
}

// CloseAll 关闭所有模块（服务退出时调用）
func CloseAll() {
	for _, m := range registry {
		_ = m.Close()
	}
}

// Get 按名称获取已注册模块
func Get(name string) Module {
	return registry[name]
}

// List 返回所有已注册模块的名称列表
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
