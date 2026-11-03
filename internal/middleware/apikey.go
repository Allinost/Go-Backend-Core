package middleware

import (
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

// apiKeyFromRequest 读取 API Key。
// 优先 X-API-Key；同时兼容 MCP 客户端常用的 Authorization: Bearer gbk_xxx。
// Bearer JWT 不在这里处理，留给 APIKeyOrJWT 的 JWT 分支。
func apiKeyFromRequest(c *gin.Context) string {
	if rawKey := c.GetHeader("X-API-Key"); rawKey != "" {
		return rawKey
	}
	const bearerPrefix = "Bearer "
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, bearerPrefix) {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
		if auth.IsAPIKey(token) {
			return token
		}
	}
	return ""
}

func APIKeyAuth(svc *auth.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := apiKeyFromRequest(c)
		if rawKey == "" {
			response.FailCode(c, errors.CodeUnauth)
			c.Abort()
			return
		}

		key, err := svc.ValidateKey(rawKey)
		if err != nil {
			response.Fail(c, errors.New(errors.CodeUnauth, err.Error()))
			c.Abort()
			return
		}

		c.Set("user_id", key.UserID)
		c.Set("auth_method", "apikey")
		c.Set("api_key_id", key.ID)
		c.Set("api_key_scopes", key.Scopes)
		c.Next()
	}
}

func APIKeyOrJWT(svc *auth.ApiKeyService, authSvc *auth.Service, bl auth.TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rawKey := apiKeyFromRequest(c); rawKey != "" {
			key, err := svc.ValidateKey(rawKey)
			if err != nil {
				response.Fail(c, errors.New(errors.CodeUnauth, err.Error()))
				c.Abort()
				return
			}
			c.Set("user_id", key.UserID)
			c.Set("auth_method", "apikey")
			c.Set("api_key_id", key.ID)
			c.Set("api_key_scopes", key.Scopes)
			c.Next()
			return
		}

		AuthRequiredWithBlacklist(authSvc, bl)(c)
	}
}
