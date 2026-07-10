package eventbus

import (
	"context"
	"time"
)

// Event 事件消息体，包含唯一标识、主题、来源、时间戳和载荷
type Event struct {
	ID        string         `json:"id"`        // 事件唯一标识
	Topic     string         `json:"topic"`     // 事件主题
	Source    string         `json:"source"`    // 事件来源
	Timestamp time.Time      `json:"timestamp"` // 事件时间戳
	Payload   map[string]any `json:"payload"`   // 事件载荷
}

// Subscription 订阅信息，包含唯一 ID、主题和事件处理函数
type Subscription struct {
	ID      string       // 订阅唯一标识
	Topic   string       // 订阅主题
	Handler EventHandler // 事件处理函数
}

// EventHandler 事件处理函数类型，接收上下文和事件，返回错误
type EventHandler func(ctx context.Context, event Event) error
