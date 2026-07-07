package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeMsg_KnownCodes(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{CodeSuccess, "success"},
		{CodeSystemErr, "系统内部错误"},
		{CodeParamErr, "请求参数错误"},
		{CodeNotFound, "资源不存在"},
		{CodeUnauth, "请先登录"},
		{CodeForbidden, "权限不足"},
		{CodeRateLimit, "请求过于频繁，请稍后再试"},
		{CodeConflict, "资源已存在"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, CodeMsg(tt.code))
	}
}

func TestCodeMsg_UnknownCode(t *testing.T) {
	assert.Equal(t, "未知错误", CodeMsg(99999))
}

func TestCodeHTTPStatus_KnownCodes(t *testing.T) {
	tests := []struct {
		code int
		want int
	}{
		{CodeSuccess, http.StatusOK},
		{CodeSystemErr, http.StatusInternalServerError},
		{CodeParamErr, http.StatusBadRequest},
		{CodeNotFound, http.StatusNotFound},
		{CodeUnauth, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeRateLimit, http.StatusTooManyRequests},
		{CodeConflict, http.StatusConflict},
		{CodeTimeout, http.StatusGatewayTimeout},
		{CodeBadGateway, http.StatusBadGateway},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, CodeHTTPStatus(tt.code))
	}
}

func TestCodeHTTPStatus_UnknownCode(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, CodeHTTPStatus(99999))
}
