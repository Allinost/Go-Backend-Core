package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { client.Close() })
	return s, client
}

func TestRedisStream_PublishAndConsume(t *testing.T) {
	_, client := setupTestRedis(t)
	bus := NewRedisStream(client, "test-group", "test-consumer")

	ctx := context.Background()
	err := bus.EnsureGroup(ctx, "test.stream")
	require.NoError(t, err)

	err = bus.Publish(ctx, "test.stream", Event{
		Payload: map[string]any{"msg": "hello"},
	})
	require.NoError(t, err)

	done := make(chan struct{})
	var received int
	consumeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	go func() {
		bus.Consume(consumeCtx, []string{"test.stream"}, func(ctx context.Context, event Event) error {
			received++
			assert.Equal(t, "hello", event.Payload["msg"])
			return nil
		})
		close(done)
	}()

	<-done
	assert.Equal(t, 1, received)
}

func TestRedisStream_MultipleMessages(t *testing.T) {
	_, client := setupTestRedis(t)
	bus := NewRedisStream(client, "test-group", "test-consumer")

	ctx := context.Background()
	bus.EnsureGroup(ctx, "test.batch")

	for i := 0; i < 5; i++ {
		bus.Publish(ctx, "test.batch", Event{
			Payload: map[string]any{"n": i},
		})
	}

	var count int
	consumeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	go bus.Consume(consumeCtx, []string{"test.batch"}, func(ctx context.Context, event Event) error {
		count++
		return nil
	})

	time.Sleep(500 * time.Millisecond)
	cancel()
	assert.Equal(t, 5, count)
}

func TestRedisStream_ReconnectOnConnectionError(t *testing.T) {
	// 使用一个无法连接的地址验证重连逻辑不会永久阻塞
	s, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	bus := NewRedisStream(client, "test-group", "test-consumer")

	ctx := context.Background()
	bus.EnsureGroup(ctx, "test.reconnect")

	// 关闭 miniredis 模拟断连
	s.Close()

	consumeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Consume 应在重试耗尽或 context 超时后返回错误
	err = bus.Consume(consumeCtx, []string{"test.reconnect"}, func(ctx context.Context, event Event) error {
		return nil
	})
	assert.Error(t, err)
}

func TestRedisStream_ConsumeContextCancel(t *testing.T) {
	_, client := setupTestRedis(t)
	bus := NewRedisStream(client, "test-group", "test-consumer")

	ctx := context.Background()
	bus.EnsureGroup(ctx, "test.cancel")

	consumeCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err := bus.Consume(consumeCtx, []string{"test.cancel"}, func(ctx context.Context, event Event) error {
		return nil
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
