package s1

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
	return "s1"
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

func (m *Module) Ping(c *gin.Context) {
	response.Success(c, gin.H{"module": "s1"})
}

func (m *Module) Version(c *gin.Context) {
	response.Success(c, gin.H{
		"module":  "s1",
		"version": m.cfg.Server.Version,
	})
}

func (m *Module) Status(c *gin.Context) {
	response.Success(c, gin.H{
		"module":   "s1",
		"uptime":   time.Since(m.startTime).String(),
		"alive":    true,
	})
}

var _ interface{ Name() string; Init(*config.Config) error; Close() error; RegisterRoutes(*gin.RouterGroup) } = (*Module)(nil)
