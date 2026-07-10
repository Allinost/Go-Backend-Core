package s2

import (
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Module struct {
	cfg       *config.Config
	startTime time.Time
}

func (m *Module) Name() string {
	return "s2"
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	m.startTime = time.Now()
	return nil
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ping", m.Ping)
	r.GET("/version", m.Version)
	r.GET("/status", m.Status)
}

// Ping 存活检查
// @Summary      s2 存活检查
// @Description  返回 pong 表示 s2 模块运行正常
// @Tags         s2-调试
// @Success      200  {object}  response.Response{data=object{module=string}}
// @Router       /s2/ping [get]
func (m *Module) Ping(c *gin.Context) {
	response.Success(c, gin.H{"module": "s2"})
}

// Version 版本信息
// @Summary      s2 版本信息
// @Description  返回 s2 模块版本号
// @Tags         s2-调试
// @Success      200 {object} response.Response{data=object{module=string,version=string}}
// @Router       /s2/version [get]
func (m *Module) Version(c *gin.Context) {
	response.Success(c, gin.H{
		"module":  "s2",
		"version": m.cfg.Server.Version,
	})
}

// Status 运行状态
// @Summary      s2 运行状态
// @Description  返回 s2 模块运行时长和健康状态
// @Tags         s2-调试
// @Success      200  {object}  response.Response{data=object{module=string,uptime=string,alive=bool}}
// @Router       /s2/status [get]
func (m *Module) Status(c *gin.Context) {
	response.Success(c, gin.H{
		"module": "s2",
		"uptime": time.Since(m.startTime).String(),
		"alive":  true,
	})
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
