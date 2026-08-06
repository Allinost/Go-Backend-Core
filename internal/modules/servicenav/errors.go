// Package servicenav 错误码和错误类型定义

package servicenav

import "fmt"

const (
	// 服务条目错误 (41001-41099)
	ErrItemNotFound     = 41001
	ErrItemNameConflict = 41002
	ErrInvalidTypeRef   = 41003

	// 服务类型错误 (41100-41199)
	ErrTypeNotFound     = 41100
	ErrTypeNameConflict = 41101
	ErrTypeInUse        = 41102

	// 参数错误 (41200-41299)
	ErrParamNameRequired = 41200
)

// errMsgs 错误码 → 中文消息映射
var errMsgs = map[int]string{
	ErrItemNotFound:      "服务条目不存在",
	ErrItemNameConflict:  "服务条目名称已存在",
	ErrInvalidTypeRef:    "包含无效的服务类型",
	ErrTypeNotFound:      "服务类型不存在",
	ErrTypeNameConflict:  "服务类型名称已存在",
	ErrTypeInUse:         "该服务类型已被服务条目引用，无法删除",
	ErrParamNameRequired: "名称不能为空",
}

// NavError 业务错误，携带错误码和消息
type NavError struct {
	Code    int
	Message string
}

// Error 实现 error 接口
func (e *NavError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// NewNavError 创建业务错误
func NewNavError(code int, args ...interface{}) *NavError {
	msg := errMsgs[code]
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	return &NavError{Code: code, Message: msg}
}
