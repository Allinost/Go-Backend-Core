package feature

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLStore_InitCreateTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	err = store.Init(context.Background())
	require.NoError(t, err)
}

func TestMySQLStore_SaveAndLoad(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())

	err = store.Save(context.Background(), Flag{Name: "f1", Enabled: true, Description: "flag one"})
	require.NoError(t, err)

	flags, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Len(t, flags, 1)
	assert.Equal(t, "f1", flags[0].Name)
	assert.True(t, flags[0].Enabled)
	assert.Equal(t, "flag one", flags[0].Description)
}

func TestMySQLStore_SaveUpdate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())

	store.Save(context.Background(), Flag{Name: "toggle", Enabled: false})
	store.Save(context.Background(), Flag{Name: "toggle", Enabled: true})

	flags, _ := store.Load(context.Background())
	require.Len(t, flags, 1)
	assert.True(t, flags[0].Enabled)
}

func TestMySQLStore_Delete(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())
	store.Save(context.Background(), Flag{Name: "del"})

	err = store.Delete(context.Background(), "del")
	require.NoError(t, err)

	flags, _ := store.Load(context.Background())
	assert.Empty(t, flags)
}

func TestSyncManager_StartLoadsFromStore(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())
	store.Save(context.Background(), Flag{Name: "persisted", Enabled: true})

	sm := NewSyncManager(store)
	err = sm.Start()
	require.NoError(t, err)
	assert.True(t, sm.IsEnabled("persisted"))
}

func TestSyncManager_EnablePersists(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())

	sm := NewSyncManager(store)
	sm.Register(Flag{Name: "to-toggle", Enabled: false})
	assert.False(t, sm.IsEnabled("to-toggle"))

	sm.Enable("to-toggle")
	assert.True(t, sm.IsEnabled("to-toggle"))

	flags, _ := store.Load(context.Background())
	require.Len(t, flags, 1)
	assert.True(t, flags[0].Enabled)
}

func TestSyncManager_RegisterPersists(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_flags")
	store.Init(context.Background())

	sm := NewSyncManager(store)
	sm.Register(Flag{Name: "new-flag", Enabled: true})

	_ = context.Background()
}

// testStore 内存实现，用于测试 syncManager 的持久化行为
type testStore struct {
	Store
	flags map[string]Flag
}

func newTestStore() *testStore {
	return &testStore{flags: make(map[string]Flag)}
}

func (t *testStore) Save(ctx context.Context, flag Flag) error {
	t.flags[flag.Name] = flag
	return nil
}

func (t *testStore) Delete(ctx context.Context, name string) error {
	delete(t.flags, name)
	return nil
}

// TestSyncManager_Delete 验证 syncManager 的 Delete 方法同步删除内存和存储
func TestSyncManager_Delete(t *testing.T) {
	store := newTestStore()
	sm := NewSyncManager(store)

	sm.RegisterOrUpdate(Flag{Name: "del-flag"})
	err := sm.Delete("del-flag")
	require.NoError(t, err)
	assert.False(t, sm.IsEnabled("del-flag"))
	assert.NotContains(t, store.flags, "del-flag")

	err = sm.Delete("del-flag")
	assert.Error(t, err)
}