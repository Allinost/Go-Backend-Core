package action

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore 缓存写操作的执行结果。
// key 由 gateway 根据 actor + action + request_id 生成。
type IdempotencyStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Save(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type memoryIdempotencyItem struct {
	value     []byte
	expiresAt time.Time
}

// MemoryIdempotencyStore 进程内幂等缓存，用于单实例或 Redis 不可用的降级场景。
type MemoryIdempotencyStore struct {
	mu          sync.Mutex
	items       map[string]memoryIdempotencyItem
	now         func() time.Time
	lastCleanup time.Time
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		items: make(map[string]memoryIdempotencyItem),
		now:   time.Now,
	}
}

func (s *MemoryIdempotencyStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[key]
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && s.now().After(item.expiresAt) {
		delete(s.items, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

func (s *MemoryIdempotencyStore) Save(_ context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if now.Sub(s.lastCleanup) > time.Minute {
		for itemKey, item := range s.items {
			if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
				delete(s.items, itemKey)
			}
		}
		s.lastCleanup = now
	}

	item := memoryIdempotencyItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = now.Add(ttl)
	}
	s.items[key] = item
	return nil
}

// RedisIdempotencyStore 基于 Redis 的幂等缓存，TTL 到期自动清理。
type RedisIdempotencyStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisIdempotencyStore(client redis.UniversalClient) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{
		client: client,
		prefix: "integration:idempotency:",
	}
}

func (s *RedisIdempotencyStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := s.client.Get(ctx, s.prefix+key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (s *RedisIdempotencyStore) Save(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, s.prefix+key, value, ttl).Err()
}
