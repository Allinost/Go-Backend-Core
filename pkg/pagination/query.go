package pagination

import "fmt"

// Order 排序方向类型
type Order string

const (
	Asc  Order = "ASC"  // 升序
	Desc Order = "DESC" // 降序
)

// CursorQuery 游标查询参数
type CursorQuery struct {
	Column    string      // 游标列名
	Cursor    interface{} // 游标值
	Direction Order       // 排序方向
	Limit     int         // 限制条数
}

// SQLClause SQL 查询子句，包含 WHERE、ORDER BY、LIMIT 和 OFFSET
type SQLClause struct {
	Where  string        // WHERE 条件
	Args   []interface{} // 参数绑定
	Order  string        // ORDER BY 子句
	Limit  int           // LIMIT 限制
	Offset int           // OFFSET 偏移量
}

// BuildCursorQuery 构建基于游标的分页 SQL 子句
func BuildCursorQuery(q CursorQuery) SQLClause {
	limit := ParseCursor("", q.Limit)
	op := ">"
	order := Asc
	if q.Direction == Desc {
		op = "<"
		order = Desc
	}
	where := ""
	var args []interface{}
	if q.Cursor != nil {
		where = fmt.Sprintf("%s %s ?", q.Column, op)
		args = []interface{}{q.Cursor}
	}
	return SQLClause{
		Where: where,
		Args:  args,
		Order: fmt.Sprintf("%s %s", q.Column, order),
		Limit: limit,
	}
}

// BuildOffsetQuery 构建基于偏移量的分页 SQL 子句
func BuildOffsetQuery(page, size int) SQLClause {
	offset, limit := ParseOffset(page, size)
	return SQLClause{
		Limit:  limit,
		Offset: offset,
	}
}

// BuildPageQuery 构建基于页码的分页 SQL 子句
func BuildPageQuery(page, size int) SQLClause {
	p, s := ParsePage(page, size)
	return SQLClause{
		Limit:  s,
		Offset: (p - 1) * s,
	}
}
