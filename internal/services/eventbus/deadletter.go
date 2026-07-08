package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type DeadLetterEntry struct {
	Event       Event     `json:"event"`
	Reason      string    `json:"reason"`
	Retries     int       `json:"retries"`
	LastAttempt time.Time `json:"last_attempt"`
}

type DeadLetterStore interface {
	Push(event Event, reason string, retries int) error
	List() ([]DeadLetterEntry, error)
	Replay(ctx context.Context, handler EventHandler) []error
	Len() (int, error)
}

type DeadLetterQueue struct {
	mu      sync.RWMutex
	entries []DeadLetterEntry
	maxSize int
}

func NewDeadLetterQueue(maxSize int) *DeadLetterQueue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &DeadLetterQueue{maxSize: maxSize}
}

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

func (dlq *DeadLetterQueue) List() ([]DeadLetterEntry, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	result := make([]DeadLetterEntry, len(dlq.entries))
	copy(result, dlq.entries)
	return result, nil
}

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

func (r *RedisDeadLetterQueue) Len() (int, error) {
	ctx := context.Background()
	len, err := r.client.XLen(ctx, r.prefix).Result()
	if err != nil {
		return 0, err
	}
	return int(len), nil
}
