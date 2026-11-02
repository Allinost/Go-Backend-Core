package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireScopeHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		authMethod string
		scopes     string
		wantStatus int
		wantCalled bool
	}{
		{"JWT 用户不受限制", "jwt", "", http.StatusOK, true},
		{"API Key 有权限", "apikey", ScopeOwnerRead, http.StatusOK, true},
		{"API Key 缺权限", "apikey", ScopeCustomerRead, http.StatusForbidden, false},
		{"API Key 通配", "apikey", "*:*", http.StatusOK, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			r := gin.New()
			r.GET("/x", func(ctx *gin.Context) {
				ctx.Set("auth_method", c.authMethod)
				ctx.Set("api_key_scopes", c.scopes)
			}, RequireScope(ScopeOwnerRead), func(ctx *gin.Context) {
				called = true
				ctx.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
			if w.Code != c.wantStatus {
				t.Fatalf("status = %d，期望 %d，body=%s", w.Code, c.wantStatus, w.Body.String())
			}
			if called != c.wantCalled {
				t.Fatalf("handler called = %v，期望 %v", called, c.wantCalled)
			}
		})
	}
}
