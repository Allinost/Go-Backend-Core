package middleware

import (
	"context"
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

func AuthRequired(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			response.FailCode(c, errors.CodeUnauth)
			c.Abort()
			return
		}

		claims, err := authSvc.ValidateAccessToken(tokenStr)
		if err != nil {
			response.Fail(c, errors.New(errors.CodeUnauth, "token 无效或已过期"))
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("token_jti", claims.ID)
		c.Set("token_exp", claims.ExpiresAt.Time)
		c.Next()
	}
}

func AuthRequiredWithBlacklist(authSvc *auth.Service, bl auth.TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			response.FailCode(c, errors.CodeUnauth)
			c.Abort()
			return
		}

		claims, err := authSvc.ValidateAccessToken(tokenStr)
		if err != nil {
			response.Fail(c, errors.New(errors.CodeUnauth, "token 无效或已过期"))
			c.Abort()
			return
		}

		if bl != nil {
			revoked, err := bl.IsRevoked(context.Background(), claims.ID)
			if err == nil && revoked {
				response.Fail(c, errors.New(errors.CodeUnauth, "token 已被撤销"))
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}
