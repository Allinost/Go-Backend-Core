package modules

import (
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type testModule struct {
	name string
}

func (m *testModule) Name() string                      { return m.name }
func (m *testModule) Init(cfg *config.Config) error     { return nil }
func (m *testModule) Close() error                      { return nil }
func (m *testModule) RegisterRoutes(r *gin.RouterGroup) {}

func resetRegistry() {
	registry = make(map[string]Module)
}

func TestRegister_And_Get(t *testing.T) {
	resetRegistry()
	m := &testModule{name: "test"}
	Register(m)

	got := Get("test")
	assert.NotNil(t, got)
	assert.Equal(t, "test", got.Name())

	got = Get("non-existent")
	assert.Nil(t, got)
}

func TestRegister_Duplicate(t *testing.T) {
	resetRegistry()
	m1 := &testModule{name: "dup"}
	Register(m1)

	m2 := &testModule{name: "dup"}
	assert.Panics(t, func() { Register(m2) })
}

func TestInitAll(t *testing.T) {
	resetRegistry()
	Register(&testModule{name: "m1"})
	Register(&testModule{name: "m2"})

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
	}

	assert.NotPanics(t, func() { InitAll(cfg) })
	assert.NotNil(t, Get("m1"))
	assert.NotNil(t, Get("m2"))
}

func TestRegisterAllRoutes(t *testing.T) {
	resetRegistry()
	Register(&testModule{name: "m1"})
	Register(&testModule{name: "m2"})

	r := gin.New()
	assert.NotPanics(t, func() { RegisterAllRoutes(r) })
}

func TestRegisterRoutesTo(t *testing.T) {
	resetRegistry()
	Register(&testModule{name: "m1"})
	Register(&testModule{name: "m2"})

	r := gin.New()
	assert.NotPanics(t, func() { RegisterRoutesTo(r, "m1") })
}

func TestCloseAll(t *testing.T) {
	resetRegistry()
	closed := make([]string, 0)
	registry["a"] = &closeRecorder{name: "a", fn: func() { closed = append(closed, "a") }}
	registry["b"] = &closeRecorder{name: "b", fn: func() { closed = append(closed, "b") }}

	CloseAll()
	assert.ElementsMatch(t, []string{"a", "b"}, closed)
}

type closeRecorder struct {
	name string
	fn   func()
}

func (m *closeRecorder) Name() string                  { return m.name }
func (m *closeRecorder) Init(cfg *config.Config) error { return nil }
func (m *closeRecorder) Close() error {
	m.fn()
	return nil
}
func (m *closeRecorder) RegisterRoutes(r *gin.RouterGroup) {}
