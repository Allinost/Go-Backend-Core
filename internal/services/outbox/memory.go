package outbox

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore 基于内存的外盒存储，用于单测与本地开发
type MemoryStore struct {
	mu     sync.RWMutex
	events map[string]Event
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{events: make(map[string]Event)}
}

// Append 写入事件，ID 重复时报错
func (m *MemoryStore) Append(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.events[event.ID]; exists {
		return ErrAlreadyExists
	}
	m.events[event.ID] = cloneEvent(event)
	return nil
}

// Get 读取事件
func (m *MemoryStore) Get(ctx context.Context, id string) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	event, ok := m.events[id]
	if !ok {
		return Event{}, ErrNotFound
	}
	return cloneEvent(event), nil
}

// List 按条件查询事件，按创建时间升序返回
func (m *MemoryStore) List(ctx context.Context, filter Filter) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Event, 0, len(m.events))
	for _, event := range m.events {
		if !matchEvent(event, filter) {
			continue
		}
		result = append(result, cloneEvent(event))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []Event{}, nil
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	return result, nil
}

// MarkDelivered 标记为已投递
func (m *MemoryStore) MarkDelivered(ctx context.Context, id string, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	event, ok := m.events[id]
	if !ok {
		return ErrNotFound
	}
	event.Status = StatusDelivered
	event.LastError = ""
	event.UpdatedAt = at
	event.DeliveredAt = &at
	m.events[id] = event
	return nil
}

// MarkFailed 标记失败并累加尝试次数
func (m *MemoryStore) MarkFailed(ctx context.Context, id, reason string, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	event, ok := m.events[id]
	if !ok {
		return ErrNotFound
	}
	event.Status = StatusFailed
	event.Attempts++
	event.LastError = strings.TrimSpace(reason)
	event.UpdatedAt = at
	m.events[id] = event
	return nil
}

func matchEvent(event Event, filter Filter) bool {
	if filter.Topic != "" && event.Topic != filter.Topic {
		return false
	}
	if filter.Source != "" && event.Source != filter.Source {
		return false
	}
	if filter.Status != "" && string(event.Status) != filter.Status {
		return false
	}
	if filter.AggregateType != "" && event.AggregateType != filter.AggregateType {
		return false
	}
	if filter.AggregateID != "" && event.AggregateID != filter.AggregateID {
		return false
	}
	if !filter.Since.IsZero() && event.CreatedAt.Before(filter.Since) {
		return false
	}
	return true
}

func cloneEvent(event Event) Event {
	if event.Payload != nil {
		event.Payload = append([]byte(nil), event.Payload...)
	}
	if event.DeliveredAt != nil {
		at := *event.DeliveredAt
		event.DeliveredAt = &at
	}
	return event
}
