package pagination

import "fmt"

type Order string

const (
	Asc  Order = "ASC"
	Desc Order = "DESC"
)

type CursorQuery struct {
	Column    string
	Cursor    interface{}
	Direction Order
	Limit     int
}

type SQLClause struct {
	Where  string
	Args   []interface{}
	Order  string
	Limit  int
	Offset int
}

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

func BuildOffsetQuery(page, size int) SQLClause {
	offset, limit := ParseOffset(page, size)
	return SQLClause{
		Limit:  limit,
		Offset: offset,
	}
}

func BuildPageQuery(page, size int) SQLClause {
	p, s := ParsePage(page, size)
	return SQLClause{
		Limit:  s,
		Offset: (p - 1) * s,
	}
}
