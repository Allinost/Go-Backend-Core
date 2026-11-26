package middleware

import (
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// uploadMaxBytes 运行时可热更新的 multipart 请求体上限（字节）
var uploadMaxBytes atomic.Int64

func init() {
	SetUploadMaxMB(100)
}

// SetUploadMaxMB 设置 multipart 上传请求体上限（MB），由配置中心 security.upload_max_mb 驱动
func SetUploadMaxMB(mb int) {
	if mb <= 0 {
		return
	}
	uploadMaxBytes.Store(int64(mb) << 20)
}

// UploadLimit 对 multipart/form-data 请求限制请求体大小，超限返回 413
func UploadLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			c.Next()
			return
		}
		limit := uploadMaxBytes.Load()
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"code":    413,
				"message": "上传文件超过大小限制",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
