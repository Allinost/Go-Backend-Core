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

	dbHealth, ok := data["database"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, dbHealth)
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

func TestModule_Close(t *testing.T) {
	m := &Module{}
	assert.NoError(t, m.Close())
}
