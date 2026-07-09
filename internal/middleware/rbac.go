package middleware

import (
	"github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

func RequirePermission(rbac *auth.RBACService, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.FailCode(c, errors.CodeUnauth)
			c.Abort()
			return
		}

		uid, ok := userID.(uint)
		if !ok {
			response.FailCode(c, errors.CodeUnauth)
			c.Abort()
			return
		}

		if !rbac.HasPermission(uid, resource, action) {
			response.FailCode(c, errors.CodeForbidden)
			c.Abort()
			return
		}

		c.Set("permission_resource", resource)
		c.Set("permission_action", action)
		c.Next()
	}
}
