package cache

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache 定义多级缓存接口，包含基本的缓存操作、统计、预热、分布式锁及失效通知能力
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	Stats() Stats
	Warmup(ctx context.Context, loader func(ctx context.Context) (map[string]interface{}, error)) error
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
	SubscribeInvalidation(ctx context.Context, handler func(key string)) error
	PublishInvalidation(ctx context.Context, key string) error
	Close() error
}

// Metrics 定义缓存指标收集接口，用于记录命中、未命中、写入和淘汰事件
type Metrics interface {
	Hit(name string)
	Miss(name string)
	Set(name string)
	Evict(name string)
}

// NopMetrics 空操作指标实现，不执行任何记录
type NopMetrics struct{}

func (NopMetrics) Hit(string)   {}
func (NopMetrics) Miss(string)  {}
func (NopMetrics) Set(string)   {}
func (NopMetrics) Evict(string) {}

// Stats 缓存统计信息
type Stats struct {
	L1Count int   // L1本地缓存当前条目数
	Hits    int64 // 总命中次数
	Misses  int64 // 总未命中次数
}

// Options 缓存配置选项
type Options struct {
	DefaultTTL    time.Duration // 默认缓存过期时间
	DefaultNilTTL time.Duration // nil值缓存的过期时间，用于防止缓存穿透
	Jitter        time.Duration // TTL随机抖动范围，防止大量缓存同时过期（缓存雪崩）
	L1MaxSize     int           // L1本地缓存最大条目数，超出后淘汰最久未命中条目
	SlowThreshold time.Duration // 慢操作日志阈值，超过此值记录警告日志
	Codec         string        // 编解码器名称，支持 "gob" 或 "json"
}

// DefaultOptions 默认缓存配置，TTL 5分钟，NilTTL 30秒，抖动10秒，L1上限10000
var DefaultOptions = Options{
	DefaultTTL:    5 * time.Minute,
	DefaultNilTTL: 30 * time.Second,
	Jitter:        10 * time.Second,
	L1MaxSize:     10000,
	SlowThreshold: 100 * time.Millisecond,
	Codec:         "gob",
}

type l1Entry struct {
	data      []byte
	expiresAt time.Time
	lastHit   int64
}

// MultiLevelCache 多级缓存实现，包含 L1（本地内存）和 L2（Redis）两级缓存
// 支持缓存预热、分布式锁、失效通知、慢日志和 Prometheus 指标收集
type MultiLevelCache struct {
	opts      Options
	rdb       redis.UniversalClient
	pubsub    *redis.PubSub
	l1        map[string]*l1Entry
	l1mu      sync.RWMutex
	sf        singleflight.Group
	codec     Codec
	close     chan struct{}
	closeOnce sync.Once
	metrics   Metrics
	hits      int64
	misses    int64
	clock     int64

	invokeMu     sync.RWMutex
	invalidation []func(string)
}

// New 创建多级缓存实例，根据选项初始化编解码器并启动后台淘汰协程
func New(rdb redis.UniversalClient, opts Options) *MultiLevelCache {
	if opts.DefaultTTL <= 0 {
		opts.DefaultTTL = DefaultOptions.DefaultTTL
	}
	if opts.DefaultNilTTL <= 0 {
		opts.DefaultNilTTL = DefaultOptions.DefaultNilTTL
	}
	if opts.Jitter < 0 {
		opts.Jitter = DefaultOptions.Jitter
	}
	if opts.L1MaxSize <= 0 {
		opts.L1MaxSize = DefaultOptions.L1MaxSize
	}
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = DefaultOptions.SlowThreshold
	}

	codec := getCodec(opts.Codec)
	if codec == nil {
		codec = getCodec("gob")
	}

	c := &MultiLevelCache{
		opts:    opts,
		rdb:     rdb,
		l1:      make(map[string]*l1Entry),
		codec:   codec,
		close:   make(chan struct{}),
		metrics: NopMetrics{},
	}

	go c.evictLoop()
	return c
}

// SetMetrics 设置自定义指标收集器
func (c *MultiLevelCache) SetMetrics(m Metrics) {
	if m != nil {
		c.metrics = m
	}
}

