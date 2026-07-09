package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Record(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)

	ctx := context.Background()
	err := svc.Record(ctx, "login", "user1", "Alice", "auth", "logged in", "192.168.1.1", "success")
	require.NoError(t, err)

	entries, err := svc.Query(ctx, Filter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "login", entries[0].Action)
	assert.Equal(t, "user1", entries[0].UserID)
}

func TestService_QueryFilterByUser(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()

	svc.Record(ctx, "login", "u1", "", "", "", "", "success")
	svc.Record(ctx, "login", "u2", "", "", "", "", "success")
	svc.Record(ctx, "logout", "u1", "", "", "", "", "success")

	entries, err := svc.Query(ctx, Filter{UserID: "u1"})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestService_QueryFilterByAction(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()

	svc.Record(ctx, "login", "u1", "", "", "", "", "success")
	svc.Record(ctx, "logout", "u1", "", "", "", "", "success")

	entries, err := svc.Query(ctx, Filter{Action: "logout"})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestService_QueryFilterByTime(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()

	svc.Record(ctx, "old", "u1", "", "", "", "", "success")
	time.Sleep(time.Millisecond)
	svc.Record(ctx, "new", "u1", "", "", "", "", "success")

	entries, err := svc.Query(ctx, Filter{StartTime: time.Now().Add(-time.Hour), EndTime: time.Now()})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestService_QueryPagination(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		svc.Record(ctx, "test", "u1", "", "", "", "", "success")
	}

	entries, err := svc.Query(ctx, Filter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entries, err = svc.Query(ctx, Filter{Offset: 8})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestService_QueryEmpty(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)

	entries, err := svc.Query(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()
	store.Append(context.Background(), Entry{Action: "test"})
	require.NoError(t, store.Close())

	entries, err := store.Query(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestHelperFunctions(t *testing.T) {
	ctx := context.Background()

	err := RecordLogin(ctx, "u1", "Alice", "10.0.0.1", "success")
	require.NoError(t, err)

	err = RecordLogout(ctx, "u1", "Alice", "10.0.0.1")
	require.NoError(t, err)

	err = RecordCreate(ctx, "u1", "Alice", "user:42", "created user 42")
	require.NoError(t, err)

	err = RecordUpdate(ctx, "u1", "Alice", "user:42", "updated email")
	require.NoError(t, err)

	err = RecordDelete(ctx, "u1", "Alice", "user:43", "deleted user 43")
	require.NoError(t, err)

	err = RecordFailure(ctx, "login", "u2", "auth", "invalid password")
	require.NoError(t, err)

	entries, err := Query(ctx, Filter{})
	require.NoError(t, err)
	assert.Len(t, entries, 6)
}

var Query = func(ctx context.Context, filter Filter) ([]Entry, error) {
	return getDefaultService().Query(ctx, filter)
}

func TestInitDefaultService(t *testing.T) {
	newSvc := NewService(NewMemoryStore())
	InitDefaultService(newSvc)
	assert.NotNil(t, getDefaultService())
}
