package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUploadLimitRejectsOversizeMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetUploadMaxMB(1)
	defer SetUploadMaxMB(100)

	r := gin.New()
	r.Use(UploadLimit())
	r.POST("/up", func(c *gin.Context) { c.Status(http.StatusOK) })

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "big.bin")
	fw.Write(make([]byte, 2<<20))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/up", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("超限应返回 413，实际 %d", w.Code)
	}
}

func TestUploadLimitPassesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetUploadMaxMB(1)
	defer SetUploadMaxMB(100)

	r := gin.New()
	r.Use(UploadLimit())
	r.POST("/j", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/j", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("JSON 请求不应受上传限制，实际 %d", w.Code)
	}
}

func TestRateLimitSetMaxRequestsHotUpdate(t *testing.T) {
	rl := NewRateLimit(1, 60e9)
	if !rl.Allow("ip") {
		t.Fatal("首个请求应放行")
	}
	if rl.Allow("ip") {
		t.Fatal("超过上限应拒绝")
	}
	rl.SetMaxRequests(5)
	if !rl.Allow("ip") {
		t.Fatal("热更新上限后应放行")
	}
}
