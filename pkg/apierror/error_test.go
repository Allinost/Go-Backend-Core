package apierror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_WithCode(t *testing.T) {
	err := New(CodeParamErr, "")
	assert.Equal(t, CodeParamErr, err.Code)
	assert.Equal(t, "请求参数错误", err.Message)
}

func TestNew_WithCustomMessage(t *testing.T) {
	err := New(CodeParamErr, "邮箱格式不正确")
	assert.Equal(t, CodeParamErr, err.Code)
	assert.Equal(t, "邮箱格式不正确", err.Message)
}

func TestError_String(t *testing.T) {
	err := New(CodeSuccess, "")
	assert.Equal(t, "[0] success", err.Error())
}

func TestError_WithDetail(t *testing.T) {
	err := New(CodeParamErr, "参数错误").WithDetail("email 字段不能为空")
	assert.Contains(t, err.Error(), "email 字段不能为空")
}

func TestIs_SameCode(t *testing.T) {
	err1 := New(CodeNotFound, "资源不存在")
	err2 := New(CodeNotFound, "用户不存在")
	assert.True(t, errors.Is(err1, err2))
}

func TestIs_DifferentCode(t *testing.T) {
	err1 := New(CodeNotFound, "")
	err2 := New(CodeForbidden, "")
	assert.False(t, errors.Is(err1, err2))
}

func TestIs_NilTarget(t *testing.T) {
	err := New(CodeSystemErr, "")
	assert.False(t, errors.Is(err, nil))
}

func TestCodeMsg_Default(t *testing.T) {
	assert.Equal(t, "未知错误", CodeMsg(99999))
}

func TestCodeMsg_Known(t *testing.T) {
	assert.Equal(t, "success", CodeMsg(CodeSuccess))
	assert.Equal(t, "请求参数错误", CodeMsg(CodeParamErr))
	assert.Equal(t, "权限不足", CodeMsg(CodeForbidden))
}
