package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORS_SetsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	CORS()(c)

	assert.Equal(t, "*", c.Writer.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS, PATCH", c.Writer.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, c.Writer.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORS_OptionsRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("OPTIONS", "/test", nil)

	CORS()(c)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, c.IsAborted())
}
