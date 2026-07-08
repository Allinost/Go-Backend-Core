package eventbus

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDLQ_PushList(t *testing.T) {
	dlq := NewDeadLetterQueue(100)
	evt := Event{ID: "evt-1", Topic: "test"}
	err := dlq.Push(evt, "handler error", 3)
	require.NoError(t, err)

	entries, err := dlq.List()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "evt-1", entries[0].Event.ID)
	assert.Equal(t, "handler error", entries[0].Reason)
	assert.Equal(t, 3, entries[0].Retries)
}

func TestDLQ_MaxSize(t *testing.T) {
	dlq := NewDeadLetterQueue(3)
	for i := 0; i < 5; i++ {
		dlq.Push(Event{ID: "evt"}, "error", 1)
	}
	n, err := dlq.Len()
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestDLQ_Len(t *testing.T) {
	dlq := NewDeadLetterQueue(100)
	n, err := dlq.Len()
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	dlq.Push(Event{ID: "e1"}, "err", 1)
	n, err = dlq.Len()
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestDLQ_ReplaySuccess(t *testing.T) {
	dlq := NewDeadLetterQueue(100)
	dlq.Push(Event{ID: "e1"}, "err", 1)
	dlq.Push(Event{ID: "e2"}, "err", 1)

	errs := dlq.Replay(context.Background(), func(ctx context.Context, event Event) error {
		return nil
	})
	assert.Empty(t, errs)
	n, _ := dlq.Len()
	assert.Equal(t, 0, n)
}

func TestDLQ_ReplayPartialFailure(t *testing.T) {
	dlq := NewDeadLetterQueue(100)
	dlq.Push(Event{ID: "e1"}, "err", 1)
	dlq.Push(Event{ID: "e2"}, "err", 1)

	errs := dlq.Replay(context.Background(), func(ctx context.Context, event Event) error {
		if event.ID == "e2" {
			return errors.New("still failing")
		}
		return nil
	})
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "still failing")
	n, _ := dlq.Len()
	assert.Equal(t, 1, n)
}

func TestRedisDLQ_PushList(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	dlq := NewRedisDeadLetterQueue(client, "dlq:test", 100)
	err = dlq.Push(Event{ID: "e1", Topic: "test"}, "timeout", 2)
	require.NoError(t, err)

	entries, err := dlq.List()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "e1", entries[0].Event.ID)
	assert.Equal(t, "timeout", entries[0].Reason)
	assert.Equal(t, 2, entries[0].Retries)
}

func TestRedisDLQ_MaxSize(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	dlq := NewRedisDeadLetterQueue(client, "dlq:maxsize", 3)
	for i := 0; i < 5; i++ {
		dlq.Push(Event{ID: "e"}, "err", 1)
	}
	n, err := dlq.Len()
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}
