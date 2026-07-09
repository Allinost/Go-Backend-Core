package s0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestModule(t *testing.T) (*Module, *gin.Engine) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:    "test",
			Version: "v0.0.0",
			Port:    29090,
			Mode:    "test",
		},
	}

	m := &Module{}
	err := m.Init(cfg)
	assert.NoError(t, err)

	r := gin.New()
	m.RegisterRoutes(r.Group("/api/v1/s0"))
	return m, r
}

func TestModule_Ping(t *testing.T) {
	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestModule_Health(t *testing.T) {
	err := database.InitAll(&config.Config{})
	assert.NoError(t, err)
	t.Cleanup(database.CloseAll)

	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, "v0.0.0", data["version"])
}

func TestModule_Metrics(t *testing.T) {
	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "go_backend_core_s0_goroutines")
}

func TestModule_Echo(t *testing.T) {
	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/echo?foo=bar", nil)
	req.Header.Set("X-Custom", "test-value")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "GET", data["method"])
	assert.Equal(t, "/api/v1/s0/echo", data["path"])
	assert.Equal(t, "foo=bar", data["query"])
}

func TestModule_Name(t *testing.T) {
	m := &Module{}
	assert.Equal(t, "s0", m.Name())
}

func TestModule_Info(t *testing.T) {
	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/info", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "test", data["name"])
	assert.Equal(t, "v0.0.0", data["version"])
	assert.Equal(t, "test", data["mode"])
	assert.Contains(t, data, "uptime")
	assert.Contains(t, data, "start_time")
	assert.Contains(t, data, "go_version")
	assert.Contains(t, data, "goroutines")
	assert.Contains(t, data, "modules")
}

func TestModule_Modules(t *testing.T) {
	_, r := newTestModule(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/s0/modules", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Contains(t, data, "count")
	assert.Contains(t, data, "modules")
}

func TestModule_Close(t *testing.T) {
	m := &Module{}
	assert.NoError(t, m.Close())
}
