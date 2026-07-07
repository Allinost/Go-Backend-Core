package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_WithCode(t *testing.T) {
	err := New(CodeParamErr, "")
	assert.Equal(t, CodeParamErr, err.Code)
	assert.Equal(t, "请求参数错误", err.Message)
	assert.Nil(t, err.Err)
}

func TestNew_WithCustomMessage(t *testing.T) {
	err := New(CodeParamErr, "邮箱格式不正确")
	assert.Equal(t, CodeParamErr, err.Code)
	assert.Equal(t, "邮箱格式不正确", err.Message)
}

func TestWrap(t *testing.T) {
	original := errors.New("原始错误")
	err := Wrap(CodeSystemErr, "处理失败", original)
	assert.Equal(t, CodeSystemErr, err.Code)
	assert.Equal(t, "处理失败", err.Message)
	assert.Equal(t, original, err.Err)
	assert.True(t, errors.Is(err, original))
}

func TestWrap_EmptyMessage(t *testing.T) {
	original := errors.New("db error")
	err := Wrap(CodeSystemErr, "", original)
	assert.Equal(t, "系统内部错误", err.Message)
}

func TestWithDetail(t *testing.T) {
	err := New(CodeParamErr, "参数错误").WithDetail("email 字段不能为空")
	assert.Equal(t, "email 字段不能为空", err.Detail)
}

func TestError_WithErr(t *testing.T) {
	err := Wrap(CodeSystemErr, "系统错误", errors.New("connection refused"))
	assert.Contains(t, err.Error(), "[1000]")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestError_WithDetail(t *testing.T) {
	err := New(CodeParamErr, "参数错误").WithDetail("email 字段不能为空")
	assert.Contains(t, err.Error(), "email 字段不能为空")
}

func TestError_Plain(t *testing.T) {
	err := New(CodeSuccess, "")
	assert.Equal(t, "[0] success", err.Error())
}
