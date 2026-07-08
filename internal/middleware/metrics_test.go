package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Allinost/go-backend-core/internal/services/monitor"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := monitor.NewMetricsRegistry("test", "http")

	router := gin.New()
	router.Use(Metrics(MetricsConfig{Registry: reg}))
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	handler := reg.Handler()
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `test_http_http_requests_total`)
	assert.Contains(t, body, `/api/test`)
}

func TestMetrics_MiddlewareErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := monitor.NewMetricsRegistry("test", "http")

	router := gin.New()
	router.Use(Metrics(MetricsConfig{Registry: reg}))
	router.GET("/not-found", func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "not found"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/not-found", nil)
	router.ServeHTTP(w, req)

	handler := reg.Handler()
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `status="404"`)
}