// jitterTTL 对基础 TTL 添加正负随机抖动，防止大量缓存同时过期导致雪崩
func jitterTTL(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}
	delta := time.Duration(rand.Int63n(int64(jitter*2))) - jitter
	ttl := base + delta
	if ttl < time.Second {
		ttl = time.Second
	}
	return ttl
}

// slowLog 记录超过慢操作阈值的缓存操作日志
func (c *MultiLevelCache) slowLog(op string, dur time.Duration) {
	if dur >= c.opts.SlowThreshold {
		log.Printf("[cache slow] %s took %v (threshold %v)", op, dur, c.opts.SlowThreshold)
	}
}

// l1Get 从 L1 本地缓存获取数据，已过期则自动删除并返回 false
func (c *MultiLevelCache) l1Get(key string) ([]byte, bool) {
	c.l1mu.Lock()
	defer c.l1mu.Unlock()
	entry, ok := c.l1[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.l1, key)
		return nil, false
	}
	c.clock++
	entry.lastHit = c.clock
	return entry.data, true
}

// l1Set 写入 L1 本地缓存，超出容量上限时淘汰最久未命中的条目
func (c *MultiLevelCache) l1Set(key string, data []byte, ttl time.Duration) {
	c.l1mu.Lock()
	defer c.l1mu.Unlock()
	if len(c.l1) >= c.opts.L1MaxSize {
		c.evictOneLocked()
	}
	c.clock++
	c.l1[key] = &l1Entry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
		lastHit:   c.clock,
	}
}

// evictOneLocked 淘汰 L1 中最久未命中的条目（LRU），调用方需持有 l1mu 锁
func (c *MultiLevelCache) evictOneLocked() {
	var oldestKey string
	var oldestHit int64 = 1<<63 - 1
	for k, v := range c.l1 {
		if v.lastHit < oldestHit {
			oldestKey = k
			oldestHit = v.lastHit
		}
	}
	if oldestKey != "" {
		delete(c.l1, oldestKey)
		c.metrics.Evict("l1")
	}
}

// l1Del 从 L1 本地缓存中删除指定键
func (c *MultiLevelCache) l1Del(key string) {
	c.l1mu.Lock()
	delete(c.l1, key)
	c.l1mu.Unlock()
}

// l1Clear 清空 L1 本地缓存所有条目
func (c *MultiLevelCache) l1Clear() {
	c.l1mu.Lock()
	c.l1 = make(map[string]*l1Entry)
	c.l1mu.Unlock()
}

// evictLoop 后台协程，每分钟扫描 L1 并清理已过期的条目
func (c *MultiLevelCache) evictLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.l1mu.Lock()
			for k, v := range c.l1 {
				if now.After(v.expiresAt) {
					delete(c.l1, k)
				}
			}
			c.l1mu.Unlock()
		case <-c.close:
			return
		}
	}
}

// encode 使用配置的编解码器将值序列化为带版本号前缀的字节数据
func (c *MultiLevelCache) encode(value interface{}) ([]byte, error) {
	return encodeWith(c.codec, value)
}

// decode 使用配置的编解码器将字节数据反序列化到目标对象
func (c *MultiLevelCache) decode(data []byte, dest interface{}) error {
	return decodeWith(data, dest)
}

// nilPlaceholder nil 值占位符，用于缓存穿透防护，与普通数据区分
var nilPlaceholder = []byte("\x00nil")

// isNilValue 判断字节数据是否为 nil 值占位符
func isNilValue(data []byte) bool {
	return len(data) == 4 && data[0] == 0x00 && data[1] == 'n' && data[2] == 'i' && data[3] == 'l'
}

// l2Key 构造 L2 Redis 缓存键，格式为 namespace:key
func l2Key(namespace, key string) string {
	return namespace + ":" + key
}

