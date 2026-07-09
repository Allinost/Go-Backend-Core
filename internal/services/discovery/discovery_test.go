package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryRegistry_Register(t *testing.T) {
	r := NewMemoryRegistry()
	err := r.Register(context.Background(), Instance{ID: "svc-1", Name: "auth", Address: "10.0.0.1", Port: 8080}, time.Minute)
	require.NoError(t, err)

	instances, err := r.Discover(context.Background(), "auth")
	require.NoError(t, err)
	assert.Len(t, instances, 1)
	assert.Equal(t, "svc-1", instances[0].ID)
}

func TestMemoryRegistry_Deregister(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register(context.Background(), Instance{ID: "svc-1", Name: "test"}, time.Minute)
	err := r.Deregister(context.Background(), "svc-1")
	require.NoError(t, err)

	instances, _ := r.Discover(context.Background(), "test")
	assert.Empty(t, instances)
}

func TestMemoryRegistry_DeregisterNotFound(t *testing.T) {
	r := NewMemoryRegistry()
	err := r.Deregister(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestMemoryRegistry_Renew(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register(context.Background(), Instance{ID: "svc-1", Name: "test"}, time.Minute)
	err := r.Renew(context.Background(), "svc-1")
	require.NoError(t, err)
}

func TestMemoryRegistry_RenewNotFound(t *testing.T) {
	r := NewMemoryRegistry()
	err := r.Renew(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestMemoryRegistry_List(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register(context.Background(), Instance{ID: "a", Name: "svc-a"}, time.Minute)
	r.Register(context.Background(), Instance{ID: "b", Name: "svc-b"}, time.Minute)

	all, err := r.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestMemoryRegistry_ListEmpty(t *testing.T) {
	r := NewMemoryRegistry()
	all, err := r.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestMemoryRegistry_DiscoverOnlyUp(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register(context.Background(), Instance{ID: "up-inst", Name: "svc", Status: "up"}, time.Minute)

	r.mu.Lock()
	r.instances["down-inst"] = Instance{ID: "down-inst", Name: "svc", Status: "down"}
	r.mu.Unlock()

	instances, _ := r.Discover(context.Background(), "svc")
	assert.Len(t, instances, 1)
	assert.Equal(t, "up-inst", instances[0].ID)
}

func TestMemoryRegistry_Watch(t *testing.T) {
	r := NewMemoryRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := r.Watch(ctx, "svc-watch")
	require.NoError(t, err)

	r.Register(context.Background(), Instance{ID: "w1", Name: "svc-watch"}, time.Minute)

	select {
	case instances := <-ch:
		assert.Len(t, instances, 1)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for watch notification")
	}
}

func TestMemoryRegistry_WatchCancel(t *testing.T) {
	r := NewMemoryRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := r.Watch(ctx, "svc")
	require.NoError(t, err)

	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after cancel")
}

func TestMemoryRegistry_Close(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register(context.Background(), Instance{ID: "x", Name: "test"}, time.Minute)
	require.NoError(t, r.Close())

	all, _ := r.List(context.Background())
	assert.Empty(t, all)
}
