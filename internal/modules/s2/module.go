package s2

import (
	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Module S2 业务模块占位
type Module struct{}

func (m *Module) Name() string {
	return "s2"
}

func (m *Module) Init(cfg *config.Config) error {
	return nil
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ping", func(c *gin.Context) {
		response.Success(c, gin.H{"module": "s2"})
	})
}

var _ interface{ Name() string; Init(*config.Config) error; Close() error; RegisterRoutes(*gin.RouterGroup) } = (*Module)(nil)
