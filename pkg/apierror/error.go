// Package apierror v1alpha1 公开错误类型和错误码。
// 提供给外部客户端解析 API 错误响应使用。在 v1 之前不保证向后兼容。
package apierror

import "fmt"

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func New(code int, message string) *AppError {
	if message == "" {
		message = CodeMsg(code)
	}
	return &AppError{Code: code, Message: message}
}

func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}
