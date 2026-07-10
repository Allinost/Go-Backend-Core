package eventbus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// EventBus 事件总线接口，定义发布、订阅和取消订阅操作
type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	PublishAsync(ctx context.Context, topic string, event Event) <-chan error
	Subscribe(topic string, handler EventHandler) (Subscription, error)
	Unsubscribe(sub Subscription) error
}

// localBus 本地内存事件总线，支持通配符订阅
type localBus struct {
	mu         sync.RWMutex
	subs       map[string][]Subscription // 主题到订阅列表的映射
	subCounter int                       // 自增订阅计数器
}

// NewLocal 创建本地事件总线
func NewLocal() EventBus {
	return &localBus{
		subs: make(map[string][]Subscription),
	}
}

// Publish 发布事件到指定主题，按通配符匹配规则分发
func (b *localBus) Publish(ctx context.Context, topic string, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	event.Topic = topic
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	var handlers []EventHandler
	for t, subs := range b.subs {
		if matchTopic(t, topic) {
			for _, sub := range subs {
				handlers = append(handlers, sub.Handler)
			}
		}
	}

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// PublishAsync 异步发布事件，在 goroutine 中执行
func (b *localBus) PublishAsync(ctx context.Context, topic string, event Event) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- b.Publish(ctx, topic, event)
		close(ch)
	}()
	return ch
}

// Subscribe 订阅指定主题的事件，支持通配符 *
func (b *localBus) Subscribe(topic string, handler EventHandler) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subCounter++
	sub := Subscription{
		ID:      fmt.Sprintf("sub_%d", b.subCounter),
		Topic:   topic,
		Handler: handler,
	}
	b.subs[topic] = append(b.subs[topic], sub)
	return sub, nil
}

// Unsubscribe 取消指定订阅，未找到时返回错误
func (b *localBus) Unsubscribe(sub Subscription) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[sub.Topic]
	for i, s := range subs {
		if s.ID == sub.ID {
			b.subs[sub.Topic] = append(subs[:i], subs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription %s not found", sub.ID)
}

// matchTopic 匹配主题，支持后缀通配符 *
func matchTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}
	if !strings.HasSuffix(pattern, "*") {
		return false
	}
	prefix := strings.TrimSuffix(pattern, "*")
	return strings.HasPrefix(topic, prefix)
}
