// Package webhook 错误码和错误类型定义
package webhook

import "fmt"

const (
	// 配置相关错误 (40001-40099)
	ErrWebhookNotFound     = 40001
	ErrDeliveryLogNotFound = 40002
	ErrConfigNotFound      = 40003

	// 参数校验错误 (40100-40199)
	ErrInvalidType       = 40100
	ErrInvalidURL        = 40101
	ErrInvalidStatus     = 40102
	ErrInvalidWhitelist  = 40103
	ErrInvalidRateLimit  = 40104

	// 发送相关错误 (40200-40299)
	ErrSendFailed        = 40200
	ErrAllWebhooksFailed = 40201
	ErrNoWebhookTarget   = 40202
)

// errMsgs 错误码 → 中文消息映射
var errMsgs = map[int]string{
	ErrWebhookNotFound:     "Webhook 配置不存在",
	ErrDeliveryLogNotFound: "投递日志不存在",
	ErrConfigNotFound:      "配置不存在",
	ErrInvalidType:         "无效的 Webhook 类型",
	ErrInvalidURL:          "无效的 Webhook URL",
	ErrInvalidStatus:       "无效的状态值",
	ErrInvalidWhitelist:    "无效的白名单配置",
	ErrInvalidRateLimit:    "无效的频率限制配置",
	ErrSendFailed:          "发送失败",
	ErrAllWebhooksFailed:   "所有 Webhook 均发送失败",
	ErrNoWebhookTarget:     "没有找到可用的 Webhook 目标",
}

// WebhookError 业务错误，携带错误码和消息
type WebhookError struct {
	Code    int
	Message string
}

// Error 实现 error 接口
func (e *WebhookError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// NewWebhookError 创建业务错误
func NewWebhookError(code int, args ...interface{}) *WebhookError {
	msg := errMsgs[code]
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	return &WebhookError{Code: code, Message: msg}
}