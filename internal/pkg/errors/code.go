package errors

import (
	"net/http"

	"github.com/Allinost/go-backend-core/pkg/apierror"
)

const (
	CodeSuccess    = apierror.CodeSuccess
	CodeUnknown    = apierror.CodeUnknown
	CodeSystemErr  = apierror.CodeSystemErr
	CodeParamErr   = apierror.CodeParamErr
	CodeNotFound   = apierror.CodeNotFound
	CodeUnauth     = apierror.CodeUnauth
	CodeForbidden  = apierror.CodeForbidden
	CodeRateLimit  = apierror.CodeRateLimit
	CodeConflict   = apierror.CodeConflict
	CodeTimeout    = apierror.CodeTimeout
	CodeBadGateway = apierror.CodeBadGateway
)

// 错误码 → HTTP 状态码映射
var codeHTTPStatus = map[int]int{
	CodeSuccess:    http.StatusOK,
	CodeUnknown:    http.StatusInternalServerError,
	CodeSystemErr:  http.StatusInternalServerError,
	CodeParamErr:   http.StatusBadRequest,
	CodeNotFound:   http.StatusNotFound,
	CodeUnauth:     http.StatusUnauthorized,
	CodeForbidden:  http.StatusForbidden,
	CodeRateLimit:  http.StatusTooManyRequests,
	CodeConflict:   http.StatusConflict,
	CodeTimeout:    http.StatusGatewayTimeout,
	CodeBadGateway: http.StatusBadGateway,
}

// CodeMsg 获取错误码对应的默认提示文案（委托到公开 API）
func CodeMsg(code int) string {
	return apierror.CodeMsg(code)
}

// CodeHTTPStatus 获取错误码对应的 HTTP 状态码
func CodeHTTPStatus(code int) int {
	if status, ok := codeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
