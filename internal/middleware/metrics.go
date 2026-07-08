package middleware

import (
	"strconv"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/monitor"
	"github.com/gin-gonic/gin"
)

type MetricsConfig struct {
	Registry *monitor.MetricsRegistry
}

func Metrics(cfg MetricsConfig) gin.HandlerFunc {
	reqTotal := cfg.Registry.NewCounter("http_requests_total", "HTTP 请求总数", "method", "path", "status")
	reqDuration := cfg.Registry.NewHistogram("http_request_duration_seconds", "HTTP 请求耗时",
		[]float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		"method", "path", "status")

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		latency := time.Since(start).Seconds()

		reqTotal.WithLabelValues(method, path, status).Inc()
		reqDuration.WithLabelValues(method, path, status).Observe(latency)
	}
}
