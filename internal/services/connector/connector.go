// Package connector 定义集成连接器接口与注册表。
//
// 连接器负责把统一事件投递到外部系统（Webhook、消息队列、OpenClaw 等），
// 业务模块不直接依赖具体连接器实现。
package connector

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrAlreadyRegistered 连接器 ID 重复
	ErrAlreadyRegistered = errors.New("connector: already registered")
	// ErrNotFound 连接器不存在
	ErrNotFound = errors.New("connector: not found")
	// ErrInvalidConfig 连接器配置不合法
	ErrInvalidConfig = errors.New("connector: invalid config")
)

// Message 投递给连接器的统一消息
type Message struct {
	EventID    string            `json:"event_id"`
	Event      string            `json:"event"`
	OccurredAt time.Time         `json:"occurred_at"`
	Payload    json.RawMessage   `json:"payload"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Connector 连接器接口
type Connector interface {
	ID() string
	Type() string
	Send(ctx context.Context, msg Message) error
}

// Descriptor 连接器可公开的描述信息
type Descriptor struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Registry 连接器注册表
type Registry struct {
	mu    sync.RWMutex
	items map[string]Connector
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Connector)}
}

// Register 注册连接器
func (r *Registry) Register(c Connector) error {
	if c == nil || strings.TrimSpace(c.ID()) == "" {
		return ErrInvalidConfig
	}
	id := strings.TrimSpace(c.ID())
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[id]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, id)
	}
	r.items[id] = c
	return nil
}

// Get 获取连接器
func (r *Registry) Get(id string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.items[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return c, nil
}

// All 返回全部连接器实例
func (r *Registry) All() []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Connector, 0, len(r.items))
	for _, c := range r.items {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID() < result[j].ID() })
	return result
}

// List 返回全部连接器描述，按 ID 排序
func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Descriptor, 0, len(r.items))
	for id, c := range r.items {
		result = append(result, Descriptor{ID: id, Type: c.Type()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// Remove 移除连接器
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, strings.TrimSpace(id))
}

// Sign 计算 Webhook 签名：HMAC-SHA256(secret, timestamp + "." + body)
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 校验 Webhook 签名，使用常量时间比较
func VerifySignature(secret string, timestamp int64, signature string, body []byte) bool {
	expected := Sign(secret, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}
