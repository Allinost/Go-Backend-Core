package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOffset_Default(t *testing.T) {
	offset, limit := ParseOffset(0, 0)
	assert.Equal(t, 0, offset)
	assert.Equal(t, 20, limit)
}

func TestParseOffset_Normal(t *testing.T) {
	offset, limit := ParseOffset(3, 10)
	assert.Equal(t, 20, offset)
	assert.Equal(t, 10, limit)
}

func TestParseOffset_SizeClamped(t *testing.T) {
	_, limit := ParseOffset(1, 200)
	assert.Equal(t, 100, limit)
}

func TestParseOffset_NegativePage(t *testing.T) {
	offset, limit := ParseOffset(-1, 10)
	assert.Equal(t, 0, offset)
	assert.Equal(t, 10, limit)
}

func TestParsePage_Default(t *testing.T) {
	page, size := ParsePage(0, 0)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, size)
}

func TestParsePage_Normal(t *testing.T) {
	page, size := ParsePage(3, 15)
	assert.Equal(t, 3, page)
	assert.Equal(t, 15, size)
}

func TestParsePage_SizeClamped(t *testing.T) {
	_, size := ParsePage(1, 500)
	assert.Equal(t, 100, size)
}

func TestParseCursor_Default(t *testing.T) {
	limit := ParseCursor("", 0)
	assert.Equal(t, 20, limit)
}

func TestParseCursor_Normal(t *testing.T) {
	limit := ParseCursor("abc123", 30)
	assert.Equal(t, 30, limit)
}

func TestParseCursor_Clamped(t *testing.T) {
	limit := ParseCursor("abc", 200)
	assert.Equal(t, 100, limit)
}

func TestNewOffsetResult(t *testing.T) {
	items := []string{"a", "b"}
	result := NewOffsetResult(items, int64(50), 1, 20)
	assert.Equal(t, items, result.Items)
	assert.Equal(t, int64(50), result.Total)
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, 20, result.Size)
	assert.True(t, result.HasMore)
}

func TestNewOffsetResult_BeyondLastPage(t *testing.T) {
	items := []string{"a", "b"}
	result := NewOffsetResult(items, int64(10), 2, 10)
	assert.False(t, result.HasMore)
}

func TestNewOffsetResult_ExactEnd(t *testing.T) {
	items := []string{"a", "b"}
	result := NewOffsetResult(items, int64(20), 1, 20)
	assert.False(t, result.HasMore)
}

func TestNewOffsetResult_NoHasMore(t *testing.T) {
	items := []string{"a"}
	result := NewOffsetResult(items, int64(1), 1, 20)
	assert.False(t, result.HasMore)
}

func TestNewPageResult(t *testing.T) {
	items := []string{"a", "b"}
	result := NewPageResult(items, int64(50), 1, 20)
	assert.Equal(t, items, result.Items)
	assert.Equal(t, int64(50), result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.Size)
	assert.True(t, result.HasMore)
}

func TestNewPageResult_LastPage(t *testing.T) {
	items := []string{"a", "b"}
	result := NewPageResult(items, int64(50), 3, 20)
	assert.False(t, result.HasMore)
}

func TestNewCursorResult(t *testing.T) {
	items := []string{"a", "b"}
	result := NewCursorResult(items, "next123", 20, true)
	assert.Equal(t, items, result.Items)
	assert.Equal(t, "next123", result.NextCursor)
	assert.Equal(t, 20, result.Limit)
	assert.True(t, result.HasMore)
}

func TestNewCursorResult_NoMore(t *testing.T) {
	items := []string{"a"}
	result := NewCursorResult(items, "", 20, false)
	assert.Empty(t, result.NextCursor)
	assert.False(t, result.HasMore)
}
