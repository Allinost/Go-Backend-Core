package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type loginEntry struct {
	failures    int
	lockedUntil time.Time
}

// LoginGuard 登录失败锁定。优先 Redis（多实例），不可用时回退进程内存。
type LoginGuard struct {
	mu          sync.Mutex
	maxFailures int
	lockFor     time.Duration
	entries     map[string]*loginEntry
	redis       *redis.Client
}

func NewLoginGuard(maxFailures int, lockFor time.Duration) *LoginGuard {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if lockFor <= 0 {
		lockFor = 15 * time.Minute
	}
	return &LoginGuard{maxFailures: maxFailures, lockFor: lockFor, entries: make(map[string]*loginEntry)}
}

var defaultLoginGuard = NewLoginGuard(5, 15*time.Minute)

func DefaultLoginGuard() *LoginGuard { return defaultLoginGuard }

// ConfigureLoginPolicy 热更新默认锁定策略。
func ConfigureLoginPolicy(maxFailures, lockMinutes int) {
	defaultLoginGuard.SetPolicy(maxFailures, time.Duration(lockMinutes)*time.Minute)
}

// EnableRedis 启用 Redis 共享锁定状态；nil 则回退内存。
func EnableRedis(rdb *redis.Client) {
	defaultLoginGuard.SetRedis(rdb)
}

func (g *LoginGuard) SetRedis(rdb *redis.Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.redis = rdb
}

func (g *LoginGuard) SetPolicy(maxFailures int, lockFor time.Duration) {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if lockFor <= 0 {
		lockFor = 15 * time.Minute
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.maxFailures = maxFailures
	g.lockFor = lockFor
}

// Check 返回是否锁定以及剩余锁定时间。
func (g *LoginGuard) Check(ctx context.Context, key string) (bool, time.Duration) {
	if g.useRedis() {
		ttl, err := g.redis.TTL(ctx, lockKey(key)).Result()
		if err == nil && ttl > 0 {
			return true, ttl
		}
	}
	return g.checkMemory(key)
}

func (g *LoginGuard) Fail(ctx context.Context, key string) {
	if g.useRedis() {
		failKey := failKey(key)
		n, err := g.redis.Incr(ctx, failKey).Result()
		if err == nil {
			g.redis.Expire(ctx, failKey, g.lockDuration())
			if n >= int64(g.maxFailuresValue()) {
				g.redis.Set(ctx, lockKey(key), "1", g.lockDuration())
				g.redis.Del(ctx, failKey)
			}
			return
		}
	}
	g.failMemory(key)
}

func (g *LoginGuard) Success(ctx context.Context, key string) {
	if g.useRedis() {
		g.redis.Del(ctx, failKey(key), lockKey(key))
	}
	g.successMemory(key)
}

func (g *LoginGuard) useRedis() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.redis != nil
}

func (g *LoginGuard) lockDuration() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lockFor
}

func (g *LoginGuard) maxFailuresValue() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.maxFailures
}

func lockKey(key string) string { return "v3:login:lock:" + strings.ToLower(key) }
func failKey(key string) string { return "v3:login:fail:" + strings.ToLower(key) }

func (g *LoginGuard) checkMemory(key string) (bool, time.Duration) {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	if !ok {
		return false, 0
	}
	if e.lockedUntil.After(now) {
		return true, e.lockedUntil.Sub(now)
	}
	if !e.lockedUntil.IsZero() {
		delete(g.entries, key)
	}
	return false, 0
}

func (g *LoginGuard) failMemory(key string) {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	if !ok {
		e = &loginEntry{}
		g.entries[key] = e
	}
	e.failures++
	if e.failures >= g.maxFailures {
		e.lockedUntil = now.Add(g.lockFor)
		e.failures = 0
	}
}

func (g *LoginGuard) successMemory(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}
