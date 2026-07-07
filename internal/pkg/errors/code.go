package errors

import "net/http"

// 通用错误码（业务模块从 10000 开始分配）
const (
	// 成功
	CodeSuccess    = 0     // 请求成功
	CodeUnknown    = -1    // 未知错误
	CodeSystemErr  = 1000  // 系统内部错误
	CodeParamErr   = 1001  // 请求参数错误
	CodeNotFound   = 1002  // 资源不存在
	CodeUnauth     = 1003  // 未授权（需登录）
	CodeForbidden  = 1004  // 无权限
	CodeRateLimit  = 1005  // 请求频率超限
	CodeConflict   = 1006  // 资源冲突（如重复创建）
	CodeTimeout    = 1007  // 请求超时
	CodeBadGateway = 1008  // 上游服务不可达
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

// 错误码 → 默认提示文案
var codeMessages = map[int]string{
	CodeSuccess:    "success",
	CodeUnknown:    "系统繁忙，请稍后再试",
	CodeSystemErr:  "系统内部错误",
	CodeParamErr:   "请求参数错误",
	CodeNotFound:   "资源不存在",
	CodeUnauth:     "请先登录",
	CodeForbidden:  "权限不足",
	CodeRateLimit:  "请求过于频繁，请稍后再试",
	CodeConflict:   "资源已存在",
	CodeTimeout:    "请求超时",
	CodeBadGateway: "上游服务不可达",
}

// CodeMsg 获取错误码对应的默认提示文案
func CodeMsg(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// CodeHTTPStatus 获取错误码对应的 HTTP 状态码
func CodeHTTPStatus(code int) int {
	if status, ok := codeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
