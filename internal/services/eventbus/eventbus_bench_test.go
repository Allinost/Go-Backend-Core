package eventbus

import (
	"context"
	"testing"
)

func BenchmarkLocal_Publish(b *testing.B) {
	bus := NewLocal()
	bus.Subscribe("bench.*", func(ctx context.Context, event Event) error {
		return nil
	})

	evt := Event{Payload: map[string]any{"key": "value"}}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(ctx, "bench.test", evt)
	}
}

func BenchmarkLocal_PublishParallel(b *testing.B) {
	bus := NewLocal()
	bus.Subscribe("bench.*", func(ctx context.Context, event Event) error {
		return nil
	})

	evt := Event{Payload: map[string]any{"key": "value"}}
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Publish(ctx, "bench.test", evt)
		}
	})
}

func BenchmarkLocal_MultipleSubs(b *testing.B) {
	bus := NewLocal()
	for i := 0; i < 10; i++ {
		bus.Subscribe("bench.*", func(ctx context.Context, event Event) error {
			return nil
		})
	}

	evt := Event{}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(ctx, "bench.test", evt)
	}
}
