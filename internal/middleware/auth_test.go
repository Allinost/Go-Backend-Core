package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func testAuthService(t *testing.T) *auth.Service {
	return auth.NewService(auth.Config{
		Secret:        "test-secret",
		AccessExpire:  time.Hour,
		RefreshExpire: 24 * time.Hour,
	})
}

func setupRouter(svc *auth.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthRequired(svc), func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	return r
}

func setupRouterWithBlacklist(svc *auth.Service, bl auth.TokenBlacklist) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthRequiredWithBlacklist(svc, bl), func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	return r
}

func TestAuthRequired_MissingHeader(t *testing.T) {
	svc := testAuthService(t)
	r := setupRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_EmptyHeader(t *testing.T) {
	svc := testAuthService(t)
	r := setupRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	svc := testAuthService(t)
	r := setupRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_ValidToken(t *testing.T) {
	svc := testAuthService(t)
	r := setupRouter(svc)

	pair, err := svc.GenerateTokenPair(1, "alice")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthRequiredWithBlacklist_Revoked(t *testing.T) {
	svc := testAuthService(t)
	bl := auth.NewInMemoryBlacklist()

	pair, _ := svc.GenerateTokenPair(1, "alice")
	claims, _ := svc.ValidateAccessToken(pair.AccessToken)
	_ = bl.Revoke(context.Background(), claims.ID, claims.ExpiresAt.Time)

	r := setupRouterWithBlacklist(svc, bl)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequiredWithBlacklist_NotRevoked(t *testing.T) {
	svc := testAuthService(t)
	bl := auth.NewInMemoryBlacklist()

	pair, _ := svc.GenerateTokenPair(1, "alice")

	r := setupRouterWithBlacklist(svc, bl)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	tests := []struct {
		header string
		want   string
	}{
		{"Bearer mytoken", "mytoken"},
		{"bearer mytoken", "mytoken"},
		{"BEARER mytoken", "mytoken"},
		{"", ""},
		{"Basic xxx", ""},
		{"Bearer", ""},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", tt.header)
		r.ServeHTTP(w, req)
	}
}
