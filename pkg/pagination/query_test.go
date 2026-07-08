package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCursorQuery_Forward(t *testing.T) {
	clause := BuildCursorQuery(CursorQuery{
		Column: "id", Cursor: int64(100), Direction: Asc, Limit: 20,
	})
	assert.Equal(t, "id > ?", clause.Where)
	assert.Equal(t, []interface{}{int64(100)}, clause.Args)
	assert.Equal(t, "id ASC", clause.Order)
	assert.Equal(t, 20, clause.Limit)
}

func TestBuildCursorQuery_Backward(t *testing.T) {
	clause := BuildCursorQuery(CursorQuery{
		Column: "id", Cursor: int64(50), Direction: Desc, Limit: 10,
	})
	assert.Equal(t, "id < ?", clause.Where)
	assert.Equal(t, []interface{}{int64(50)}, clause.Args)
	assert.Equal(t, "id DESC", clause.Order)
	assert.Equal(t, 10, clause.Limit)
}

func TestBuildCursorQuery_NoCursor(t *testing.T) {
	clause := BuildCursorQuery(CursorQuery{
		Column: "id", Direction: Asc, Limit: 20,
	})
	assert.Empty(t, clause.Where)
	assert.Nil(t, clause.Args)
	assert.Equal(t, "id ASC", clause.Order)
	assert.Equal(t, 20, clause.Limit)
}

func TestBuildCursorQuery_StringCursor(t *testing.T) {
	clause := BuildCursorQuery(CursorQuery{
		Column: "name", Cursor: "abc", Direction: Asc, Limit: 15,
	})
	assert.Equal(t, "name > ?", clause.Where)
	assert.Equal(t, []interface{}{"abc"}, clause.Args)
	assert.Equal(t, "name ASC", clause.Order)
	assert.Equal(t, 15, clause.Limit)
}

func TestBuildCursorQuery_InvalidLimitDefaults(t *testing.T) {
	clause := BuildCursorQuery(CursorQuery{
		Column: "id", Direction: Asc, Limit: 0,
	})
	assert.Equal(t, 20, clause.Limit)
}

func TestBuildOffsetQuery(t *testing.T) {
	clause := BuildOffsetQuery(3, 10)
	assert.Equal(t, 10, clause.Limit)
	assert.Equal(t, 20, clause.Offset)
}

func TestBuildOffsetQuery_Default(t *testing.T) {
	clause := BuildOffsetQuery(0, 0)
	assert.Equal(t, 20, clause.Limit)
	assert.Equal(t, 0, clause.Offset)
}

func TestBuildPageQuery(t *testing.T) {
	clause := BuildPageQuery(3, 10)
	assert.Equal(t, 10, clause.Limit)
	assert.Equal(t, 20, clause.Offset)
}

func TestBuildPageQuery_FirstPage(t *testing.T) {
	clause := BuildPageQuery(1, 10)
	assert.Equal(t, 10, clause.Limit)
	assert.Equal(t, 0, clause.Offset)
}
