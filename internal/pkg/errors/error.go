package errors

import "fmt"

// AppError 应用层业务错误，包含错误码、用户提示和内部调试信息
type AppError struct {
	Code    int    `json:"code"`               // 业务错误码
	Message string `json:"message"`            // 用户可见的提示信息
	Detail  string `json:"detail,omitempty"`   // 详细错误描述（开发环境可见）
	Err     error  `json:"-"`                  // 内部原始错误（不序列化到响应）
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New 创建业务错误，message 为空时使用错误码默认文案
func New(code int, message string) *AppError {
	if message == "" {
		message = CodeMsg(code)
	}
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装原始 error 为业务错误
func Wrap(code int, message string, err error) *AppError {
	if message == "" {
		message = CodeMsg(code)
	}
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WithDetail 为业务错误附加详细描述（开发环境调试用）
func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}
