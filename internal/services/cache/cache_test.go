package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCache(t *testing.T) (*MultiLevelCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	c := New(rdb, Options{DefaultTTL: 1 * time.Hour, DefaultNilTTL: 10 * time.Second})
	t.Cleanup(func() { c.Close() })

	return c, mr
}

func TestSetAndGet(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "key1", "hello", 0)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "key1", &val)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestGetMiss(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	var val string
	err := c.Get(ctx, "nonexistent", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestDelete(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	err = c.Delete(ctx, "key1")
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "key1", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestExists(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	exists, err := c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)

	err = c.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	exists, err = c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestClear(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "key1", "v1", 0)
	require.NoError(t, err)
	err = c.Set(ctx, "key2", "v2", 0)
	require.NoError(t, err)

	err = c.Clear(ctx)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "key1", &val)
	assert.Equal(t, ErrMiss, err)
	err = c.Get(ctx, "key2", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestTTL(t *testing.T) {
	c, mr := testCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "ephemeral", "soon", 50*time.Millisecond)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "ephemeral", &val)
	require.NoError(t, err)
	assert.Equal(t, "soon", val)

	mr.FastForward(100 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	err = c.Get(ctx, "ephemeral", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestL1OnlyTTL(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	err := c.Set(ctx, "fast", "expire", 10*time.Millisecond)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	var val string
	err = c.Get(ctx, "fast", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestStructValue(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	type User struct {
		ID   int
		Name string
	}
	u := User{ID: 1, Name: "alice"}

	err := c.Set(ctx, "user:1", u, 0)
	require.NoError(t, err)

	var got User
	err = c.Get(ctx, "user:1", &got)
	require.NoError(t, err)
	assert.Equal(t, u, got)
}

func TestSetNil(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.SetNil(ctx, "empty-key")
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "empty-key", &val)
	assert.Equal(t, ErrNilValue, err)
}

func TestJitter(t *testing.T) {
	base := 1 * time.Minute
	jitter := 30 * time.Second

	results := make(map[time.Duration]int)
	for i := 0; i < 1000; i++ {
		ttl := jitterTTL(base, jitter)
		results[ttl]++
		assert.GreaterOrEqual(t, ttl, time.Second)
		assert.LessOrEqual(t, ttl, base+jitter)
	}
	assert.Greater(t, len(results), 1)
}

func TestJitterNoJitter(t *testing.T) {
	ttl := jitterTTL(5*time.Minute, 0)
	assert.Equal(t, 5*time.Minute, ttl)
}

func TestL1Eviction(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour, L1MaxSize: 3})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		err := c.Set(ctx, "k", i, 0)
		require.NoError(t, err)
	}
	assert.LessOrEqual(t, len(c.l1), 3)
}

func TestConcurrentAccess(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key"
			err := c.Set(ctx, key, i, 0)
			assert.NoError(t, err)
			var val int
			err = c.Get(ctx, key, &val)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
}

func TestNoRedis(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	err := c.Set(ctx, "local", "value", 0)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "local", &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)

	err = c.Delete(ctx, "local")
	require.NoError(t, err)

	_, err = c.Exists(ctx, "local")
	require.NoError(t, err)

	err = c.Clear(ctx)
	require.NoError(t, err)
}

func TestClose(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	err := c.Close()
	require.NoError(t, err)
	assert.NotPanics(t, func() { c.Close() })
}

func TestL1Evict(t *testing.T) {
	c, _ := testCache(t)

	c.l1Set("stale", []byte("data"), -time.Second)
	time.Sleep(10 * time.Millisecond)

	_, ok := c.l1Get("stale")
	assert.False(t, ok)
}

func TestDefaultOptions(t *testing.T) {
	assert.Equal(t, 5*time.Minute, DefaultOptions.DefaultTTL)
	assert.Equal(t, 30*time.Second, DefaultOptions.DefaultNilTTL)
	assert.Equal(t, 10*time.Second, DefaultOptions.Jitter)
	assert.Equal(t, 10000, DefaultOptions.L1MaxSize)
}

func TestIsNilValue(t *testing.T) {
	assert.True(t, isNilValue(nilPlaceholder))
	assert.False(t, isNilValue([]byte{0x00}))
	assert.False(t, isNilValue([]byte{0x01, 0x02}))
}

func TestEncodeDecode(t *testing.T) {
	type Item struct{ A, B int }
	original := Item{A: 1, B: 2}

	codec := gobCodec{}
	data, err := encodeWith(codec, original)
	require.NoError(t, err)

	var decoded Item
	err = decodeWith(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecode_JSON(t *testing.T) {
	type Item struct{ A, B int }
	original := Item{A: 1, B: 2}

	codec := jsonCodec{}
	data, err := encodeWith(codec, original)
	require.NoError(t, err)

	var decoded Item
	err = decodeWith(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecode_LegacyGob(t *testing.T) {
	type Item struct{ A, B int }
	original := Item{A: 1, B: 2}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded Item
	err := decodeWith(buf.Bytes(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecode_VersionMismatch(t *testing.T) {
	err := decodeWith([]byte{0xFF, 0x01}, nil)
	assert.Error(t, err)
}

func TestEncodeDecode_EmptyData(t *testing.T) {
	err := decodeWith([]byte{}, nil)
	assert.Error(t, err)
}

func TestCacheError(t *testing.T) {
	assert.Equal(t, "cache: miss", ErrMiss.Error())
	assert.Equal(t, "cache: nil value", ErrNilValue.Error())
}

func TestNewWithDefaults(t *testing.T) {
	c := New(nil, Options{})
	t.Cleanup(func() { c.Close() })
	assert.Equal(t, DefaultOptions.DefaultTTL, c.opts.DefaultTTL)
	assert.Equal(t, DefaultOptions.DefaultNilTTL, c.opts.DefaultNilTTL)
	assert.Equal(t, 0*time.Second, c.opts.Jitter)
	assert.Equal(t, DefaultOptions.L1MaxSize, c.opts.L1MaxSize)
}

func TestCtxDeadline(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "key", "val", 0)
	require.NoError(t, err)

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	var val string
	err = c.Get(cancelledCtx, "key", &val)
	assert.Error(t, err)
}

func TestGetMany(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "a", "one", 0)
	_ = c.Set(ctx, "b", "two", 0)

	result, err := c.GetMany(ctx, []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	var got string
	_ = decodeWith(result["a"], &got)
	assert.Equal(t, "one", got)
	_ = decodeWith(result["b"], &got)
	assert.Equal(t, "two", got)
}

func TestGetMany_EmptyKeys(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	result, err := c.GetMany(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetMany_AllMiss(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	result, err := c.GetMany(ctx, []string{"x", "y"})
	assert.Nil(t, result)
	assert.Equal(t, ErrMiss, err)
}

func TestSetMany(t *testing.T) {
	c, mr := testCache(t)
	ctx := context.Background()

	items := map[string]interface{}{
		"x": 10,
		"y": 20,
	}
	err := c.SetMany(ctx, items, 0)
	require.NoError(t, err)

	assert.True(t, mr.Exists("cache:x"))
	assert.True(t, mr.Exists("cache:y"))

	var val int
	err = c.Get(ctx, "x", &val)
	require.NoError(t, err)
	assert.Equal(t, 10, val)

	err = c.Get(ctx, "y", &val)
	require.NoError(t, err)
	assert.Equal(t, 20, val)
}

func TestStats(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	stats := c.Stats()
	assert.Equal(t, 0, stats.L1Count)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)

	_ = c.Set(ctx, "k", "v", 0)
	var val string
	_ = c.Get(ctx, "k", &val)
	_ = c.Get(ctx, "miss", &val)

	stats = c.Stats()
	assert.Equal(t, 1, stats.L1Count)
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

func TestMetricsHooks(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	var hits, misses, sets, evicts int64
	c.SetMetrics(&testMetrics{
		hit:   func() { atomic.AddInt64(&hits, 1) },
		miss:  func() { atomic.AddInt64(&misses, 1) },
		set:   func() { atomic.AddInt64(&sets, 1) },
		evict: func() { atomic.AddInt64(&evicts, 1) },
	})

	_ = c.Set(ctx, "k", "v", 0)
	assert.Equal(t, int64(2), sets, "Set calls metrics for L1 and L2")

	var val string
	_ = c.Get(ctx, "k", &val)
	assert.Equal(t, int64(1), hits, "L1 hit")

	_ = c.Get(ctx, "miss", &val)
	assert.Equal(t, int64(1), misses, "L2 miss")
}

type testMetrics struct {
	hit, miss, set, evict func()
}

func (m *testMetrics) Hit(s string)   { m.hit() }
func (m *testMetrics) Miss(s string)  { m.miss() }
func (m *testMetrics) Set(s string)   { m.set() }
func (m *testMetrics) Evict(s string) { m.evict() }

func TestWarmup(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Warmup(ctx, func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"a": "one", "b": "two"}, nil
	})
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "a", &val)
	require.NoError(t, err)
	assert.Equal(t, "one", val)

	err = c.Get(ctx, "b", &val)
	require.NoError(t, err)
	assert.Equal(t, "two", val)
}

func TestWarmup_LoaderError(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	err := c.Warmup(ctx, func(ctx context.Context) (map[string]interface{}, error) {
		return nil, fmt.Errorf("loader failed")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loader failed")
}

func TestTryLock_Success(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	ok, err := c.TryLock(ctx, "resource:1", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.TryLock(ctx, "resource:1", 10*time.Second)
	require.NoError(t, err)
	assert.False(t, ok, "second lock should fail")
}

func TestTryLock_NoRedis(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	ok, err := c.TryLock(ctx, "k", 10*time.Second)
	assert.False(t, ok)
	assert.Equal(t, ErrNoRedis, err)
}

func TestUnlock(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	ok, err := c.TryLock(ctx, "mutex", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)

	err = c.Unlock(ctx, "mutex")
	require.NoError(t, err)

	ok, err = c.TryLock(ctx, "mutex", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok, "should be lockable after unlock")
}

func TestPublishInvalidation(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "stale", "value", time.Hour)

	var val string
	err := c.Get(ctx, "stale", &val)
	require.NoError(t, err)

	err = c.PublishInvalidation(ctx, "stale")
	require.NoError(t, err)

	err = c.Get(ctx, "stale", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestPublishInvalidation_NoRedis(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v", time.Hour)
	err := c.PublishInvalidation(ctx, "k")
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "k", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestSlowLog(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour, SlowThreshold: 1 * time.Nanosecond})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	err := c.Set(ctx, "k", "v", 0)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "k", &val)
	require.NoError(t, err)
	assert.Equal(t, "v", val)
}

func TestDefaultSlowThreshold(t *testing.T) {
	assert.Equal(t, 100*time.Millisecond, DefaultOptions.SlowThreshold)
}

func TestL1LRUEviction(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour, L1MaxSize: 3})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	_ = c.Set(ctx, "a", 1, 0)
	_ = c.Set(ctx, "b", 2, 0)
	_ = c.Set(ctx, "c", 3, 0)

	// access "a" to make it most recently used
	var val int
	_ = c.Get(ctx, "a", &val)

	// this should evict "b" (oldest lastHit, not "a")
	_ = c.Set(ctx, "d", 4, 0)

	// "a" should still be in L1
	err := c.Get(ctx, "a", &val)
	require.NoError(t, err)
	assert.Equal(t, 1, val)

	// "b" should be evicted
	err = c.Get(ctx, "b", &val)
	assert.Equal(t, ErrMiss, err)
}

func TestSingleflightBreakdown(t *testing.T) {
	c, mr := testCache(t)
	ctx := context.Background()

	// Pre-encode the value with gob so L2 returns valid data
	data, err := encodeWith(gobCodec{}, "data")
	require.NoError(t, err)
	mr.Set("cache:hot", string(data))

	var wg sync.WaitGroup
	results := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var val string
			if err := c.Get(ctx, "hot", &val); err == nil {
				results[idx] = val
			}
		}(i)
	}
	wg.Wait()

	for _, r := range results {
		assert.Equal(t, "data", r)
	}
}

func TestGetMany_NoRedis(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	_ = c.Set(ctx, "a", "one", 0)

	result, err := c.GetMany(ctx, []string{"a", "b"})
	require.NoError(t, err)
	assert.Len(t, result, 1)

	var got string
	_ = decodeWith(result["a"], &got)
	assert.Equal(t, "one", got)
}

func TestSetMany_NoRedis(t *testing.T) {
	c := New(nil, Options{DefaultTTL: 1 * time.Hour})
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()

	err := c.SetMany(ctx, map[string]interface{}{"a": 1, "b": 2}, 0)
	require.NoError(t, err)

	var val int
	err = c.Get(ctx, "a", &val)
	require.NoError(t, err)
	assert.Equal(t, 1, val)
}

func TestRaceDetection(t *testing.T) {
	c, _ := testCache(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "race"
			_ = c.Set(ctx, key, i, time.Duration(i)*time.Millisecond)
			var val int
			_ = c.Get(ctx, key, &val)
			_, _ = c.Exists(ctx, key)
			_ = c.Delete(ctx, key)
			_ = c.Stats()
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Clear(ctx)
		}()
	}

	wg.Wait()
}

func BenchmarkSet(b *testing.B) {
	c, _ := testCacheB(b)
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = c.Set(ctx, fmt.Sprintf("key:%d", i), "value", time.Hour)
	}
}

func BenchmarkGet_Hit(b *testing.B) {
	c, _ := testCacheB(b)
	ctx := context.Background()
	_ = c.Set(ctx, "hit", "bench-value", time.Hour)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var val string
		_ = c.Get(ctx, "hit", &val)
	}
}

func BenchmarkGet_Miss(b *testing.B) {
	c, _ := testCacheB(b)
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var val string
		_ = c.Get(ctx, fmt.Sprintf("miss:%d", i), &val)
	}
}

func BenchmarkGetMany(b *testing.B) {
	c, _ := testCacheB(b)
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_ = c.Set(ctx, fmt.Sprintf("k:%d", i), i, time.Hour)
	}
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("k:%d", i)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = c.GetMany(ctx, keys)
	}
}

func BenchmarkTryLock(b *testing.B) {
	c, _ := testCacheB(b)
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = c.TryLock(ctx, fmt.Sprintf("lock:%d", i), time.Second)
	}
}

func testCacheB(b *testing.B) (*MultiLevelCache, *miniredis.Miniredis) {
	b.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b.Cleanup(func() { rdb.Close() })

	c := New(rdb, Options{DefaultTTL: 1 * time.Hour, DefaultNilTTL: 10 * time.Second})
	b.Cleanup(func() { c.Close() })

	return c, mr
}
