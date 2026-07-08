package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/services/monitor"
)

var (
	global   map[string]Cache
	globalMu sync.RWMutex
)

type promMetrics struct {
	hit   *prometheus.CounterVec
	miss  *prometheus.CounterVec
	set   *prometheus.CounterVec
	evict *prometheus.CounterVec
}

func newPromMetrics(reg *monitor.MetricsRegistry) *promMetrics {
	return &promMetrics{
		hit:   reg.NewCounter("cache_hits_total", "缓存命中次数", "level"),
		miss:  reg.NewCounter("cache_misses_total", "缓存未命中次数", "level"),
		set:   reg.NewCounter("cache_sets_total", "缓存写入次数", "level"),
		evict: reg.NewCounter("cache_evictions_total", "缓存淘汰次数", "level"),
	}
}

func (p *promMetrics) Hit(lvl string)   { p.hit.WithLabelValues(lvl).Inc() }
func (p *promMetrics) Miss(lvl string)  { p.miss.WithLabelValues(lvl).Inc() }
func (p *promMetrics) Set(lvl string)   { p.set.WithLabelValues(lvl).Inc() }
func (p *promMetrics) Evict(lvl string) { p.evict.WithLabelValues(lvl).Inc() }

func InitAll(cfg *config.Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if global == nil {
		global = make(map[string]Cache)
	}

	ns := cfg.Cache
	defaultTTL, _ := time.ParseDuration(ns.DefaultTTL)
	if defaultTTL <= 0 {
		defaultTTL = DefaultOptions.DefaultTTL
	}
	defaultNilTTL, _ := time.ParseDuration(ns.DefaultNilTTL)
	if defaultNilTTL <= 0 {
		defaultNilTTL = DefaultOptions.DefaultNilTTL
	}
	jitter, _ := time.ParseDuration(ns.Jitter)
	if jitter < 0 {
		jitter = DefaultOptions.Jitter
	}
	l1MaxSize := ns.L1MaxSize
	if l1MaxSize <= 0 {
		l1MaxSize = DefaultOptions.L1MaxSize
	}

	opts := Options{
		DefaultTTL:    defaultTTL,
		DefaultNilTTL: defaultNilTTL,
		Jitter:        jitter,
		L1MaxSize:     l1MaxSize,
	}

	var rdb redis.UniversalClient
	if ns.RedisKey != "" {
		client := database.GetRedis(ns.RedisKey)
		if client != nil {
			rdb = client
		}
	}

	c := New(rdb, opts)

	reg := monitor.NewMetricsRegistry("go_backend_core", "cache")
	c.SetMetrics(newPromMetrics(reg))

	global["default"] = c
	return nil
}

func Get(name string) Cache {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if global == nil {
		return nil
	}
	c := global[name]
	if c == nil {
		c = global["default"]
	}
	return c
}

func CloseAll() {
	globalMu.Lock()
	defer globalMu.Unlock()
	for name, c := range global {
		if err := c.Close(); err != nil {
			fmt.Printf("cache: 关闭 [%s] 失败: %v\n", name, err)
		}
	}
	global = make(map[string]Cache)
}