// Get 从缓存获取键对应的值，优先查 L1，未命中则查 L2（Redis），使用 singleflight 防止并发穿透
func (c *MultiLevelCache) Get(ctx context.Context, key string, dest interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	start := time.Now()
	defer func() { c.slowLog("GET "+key, time.Since(start)) }()

	if data, ok := c.l1Get(key); ok {
		c.hits++
		c.metrics.Hit("l1")
		if isNilValue(data) {
			return ErrNilValue
		}
		return c.decode(data, dest)
	}

	if c.rdb != nil {
		v, err, _ := c.sf.Do(key, func() (interface{}, error) {
			data, err := c.rdb.Get(ctx, l2Key("cache", key)).Bytes()
			if err == nil {
				c.l1Set(key, data, c.opts.DefaultTTL)
				if isNilValue(data) {
					return nil, ErrNilValue
				}
				return data, nil
			}
			if err == redis.Nil {
				return nil, ErrMiss
			}
			return nil, err
		})
		if err != nil {
			c.misses++
			c.metrics.Miss("l2")
			if err == ErrMiss || err == ErrNilValue {
				return err
			}
			return err
		}
		c.hits++
		c.metrics.Hit("l2")
		return c.decode(v.([]byte), dest)
	}

	c.misses++
	c.metrics.Miss("l1")
	return ErrMiss
}

// GetMany 批量获取多个键的原始字节值，分别查询 L1 和 L2（Redis MGET），仅返回命中数据
func (c *MultiLevelCache) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return map[string][]byte{}, nil
	}
	start := time.Now()
	defer func() { c.slowLog(fmt.Sprintf("MGET %d keys", len(keys)), time.Since(start)) }()

	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data, ok := c.l1Get(key)
		if ok {
			result[key] = data
			c.hits++
			c.metrics.Hit("l1")
		}
	}

	if c.rdb != nil {
		var missing []string
		for _, key := range keys {
			if _, found := result[key]; !found {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			args := make([]string, len(missing))
			for i, k := range missing {
				args[i] = l2Key("cache", k)
			}
			results, err := c.rdb.MGet(ctx, args...).Result()
			if err != nil {
				return nil, err
			}
			for i, val := range results {
				if val == nil {
					c.misses++
					c.metrics.Miss("l2")
					continue
				}
				data, ok := val.(string)
				if !ok {
					continue
				}
				raw := []byte(data)
				c.l1Set(missing[i], raw, c.opts.DefaultTTL)
				result[missing[i]] = raw
				c.hits++
				c.metrics.Hit("l2")
			}
		}
	}

	if len(result) == 0 {
		return nil, ErrMiss
	}
	return result, nil
}

// Set 将值写入 L1 和 L2 缓存，自动应用 TTL 抖动以防止缓存雪崩
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.opts.DefaultTTL
	}
	ttl = jitterTTL(ttl, c.opts.Jitter)
	start := time.Now()
	defer func() { c.slowLog("SET "+key, time.Since(start)) }()

	data, err := c.encode(value)
	if err != nil {
		return err
	}
	c.l1Set(key, data, ttl)
	c.metrics.Set("l1")
	if c.rdb != nil {
		c.metrics.Set("l2")
		return c.rdb.Set(ctx, l2Key("cache", key), data, ttl).Err()
	}
	return nil
}

// SetMany 批量写入多个缓存项到 L1 和 L2（Redis MSET）
func (c *MultiLevelCache) SetMany(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.opts.DefaultTTL
	}
	ttl = jitterTTL(ttl, c.opts.Jitter)
	start := time.Now()
	defer func() { c.slowLog(fmt.Sprintf("MSET %d keys", len(items)), time.Since(start)) }()

	pairs := make([]interface{}, 0, len(items)*2)
	for key, value := range items {
		data, err := c.encode(value)
		if err != nil {
			return err
		}
		c.l1Set(key, data, ttl)
		c.metrics.Set("l1")
		if c.rdb != nil {
			pairs = append(pairs, l2Key("cache", key), data)
		}
	}

	if c.rdb != nil && len(pairs) > 0 {
		c.metrics.Set("l2")
		return c.rdb.MSet(ctx, pairs...).Err()
	}
	return nil
}

// SetNil 将键标记为 nil 值缓存（使用占位符），防止缓存穿透
func (c *MultiLevelCache) SetNil(ctx context.Context, key string) error {
	ttl := c.opts.DefaultNilTTL
	c.l1Set(key, nilPlaceholder, ttl)
	if c.rdb != nil {
		return c.rdb.Set(ctx, l2Key("cache", key), nilPlaceholder, ttl).Err()
	}
	return nil
}

// Delete 从 L1 和 L2 中删除指定的缓存键
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	c.l1Del(key)
	if c.rdb != nil {
		return c.rdb.Del(ctx, l2Key("cache", key)).Err()
	}
	return nil
}

