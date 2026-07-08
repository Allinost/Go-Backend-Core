package eventbus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	PublishAsync(ctx context.Context, topic string, event Event) <-chan error
	Subscribe(topic string, handler EventHandler) (Subscription, error)
	Unsubscribe(sub Subscription) error
}

type localBus struct {
	mu         sync.RWMutex
	subs       map[string][]Subscription
	subCounter int
}

func NewLocal() EventBus {
	return &localBus{
		subs: make(map[string][]Subscription),
	}
}

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

func (b *localBus) PublishAsync(ctx context.Context, topic string, event Event) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- b.Publish(ctx, topic, event)
		close(ch)
	}()
	return ch
}

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
