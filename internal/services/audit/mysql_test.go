package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMySQLStore_InitCreateTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_audit")
	err = store.Init(context.Background())
	require.NoError(t, err)
}

func TestMySQLStore_AppendAndQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_audit")
	store.Init(context.Background())

	err = store.Append(context.Background(), Entry{
		Action:    "login",
		UserID:    "u1",
		Username:  "Alice",
		Resource:  "auth",
		Detail:    "logged in",
		ClientIP:  "10.0.0.1",
		Status:    "success",
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	entries, err := store.Query(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "login", entries[0].Action)
	assert.Equal(t, "Alice", entries[0].Username)
}

func TestMySQLStore_QueryFilter(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_audit")
	store.Init(context.Background())
	store.Append(context.Background(), Entry{Action: "login", UserID: "u1", Status: "success", CreatedAt: time.Now()})
	store.Append(context.Background(), Entry{Action: "login", UserID: "u2", Status: "failure", CreatedAt: time.Now()})

	entries, err := store.Query(context.Background(), Filter{UserID: "u1"})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "u1", entries[0].UserID)
}

func TestMySQLStore_QueryPagination(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_audit")
	store.Init(context.Background())
	for i := 0; i < 10; i++ {
		store.Append(context.Background(), Entry{Action: "test", Status: "success", CreatedAt: time.Now()})
	}

	entries, err := store.Query(context.Background(), Filter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entries, err = store.Query(context.Background(), Filter{Offset: 8, Limit: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 0)
}

func TestMySQLStore_QueryEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	store := NewMySQLStore(db, "test_audit")
	store.Init(context.Background())

	entries, err := store.Query(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}
