package migrate

import (
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestModuleName(t *testing.T) {
	m := &Module{}
	assert.Equal(t, "migrate", m.Name())
}

func TestModuleClose(t *testing.T) {
	m := &Module{}
	assert.NoError(t, m.Close())
}

func TestModuleInit_NoDB(t *testing.T) {
	m := &Module{}
	err := m.Init(&config.Config{})
	assert.NoError(t, err)
	assert.Nil(t, m.adapter)
}

func TestModuleRegisterRoutes_NoPanic(t *testing.T) {
	m := &Module{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	assert.NotPanics(t, func() {
		m.RegisterRoutes(r.Group("/api/v1/migrate"))
	})
}

func TestModuleInterface(t *testing.T) {
	m := &Module{}
	assert.Implements(t, (*interface {
		Name() string
		Init(*config.Config) error
		Close() error
		RegisterRoutes(*gin.RouterGroup)
	})(nil), m)
}
