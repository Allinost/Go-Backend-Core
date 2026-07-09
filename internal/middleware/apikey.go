package middleware

import (
	"github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

func APIKeyAuth(svc *auth.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := c.GetHeader("X-API-Key")
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
		if c.GetHeader("X-API-Key") != "" {
			rawKey := c.GetHeader("X-API-Key")
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
