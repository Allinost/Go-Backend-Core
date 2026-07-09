package notification

import (
	"context"
	"fmt"
	"sync"
)

// Channel 通知渠道类型
type Channel string

const (
	ChannelEmail   Channel = "email"
	ChannelSMS     Channel = "sms"
	ChannelPush    Channel = "push"
	ChannelWebhook Channel = "webhook"
)

// Priority 通知优先级
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

// Message 通知消息体
type Message struct {
	ID       string            `json:"id"`
	Channel  Channel           `json:"channel"`
	To       []string          `json:"to"`
	Subject  string            `json:"subject,omitempty"`
	Body     string            `json:"body"`
	HTMLBody string            `json:"html_body,omitempty"`
	Priority Priority          `json:"priority"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Result 通知发送结果
type Result struct {
	MessageID string  `json:"message_id"`
	Channel   Channel `json:"channel"`
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
}

// Sender 通知发送器接口，每种渠道独立实现
type Sender interface {
	Name() string
	Channel() Channel
	Send(ctx context.Context, msg Message) (*Result, error)
	Close() error
}

// Service 通知服务，支持多发送器注册和回退机制
type Service struct {
	mu      sync.RWMutex
	senders map[Channel][]Sender
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
	name string
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
