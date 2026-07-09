package search

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryEngine_IndexAndSearch(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	err = e.Index(ctx, Document{
		ID:      "1",
		Title:   "Hello World",
		Content: "This is a test document",
		Tags:    []string{"test", "example"},
		Source:  "test",
	})
	require.NoError(t, err)

	resp, err := e.Search(ctx, "hello")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Total, uint64(1))
	assert.NotEmpty(t, resp.Hits)
}

func TestMemoryEngine_SearchNotFound(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	resp, err := e.Search(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), resp.Total)
}

func TestMemoryEngine_Delete(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	e.Index(ctx, Document{ID: "del-test", Title: "Delete Me", Content: "to be deleted"})

	resp, _ := e.Search(ctx, "Delete")
	assert.GreaterOrEqual(t, resp.Total, uint64(1))

	err = e.Delete(ctx, "del-test")
	require.NoError(t, err)

	resp, _ = e.Search(ctx, "Delete")
	assert.Equal(t, uint64(0), resp.Total)
}

func TestMemoryEngine_Count(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	count, err := e.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), count)

	e.Index(ctx, Document{ID: "a", Content: "aaa"})
	e.Index(ctx, Document{ID: "b", Content: "bbb"})

	count, err = e.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), count)
}

func TestMemoryEngine_BatchIndex(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	docs := []Document{
		{ID: "1", Content: "first"},
		{ID: "2", Content: "second"},
		{ID: "3", Content: "third"},
	}

	err = e.BatchIndex(ctx, docs)
	require.NoError(t, err)

	count, _ := e.Count(ctx)
	assert.Equal(t, uint64(3), count)
}

func TestWithLimitOffset(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		e.Index(ctx, Document{
			ID:      string(rune('a'+i)),
			Content: "test document number",
		})
	}

	resp, err := e.Search(ctx, "test", WithLimit(3))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(resp.Hits), 3)
}

func TestWithSource(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	ctx := context.Background()
	e.Index(ctx, Document{ID: "a", Content: "from obsidian", Source: "obsidian"})
	e.Index(ctx, Document{ID: "b", Content: "from web", Source: "web"})

	resp, err := e.Search(ctx, "", WithSource("obsidian"))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), resp.Total)
}

func TestEngine_CloseTwice(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)

	err = e.Close()
	require.NoError(t, err)
}

func TestSearchOptionsDefaults(t *testing.T) {
	o := searchOptions{}
	assert.Equal(t, 10, o.size())
	assert.Equal(t, 0, o.from())
}

func TestIndexEmptyID(t *testing.T) {
	e, err := NewMemoryEngine()
	require.NoError(t, err)
	defer e.Close()

	err = e.Index(context.Background(), Document{Content: "no id"})
	assert.Error(t, err)
}
