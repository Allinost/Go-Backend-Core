package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist token 黑名单接口，支持吊销和查询
type TokenBlacklist interface {
	Revoke(ctx context.Context, jti string, exp time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// InMemoryBlacklist 基于内存的 token 黑名单实现
type InMemoryBlacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// NewInMemoryBlacklist 创建内存黑名单实例
func NewInMemoryBlacklist() *InMemoryBlacklist {
	return &InMemoryBlacklist{entries: make(map[string]time.Time)}
}

// Revoke 将指定 JTI 的 token 加入黑名单
func (b *InMemoryBlacklist) Revoke(_ context.Context, jti string, exp time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = exp
	return nil
}

// IsRevoked 检查指定 JTI 是否已被吊销，过期记录自动清理
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

// RedisBlacklist 基于 Redis 的 token 黑名单实现
type RedisBlacklist struct {
	client *redis.Client
	prefix string
}

// NewRedisBlacklist 创建 Redis 黑名单实例
func NewRedisBlacklist(client *redis.Client) *RedisBlacklist {
	return &RedisBlacklist{client: client, prefix: "blacklist:"}
}

// Revoke 将 JTI 写入 Redis，并设置与 token 过期时间一致的 TTL
func (b *RedisBlacklist) Revoke(ctx context.Context, jti string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		ttl = time.Hour
	}
	return b.client.Set(ctx, b.prefix+jti, "1", ttl).Err()
}

// IsRevoked 查询 Redis 中是否存在指定 JTI（即是否被吊销）
func (b *RedisBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	val, err := b.client.Exists(ctx, b.prefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("auth: 黑名单查询失败: %w", err)
	}
	return val > 0, nil
}
