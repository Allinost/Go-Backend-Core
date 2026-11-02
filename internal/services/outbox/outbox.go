// Package outbox 提供事务外盒（outbox）事件模型与存储抽象。
//
// 业务写操作与事件写入共用同一数据库事务时，可先落库为 pending，
// 再由分发器异步投递到连接器，避免直接调用外部系统造成数据不一致。
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Status 事件投递状态
type Status string

const (
	// StatusPending 待投递
	StatusPending Status = "pending"
	// StatusDelivered 已投递
	StatusDelivered Status = "delivered"
	// StatusFailed 投递失败，可重试
	StatusFailed Status = "failed"
)

// ErrNotFound 事件不存在
var ErrNotFound = errors.New("outbox: event not found")

// ErrAlreadyExists 事件 ID 已存在
var ErrAlreadyExists = errors.New("outbox: event already exists")

// Event 外盒事件
type Event struct {
	ID            string          `json:"id"`
	Topic         string          `json:"topic"`
	Source        string          `json:"source"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
	Status        Status          `json:"status"`
	Attempts      int             `json:"attempts"`
	LastError     string          `json:"last_error,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeliveredAt   *time.Time      `json:"delivered_at,omitempty"`
}

// EmitInput 业务侧写入事件所需的最小字段
type EmitInput struct {
	Topic         string
	Source        string
	AggregateType string
	AggregateID   string
	Payload       any
}

// Filter 事件查询条件，空字段不参与过滤
type Filter struct {
	Topic         string
	Source        string
	Status        string
	AggregateType string
	AggregateID   string
	Since         time.Time
	Offset        int
	Limit         int
}

// Store 外盒存储接口
type Store interface {
	Append(ctx context.Context, event Event) error
	Get(ctx context.Context, id string) (Event, error)
	List(ctx context.Context, filter Filter) ([]Event, error)
	MarkDelivered(ctx context.Context, id string, at time.Time) error
	MarkFailed(ctx context.Context, id, reason string, at time.Time) error
	DeleteDeliveredBefore(ctx context.Context, before time.Time) (int64, error)
}

// Emitter 供业务模块依赖的最小写入接口
type Emitter interface {
	Emit(ctx context.Context, input EmitInput) (Event, error)
}

// Service 外盒服务
type Service struct {
	store Store
	now   func() time.Time
}

// NewService 创建外盒服务
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Emit 写入一条 pending 事件
func (s *Service) Emit(ctx context.Context, input EmitInput) (Event, error) {
	input.Topic = strings.TrimSpace(input.Topic)
	if input.Topic == "" {
		return Event{}, errors.New("outbox: topic 不能为空")
	}

	payload, err := marshalPayload(input.Payload)
	if err != nil {
		return Event{}, err
	}

	now := s.now().UTC()
	event := Event{
		ID:            uuid.New().String(),
		Topic:         input.Topic,
		Source:        strings.TrimSpace(input.Source),
		AggregateType: strings.TrimSpace(input.AggregateType),
		AggregateID:   strings.TrimSpace(input.AggregateID),
		Payload:       payload,
		Status:        StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.Append(ctx, event); err != nil {
		return Event{}, err
	}
	return event, nil
}

// Get 读取单条事件
func (s *Service) Get(ctx context.Context, id string) (Event, error) {
	if strings.TrimSpace(id) == "" {
		return Event{}, ErrNotFound
	}
	return s.store.Get(ctx, id)
}

// List 查询事件
func (s *Service) List(ctx context.Context, filter Filter) ([]Event, error) {
	return s.store.List(ctx, filter)
}

// MarkDelivered 标记事件已投递
func (s *Service) MarkDelivered(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.store.MarkDelivered(ctx, id, s.now().UTC())
}

// MarkFailed 标记事件投递失败，并累加重试次数
func (s *Service) MarkFailed(ctx context.Context, id, reason string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.store.MarkFailed(ctx, id, reason, s.now().UTC())
}

// DeleteDeliveredBefore 清理指定时间之前已投递的事件，返回删除条数。
func (s *Service) DeleteDeliveredBefore(ctx context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, nil
	}
	return s.store.DeleteDeliveredBefore(ctx, before)
}

func marshalPayload(payload any) (json.RawMessage, error) {
	switch v := payload.(type) {
	case nil:
		return json.RawMessage("null"), nil
	case json.RawMessage:
		if len(v) == 0 {
			return json.RawMessage("null"), nil
		}
		if !json.Valid(v) {
			return nil, fmt.Errorf("outbox: payload 不是合法 JSON")
		}
		return v, nil
	case []byte:
		if len(v) == 0 {
			return json.RawMessage("null"), nil
		}
		if !json.Valid(v) {
			return nil, fmt.Errorf("outbox: payload 不是合法 JSON")
		}
		return json.RawMessage(v), nil
	default:
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("outbox: 序列化 payload 失败: %w", err)
		}
		return data, nil
	}
}
