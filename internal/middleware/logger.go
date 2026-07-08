package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 自动注入 TraceID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = logger.NewTraceID()
		}
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info().
			Str("trace_id", traceID).
			Int("status", status).
			Str("method", method).
			Str("path", path).
			Dur("latency", latency).
			Int("size", c.Writer.Size()).
			Msg("request")
	}
}
