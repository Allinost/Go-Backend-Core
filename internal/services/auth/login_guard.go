package auth

import (
	"sync"
	"time"
)

type loginEntry struct {
	failures    int
	lockedUntil time.Time
}

// LoginGuard 进程内登录失败锁定。V3.0 单实例部署足够；多实例可后续换 Redis。
type LoginGuard struct {
	mu          sync.Mutex
	maxFailures int
	lockFor     time.Duration
	entries     map[string]*loginEntry
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
func (g *LoginGuard) Check(key string) (bool, time.Duration) {
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

func (g *LoginGuard) Fail(key string) {
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

func (g *LoginGuard) Success(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}
