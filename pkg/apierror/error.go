// Package apierror v1alpha1 公开错误类型和错误码。
// 提供给外部客户端解析 API 错误响应使用。在 v1 之前不保证向后兼容。
package apierror

import "fmt"

// AppError 应用层错误结构体，包含错误码、消息和详情
type AppError struct {
	Code    int    `json:"code"`             // 错误码
	Message string `json:"message"`          // 错误消息
	Detail  string `json:"detail,omitempty"` // 错误详情（可选）
}

// Error 实现 error 接口，返回 "[code] message" 或 "[code] message (detail)"
func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Is 判断两个 AppError 是否具有相同的错误码（用于 errors.Is）
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// New 创建一个 AppError，如果 message 为空则使用错误码对应的默认消息
func New(code int, message string) *AppError {
	if message == "" {
		message = CodeMsg(code)
	}
	return &AppError{Code: code, Message: message}
}

// WithDetail 为 AppError 设置错误详情，支持链式调用
func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}