// Exists 检查缓存键是否存在，先查 L1 本地缓存，未命中则查 L2 Redis
func (c *MultiLevelCache) Exists(ctx context.Context, key string) (bool, error) {
	if _, ok := c.l1Get(key); ok {
		return true, nil
	}
	if c.rdb != nil {
		n, err := c.rdb.Exists(ctx, l2Key("cache", key)).Result()
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
	return false, nil
}

// Clear 清空 L1 缓存，并通过 Redis SCAN + DEL 遍历删除所有 cache: 前缀的 L2 键
func (c *MultiLevelCache) Clear(ctx context.Context) error {
	c.l1Clear()
	if c.rdb != nil {
		iter := c.rdb.Scan(ctx, 0, "cache:*", 0).Iterator()
		for iter.Next(ctx) {
			if err := c.rdb.Del(ctx, iter.Val()).Err(); err != nil {
				return err
			}
		}
		return iter.Err()
	}
	return nil
}

// Warmup 通过加载函数批量预热缓存，将返回的所有键值对写入缓存
func (c *MultiLevelCache) Warmup(ctx context.Context, loader func(ctx context.Context) (map[string]interface{}, error)) error {
	items, err := loader(ctx)
	if err != nil {
		return fmt.Errorf("cache warmup loader: %w", err)
	}
	for key, value := range items {
		if err := c.Set(ctx, key, value, c.opts.DefaultTTL); err != nil {
			return fmt.Errorf("cache warmup set %s: %w", key, err)
		}
	}
	return nil
}

// TryLock 基于 Redis SetNX 实现分布式锁，尝试获取锁并返回是否成功
func (c *MultiLevelCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if c.rdb == nil {
		return false, ErrNoRedis
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	ok, err := c.rdb.SetNX(ctx, l2Key("lock", key), "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Unlock 释放基于 Redis 的分布式锁（直接删除 lock: 前缀的键）
func (c *MultiLevelCache) Unlock(ctx context.Context, key string) error {
	if c.rdb == nil {
		return ErrNoRedis
	}
	return c.rdb.Del(ctx, l2Key("lock", key)).Err()
}

// SubscribeInvalidation 注册缓存失效回调，并订阅 Redis 过期事件以自动清理 L1
func (c *MultiLevelCache) SubscribeInvalidation(ctx context.Context, handler func(key string)) error {
	c.invokeMu.Lock()
	c.invalidation = append(c.invalidation, handler)
	c.invokeMu.Unlock()

	if c.rdb != nil && c.pubsub == nil {
		c.pubsub = c.rdb.Subscribe(ctx, "__keyevent@0__:expired")
		go c.invalidationLoop(ctx)
	}
	return nil
}

// PublishInvalidation 主动发布缓存失效通知，同时删除 L1 和 L2 中的对应键
func (c *MultiLevelCache) PublishInvalidation(ctx context.Context, key string) error {
	if c.rdb != nil {
		if err := c.rdb.Del(ctx, l2Key("cache", key)).Err(); err != nil {
			return err
		}
	}
	c.l1Del(key)
	if c.rdb != nil {
		return c.rdb.Publish(ctx, "cache:invalidate", key).Err()
	}
	return nil
}

func (c *MultiLevelCache) invalidationLoop(ctx context.Context) {
	ch := c.pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			c.l1Del(msg.Payload)
		case <-c.close:
			if c.pubsub != nil {
				c.pubsub.Close()
			}
			return
		case <-ctx.Done():
			if c.pubsub != nil {
				c.pubsub.Close()
			}
			return
		}
	}
}

func (c *MultiLevelCache) Stats() Stats {
	c.l1mu.RLock()
	count := len(c.l1)
	c.l1mu.RUnlock()
	return Stats{
		L1Count: count,
		Hits:    c.hits,
		Misses:  c.misses,
	}
}

func (c *MultiLevelCache) Close() error {
	c.closeOnce.Do(func() { close(c.close) })
	if c.pubsub != nil {
		c.pubsub.Close()
	}
	c.l1Clear()
	return nil
}

type CacheError string

func (e CacheError) Error() string { return string(e) }

const (
	ErrMiss     = CacheError("cache: miss")
	ErrNilValue = CacheError("cache: nil value")
	ErrNoRedis  = CacheError("cache: redis not available")
)
