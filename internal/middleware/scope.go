package middleware

import (
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	ScopeOwnerRead         = authz.ScopeOwnerRead
	ScopeOwnerWrite        = authz.ScopeOwnerWrite
	ScopeCustomerRead      = authz.ScopeCustomerRead
	ScopeCustomerOrderReq  = authz.ScopeCustomerOrderReq
	ScopeIntegrationManage = authz.ScopeIntegrationManage
)

// HasScope 判断 API Key 的作用域是否满足任一 required。
func HasScope(raw string, required ...string) bool {
	return authz.Has(raw, required...)
}

// RequireScope 限制 API Key 必须拥有指定作用域之一。
// JWT 登录用户视为 owner 控制台，不受 API Key 作用域限制。
func RequireScope(required ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("auth_method") != "apikey" {
			c.Next()
			return
		}
		raw := c.GetString("api_key_scopes")
		if !authz.Has(raw, required...) {
			response.Fail(c, appErrors.New(appErrors.CodeForbidden,
				"API Key 缺少权限: "+strings.Join(required, " 或 ")))
			c.Abort()
			return
		}
		c.Next()
	}
}
