package notification

import (
	"context"
	"fmt"
	"sync"
)

// Channel 通知渠道类型
type Channel string

const (
	ChannelEmail   Channel = "email"   // 邮件
	ChannelSMS     Channel = "sms"     // 短信
	ChannelPush    Channel = "push"    // 推送
	ChannelWebhook Channel = "webhook" // Webhook
)

// Priority 通知优先级
type Priority int

const (
	PriorityLow    Priority = 0 // 低
	PriorityNormal Priority = 1 // 普通
	PriorityHigh   Priority = 2 // 高
	PriorityUrgent Priority = 3 // 紧急
)

// Message 通知消息体
type Message struct {
	ID       string            `json:"id"`                  // 消息唯一标识
	Channel  Channel           `json:"channel"`             // 通知渠道
	To       []string          `json:"to"`                  // 接收人列表
	Subject  string            `json:"subject,omitempty"`   // 主题
	Body     string            `json:"body"`                // 消息正文（纯文本）
	HTMLBody string            `json:"html_body,omitempty"` // HTML 正文
	Priority Priority          `json:"priority"`            // 优先级
	Metadata map[string]string `json:"metadata,omitempty"`  // 附加元数据
}

// Result 通知发送结果
type Result struct {
	MessageID string  `json:"message_id"`      // 消息 ID
	Channel   Channel `json:"channel"`         // 发送渠道
	Success   bool    `json:"success"`         // 是否成功
	Error     string  `json:"error,omitempty"` // 错误信息
}

// Sender 通知发送器接口，每种渠道独立实现
type Sender interface {
	Name() string                                           // 发送器名称
	Channel() Channel                                       // 通知渠道
	Send(ctx context.Context, msg Message) (*Result, error) // 发送消息
	Close() error                                           // 关闭发送器
}

// Service 通知服务，支持多发送器注册和回退机制
type Service struct {
	mu      sync.RWMutex
	senders map[Channel][]Sender // 渠道到发送器列表的映射
}

func NewService() *Service {
	return &Service{
		senders: make(map[Channel][]Sender),
	}
}

// RegisterSender 注册一个发送器到指定渠道
func (s *Service) RegisterSender(sender Sender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.senders[sender.Channel()] = append(s.senders[sender.Channel()], sender)
}

// Send 发送通知，按注册顺序尝试，成功立即返回，全部失败则返回错误
func (s *Service) Send(ctx context.Context, msg Message) (*Result, error) {
	s.mu.RLock()
	senders, ok := s.senders[msg.Channel]
	s.mu.RUnlock()

	if !ok || len(senders) == 0 {
		return nil, fmt.Errorf("notification: no sender for channel %s", msg.Channel)
	}

	var lastErr error
	for _, sender := range senders {
		result, err := sender.Send(ctx, msg)
		if err != nil {
			lastErr = err
			continue
		}
		if result.Success {
			return result, nil
		}
		lastErr = fmt.Errorf("sender %s failed: %s", sender.Name(), result.Error)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("all senders returned unsuccessful results")
	}
	return nil, fmt.Errorf("notification: all senders failed: %w", lastErr)
}

// SendToAll 向该渠道所有发送器发送，返回每个发送器的结果
func (s *Service) SendToAll(ctx context.Context, msg Message) []*Result {
	s.mu.RLock()
	senders, ok := s.senders[msg.Channel]
	s.mu.RUnlock()

	if !ok {
		return []*Result{{MessageID: msg.ID, Channel: msg.Channel, Success: false, Error: "no sender"}}
	}

	var results []*Result
	for _, sender := range senders {
		result, err := sender.Send(ctx, msg)
		if err != nil {
			results = append(results, &Result{
				MessageID: msg.ID, Channel: msg.Channel,
				Success: false, Error: err.Error(),
			})
		} else {
			results = append(results, result)
		}
	}
	return results
}

// Close 关闭所有已注册的发送器
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, senders := range s.senders {
		for _, sender := range senders {
			_ = sender.Close()
		}
	}
}

// LogSender 日志发送器，仅记录不真实发送，用于开发和测试
type LogSender struct {
	name string // 发送器名称
}

func NewLogSender(name string) *LogSender {
	return &LogSender{name: name}
}

func (l *LogSender) Name() string { return l.name }

func (l *LogSender) Channel() Channel { return ChannelEmail }

// Send 模拟发送，直接返回成功
func (l *LogSender) Send(ctx context.Context, msg Message) (*Result, error) {
	return &Result{
		MessageID: msg.ID,
		Channel:   ChannelEmail,
		Success:   true,
	}, nil
}

func (l *LogSender) Close() error { return nil }
