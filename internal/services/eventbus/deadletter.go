package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// DeadLetterEntry 死信队列条目，记录失败事件及相关信息
type DeadLetterEntry struct {
	Event       Event     `json:"event"`        // 原始事件
	Reason      string    `json:"reason"`       // 失败原因
	Retries     int       `json:"retries"`      // 已重试次数
	LastAttempt time.Time `json:"last_attempt"` // 最后尝试时间
}

// DeadLetterStore 死信存储接口，支持推送、列出、重放和长度查询
type DeadLetterStore interface {
	Push(event Event, reason string, retries int) error       // 推送死信
	List() ([]DeadLetterEntry, error)                         // 列出所有死信
	Replay(ctx context.Context, handler EventHandler) []error // 重放死信
	Len() (int, error)                                        // 死信数量
}

// DeadLetterQueue 内存死信队列，固定容量，超出时丢弃最旧条目
type DeadLetterQueue struct {
	mu      sync.RWMutex
	entries []DeadLetterEntry // 死信条目列表
	maxSize int               // 最大容量
}

// NewDeadLetterQueue 创建内存死信队列，默认最大 1000
func NewDeadLetterQueue(maxSize int) *DeadLetterQueue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &DeadLetterQueue{maxSize: maxSize}
}

// Push 向死信队列添加条目，超出容量时丢弃最早条目
func (dlq *DeadLetterQueue) Push(event Event, reason string, retries int) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	if len(dlq.entries) >= dlq.maxSize {
		dlq.entries = dlq.entries[1:]
	}

	dlq.entries = append(dlq.entries, DeadLetterEntry{
		Event:       event,
		Reason:      reason,
		Retries:     retries,
		LastAttempt: time.Now(),
	})
	return nil
}

// List 返回所有死信条目副本
func (dlq *DeadLetterQueue) List() ([]DeadLetterEntry, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	result := make([]DeadLetterEntry, len(dlq.entries))
	copy(result, dlq.entries)
	return result, nil
}

// Replay 重放所有死信，成功条目被清除，失败条目重新入队
func (dlq *DeadLetterQueue) Replay(ctx context.Context, handler EventHandler) []error {
	dlq.mu.Lock()
	entries := dlq.entries
	dlq.entries = nil
	dlq.mu.Unlock()

	var errs []error
	for _, entry := range entries {
		if err := handler(ctx, entry.Event); err != nil {
			errs = append(errs, fmt.Errorf("replay failed for event %s: %w", entry.Event.ID, err))
			dlq.Push(entry.Event, err.Error(), entry.Retries+1)
		}
	}
	return errs
}

// Len 返回当前死信数量
func (dlq *DeadLetterQueue) Len() (int, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.entries), nil
}

// RedisDeadLetterQueue Redis 持久化死信队列（生产环境使用）
type RedisDeadLetterQueue struct {
	client  redis.UniversalClient
	prefix  string // Redis key 前缀，如 "dlq:mytopic"
	maxSize int64
}

// NewRedisDeadLetterQueue 创建 Redis 持久化死信队列，默认最大 1000
func NewRedisDeadLetterQueue(client redis.UniversalClient, prefix string, maxSize int) *RedisDeadLetterQueue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &RedisDeadLetterQueue{
		client:  client,
		prefix:  prefix,
		maxSize: int64(maxSize),
	}
}

// Push 将死信入队到 Redis Stream，使用管道批量操作
func (r *RedisDeadLetterQueue) Push(event Event, reason string, retries int) error {
	entry := DeadLetterEntry{
		Event:       event,
		Reason:      reason,
		Retries:     retries,
		LastAttempt: time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal dead letter: %w", err)
	}

	ctx := context.Background()
	pipe := r.client.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: r.prefix,
		Values: map[string]any{"data": string(data)},
		MaxLen: r.maxSize,
		Approx: true,
	})
	_, err = pipe.Exec(ctx)
	return err
}

// List 从 Redis Stream 读取所有死信条目
func (r *RedisDeadLetterQueue) List() ([]DeadLetterEntry, error) {
	ctx := context.Background()
	msgs, err := r.client.XRange(ctx, r.prefix, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("read dead letters: %w", err)
	}

	entries := make([]DeadLetterEntry, 0, len(msgs))
	for _, msg := range msgs {
		var entry DeadLetterEntry
		dataStr, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(dataStr), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Replay 重放所有 Redis 死信，成功后清空 Stream
func (r *RedisDeadLetterQueue) Replay(ctx context.Context, handler EventHandler) []error {
	entries, err := r.List()
	if err != nil {
		return []error{fmt.Errorf("list dead letters: %w", err)}
	}

	var errs []error
	for _, entry := range entries {
		if err := handler(ctx, entry.Event); err != nil {
			errs = append(errs, fmt.Errorf("replay failed for event %s: %w", entry.Event.ID, err))
			r.Push(entry.Event, err.Error(), entry.Retries+1)
		}
	}

	r.client.Del(ctx, r.prefix)
	return errs
}

// Len 返回 Redis Stream 中的死信数量
func (r *RedisDeadLetterQueue) Len() (int, error) {
	ctx := context.Background()
	len, err := r.client.XLen(ctx, r.prefix).Result()
	if err != nil {
		return 0, err
	}
	return int(len), nil
}
