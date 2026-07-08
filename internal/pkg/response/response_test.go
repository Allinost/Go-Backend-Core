package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, gin.H{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.Equal(t, "value", resp.Data.(map[string]interface{})["key"])
}

func TestSuccess_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Nil(t, resp.Data)
}

func TestFailCode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	FailCode(c, appErr.CodeNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, appErr.CodeNotFound, resp.Code)
	assert.Equal(t, "资源不存在", resp.Message)
}

func TestSuccessWithMsg(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SuccessWithMsg(c, gin.H{"key": "value"}, "操作成功")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "操作成功", resp.Message)
	assert.Equal(t, "value", resp.Data.(map[string]interface{})["key"])
}

func TestFail_WithAppError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := appErr.New(appErr.CodeForbidden, "自定义无权限")
	Fail(c, err)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp Response
	err2 := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err2)
	assert.Equal(t, appErr.CodeForbidden, resp.Code)
	assert.Equal(t, "自定义无权限", resp.Message)
}

func TestParamErr(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ParamErr(c, "email 不能为空")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, appErr.CodeParamErr, resp.Code)
	assert.Equal(t, "请求参数错误", resp.Message)
}
