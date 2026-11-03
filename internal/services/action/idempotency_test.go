package action

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
)

func TestGatewayIdempotency(t *testing.T) {
	registry := NewRegistry()
	var calls int32
	registry.MustRegister(Definition{
		Name:     "order.create",
		Scopes:   []string{authz.ScopeOwnerWrite},
		Mutating: true,
		Handler: func(_ context.Context, _ Request) (any, error) {
			n := atomic.AddInt32(&calls, 1)
			return map[string]any{"call": n}, nil
		},
	})
	gateway := NewGateway(registry, WithIdempotency(NewMemoryIdempotencyStore(), time.Minute))
	req := Request{
		Action:    "order.create",
		Scopes:    authz.ScopeOwnerWrite,
		Actor:     "user:1",
		RequestID: "req-1",
	}

	first, err := gateway.Invoke(context.Background(), req)
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	second, err := gateway.Invoke(context.Background(), req)
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	if second.Replayed != true {
		t.Fatalf("second.Replayed = false, want true")
	}
	firstJSON, _ := json.Marshal(first.Data)
	secondJSON, _ := json.Marshal(second.Data)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("cached result mismatch: %s vs %s", firstJSON, secondJSON)
	}

	req.RequestID = "req-2"
	if _, err := gateway.Invoke(context.Background(), req); err != nil {
		t.Fatalf("third invoke: %v", err)
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}

	req.RequestID = ""
	if _, err := gateway.Invoke(context.Background(), req); err != nil {
		t.Fatalf("no request id invoke: %v", err)
	}
	if calls != 3 {
		t.Fatalf("handler calls = %d, want 3", calls)
	}
}

func TestGatewayIdempotencyDoesNotCacheErrors(t *testing.T) {
	registry := NewRegistry()
	var calls int32
	fail := true
	registry.MustRegister(Definition{
		Name:     "order.create",
		Scopes:   []string{authz.ScopeOwnerWrite},
		Mutating: true,
		Handler: func(_ context.Context, _ Request) (any, error) {
			n := atomic.AddInt32(&calls, 1)
			if fail {
				return nil, errors.New("boom")
			}
			return map[string]any{"call": n}, nil
		},
	})
	gateway := NewGateway(registry, WithIdempotency(NewMemoryIdempotencyStore(), time.Minute))
	req := Request{
		Action:    "order.create",
		Scopes:    authz.ScopeOwnerWrite,
		Actor:     "user:1",
		RequestID: "retry-1",
	}

	if _, err := gateway.Invoke(context.Background(), req); err == nil {
		t.Fatalf("first invoke should fail")
	}
	fail = false
	result, err := gateway.Invoke(context.Background(), req)
	if err != nil {
		t.Fatalf("retry invoke: %v", err)
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}
	if result.Replayed {
		t.Fatalf("retry should execute, not replay failed call")
	}
}

func TestGatewayIdempotencyConcurrent(t *testing.T) {
	registry := NewRegistry()
	var calls int32
	registry.MustRegister(Definition{
		Name:     "order.create",
		Scopes:   []string{authz.ScopeOwnerWrite},
		Mutating: true,
		Handler: func(_ context.Context, _ Request) (any, error) {
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&calls, 1)
			return map[string]any{"ok": true}, nil
		},
	})
	gateway := NewGateway(registry, WithIdempotency(NewMemoryIdempotencyStore(), time.Minute))
	req := Request{
		Action:    "order.create",
		Scopes:    authz.ScopeOwnerWrite,
		Actor:     "user:1",
		RequestID: "concurrent-1",
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := gateway.Invoke(context.Background(), req); err != nil {
				t.Errorf("invoke: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
}

func TestGatewayIdempotencyIgnoresReadActions(t *testing.T) {
	registry := NewRegistry()
	var calls int32
	registry.MustRegister(Definition{
		Name:   "stock.read",
		Scopes: []string{authz.ScopeOwnerRead},
		Handler: func(_ context.Context, _ Request) (any, error) {
			atomic.AddInt32(&calls, 1)
			return map[string]any{"ok": true}, nil
		},
	})
	gateway := NewGateway(registry, WithIdempotency(NewMemoryIdempotencyStore(), time.Minute))
	req := Request{
		Action:    "stock.read",
		Scopes:    authz.ScopeOwnerRead,
		Actor:     "user:1",
		RequestID: "read-1",
	}
	for i := 0; i < 3; i++ {
		if _, err := gateway.Invoke(context.Background(), req); err != nil {
			t.Fatalf("invoke: %v", err)
		}
	}
	if calls != 3 {
		t.Fatalf("handler calls = %d, want 3", calls)
	}
}
