package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

func TestAPIKeyOrJWTBearerAPIKey(t *testing.T) {
	store := auth.NewInMemoryApiKeyStore()
	svc := auth.NewApiKeyService(store)
	key, err := svc.GenerateKey("mcp", 7, "owner:read", nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", APIKeyOrJWT(svc, nil, nil), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.GetUint("user_id"),
			"scopes":  c.GetString("api_key_scopes"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+key.RawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body == "" || body == `{"scopes":"","user_id":0}` {
		t.Fatalf("API Key 未写入上下文: %s", body)
	}
}

func TestAPIKeyAuthXAPIKey(t *testing.T) {
	store := auth.NewInMemoryApiKeyStore()
	svc := auth.NewApiKeyService(store)
	key, err := svc.GenerateKey("x-api-key", 8, "customer:read", nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", APIKeyAuth(svc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.GetUint("user_id"),
			"scopes":  c.GetString("api_key_scopes"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", key.RawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAPIKeyAuthRejectsJWTAsBearer(t *testing.T) {
	store := auth.NewInMemoryApiKeyStore()
	svc := auth.NewApiKeyService(store)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", APIKeyAuth(svc), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-gbk-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}
