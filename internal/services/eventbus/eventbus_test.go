package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocal_PublishSubscribe(t *testing.T) {
	bus := NewLocal()
	received := make(chan Event, 1)

	sub, err := bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		received <- event
		return nil
	})
	assert.NoError(t, err)

	evt := Event{Payload: map[string]any{"key": "value"}}
	err = bus.Publish(context.Background(), "test.topic", evt)
	assert.NoError(t, err)

	r := <-received
	assert.Equal(t, "test.topic", r.Topic)
	assert.Equal(t, "value", r.Payload["key"])
	assert.NotEmpty(t, r.Timestamp)

	bus.Unsubscribe(sub)
}

func TestLocal_Unsubscribe(t *testing.T) {
	bus := NewLocal()
	var count atomic.Int32

	sub, _ := bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.topic", Event{})
	assert.Equal(t, int32(1), count.Load())

	bus.Unsubscribe(sub)
	bus.Publish(context.Background(), "test.topic", Event{})
	assert.Equal(t, int32(1), count.Load())
}

func TestLocal_UnsubscribeNotFound(t *testing.T) {
	bus := NewLocal()
	err := bus.Unsubscribe(Subscription{ID: "nonexistent", Topic: "test"})
	assert.Error(t, err)
}

func TestLocal_MultipleSubscribers(t *testing.T) {
	bus := NewLocal()
	var count1, count2 atomic.Int32

	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		count1.Add(1)
		return nil
	})
	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		count2.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.topic", Event{})
	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(1), count2.Load())
}

func TestLocal_WildcardTopic(t *testing.T) {
	bus := NewLocal()
	var count atomic.Int32

	bus.Subscribe("test.*", func(ctx context.Context, event Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.alpha", Event{})
	bus.Publish(context.Background(), "test.beta", Event{})
	bus.Publish(context.Background(), "other.topic", Event{})
	assert.Equal(t, int32(2), count.Load())
}

func TestLocal_ExactMatch(t *testing.T) {
	bus := NewLocal()
	var count atomic.Int32

	bus.Subscribe("test.one", func(ctx context.Context, event Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.one", Event{})
	bus.Publish(context.Background(), "test.one.two", Event{})
	assert.Equal(t, int32(1), count.Load())
}

func TestLocal_ConcurrentPublish(t *testing.T) {
	bus := NewLocal()
	var count atomic.Int32

	bus.Subscribe("test.*", func(ctx context.Context, event Event) error {
		count.Add(1)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(context.Background(), "test.n", Event{})
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(20), count.Load())
}

func TestLocal_PublishAsync(t *testing.T) {
	bus := NewLocal()
	received := make(chan Event, 1)
	_, _ = bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		received <- event
		return nil
	})

	ch := bus.PublishAsync(context.Background(), "test.topic", Event{
		Payload: map[string]any{"key": "value"},
	})

	err := <-ch
	assert.NoError(t, err)
	ev := <-received
	assert.Equal(t, "value", ev.Payload["key"])
}

func TestLocal_PublishAsync_Error(t *testing.T) {
	bus := NewLocal()
	_, _ = bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		return assert.AnError
	})

	ch := bus.PublishAsync(context.Background(), "test.topic", Event{})
	err := <-ch
	assert.Error(t, err)
}

func TestLocal_PublishAsync_NoBlock(t *testing.T) {
	bus := NewLocal()
	ch := bus.PublishAsync(context.Background(), "no_subscribers", Event{})
	err := <-ch
	assert.NoError(t, err)
}

func TestLocal_PublishToNoSubscribers(t *testing.T) {
	bus := NewLocal()
	err := bus.Publish(context.Background(), "nonexistent", Event{})
	assert.NoError(t, err)
}
