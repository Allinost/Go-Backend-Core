package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBlacklist interface {
	Revoke(ctx context.Context, jti string, exp time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

type InMemoryBlacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

func NewInMemoryBlacklist() *InMemoryBlacklist {
	return &InMemoryBlacklist{entries: make(map[string]time.Time)}
}

func (b *InMemoryBlacklist) Revoke(_ context.Context, jti string, exp time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = exp
	return nil
}

func (b *InMemoryBlacklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	exp, ok := b.entries[jti]
	if !ok {
		return false, nil
	}
	if time.Now().After(exp) {
		delete(b.entries, jti)
		return false, nil
	}
	return true, nil
}

type RedisBlacklist struct {
	client *redis.Client
	prefix string
}

func NewRedisBlacklist(client *redis.Client) *RedisBlacklist {
	return &RedisBlacklist{client: client, prefix: "blacklist:"}
}

func (b *RedisBlacklist) Revoke(ctx context.Context, jti string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		ttl = time.Hour
	}
	return b.client.Set(ctx, b.prefix+jti, "1", ttl).Err()
}

func (b *RedisBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	val, err := b.client.Exists(ctx, b.prefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("auth: 黑名单查询失败: %w", err)
	}
	return val > 0, nil
}
