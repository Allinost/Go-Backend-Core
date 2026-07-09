package s2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestModule(t *testing.T) *Module {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:    "test",
			Version: "v0.0.0",
			Mode:    "test",
		},
	}
	m := &Module{}
	err := m.Init(cfg)
	assert.NoError(t, err)
	return m
}

func setupRouter(m *Module) *gin.Engine {
	r := gin.New()
	m.RegisterRoutes(r.Group("/api/v1/s2"))
	return r
}

func TestS2Module_Name(t *testing.T) {
	m := &Module{}
	assert.Equal(t, "s2", m.Name())
}

func TestS2Module_InitClose(t *testing.T) {
	m := newTestModule(t)
	assert.NoError(t, m.Close())
}

func TestS2Module_Ping(t *testing.T) {
	m := newTestModule(t)
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s2/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "s2", data["module"])
}

func TestS2Module_Version(t *testing.T) {
	m := newTestModule(t)
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s2/version", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "s2", data["module"])
	assert.Equal(t, "v0.0.0", data["version"])
}

func TestS2Module_Status(t *testing.T) {
	m := newTestModule(t)
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s2/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "s2", data["module"])
	assert.Contains(t, data, "uptime")
	assert.Equal(t, true, data["alive"])
}
