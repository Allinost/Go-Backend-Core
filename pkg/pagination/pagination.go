// Package pagination v1alpha1 分页参数解析、响应封装和查询构建。
// 提供 Offset/Page/Cursor 三种分页模式及对应的 SQL 查询构建器。
// 在 v1 之前不保证向后兼容。
package pagination

type Mode string

const (
	ModeOffset Mode = "offset"
	ModePage   Mode = "page"
	ModeCursor Mode = "cursor"
)

type Params struct {
	Mode   Mode
	Page   int
	Size   int
	Cursor string
	Limit  int
}

type Result struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total,omitempty"`
	Page       int         `json:"page,omitempty"`
	Size       int         `json:"size,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
	Cursor     string      `json:"cursor,omitempty"`
	NextCursor string      `json:"next_cursor,omitempty"`
	HasMore    bool        `json:"has_more"`
}

func ParseOffset(page, size int) (offset int, limit int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	} else if size > 100 {
		size = 100
	}
	return (page - 1) * size, size
}

func ParsePage(page, size int) (p int, s int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	} else if size > 100 {
		size = 100
	}
	return page, size
}

func ParseCursor(cursor string, limit int) (l int) {
	if limit < 1 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	return limit
}

func NewOffsetResult(items interface{}, total int64, page, size int) Result {
	offset, limit := ParseOffset(page, size)
	hasMore := int64(offset+limit) < total
	return Result{
		Items:   items,
		Total:   total,
		Offset:  offset,
		Size:    limit,
		HasMore: hasMore,
	}
}

func NewPageResult(items interface{}, total int64, page, size int) Result {
	p, s := ParsePage(page, size)
	hasMore := int64(p*s) < total
	return Result{
		Items:   items,
		Total:   total,
		Page:    p,
		Size:    s,
		HasMore: hasMore,
	}
}

func NewCursorResult(items interface{}, nextCursor string, limit int, hasMore bool) Result {
	l := ParseCursor(nextCursor, limit)
	return Result{
		Items:      items,
		NextCursor: nextCursor,
		Limit:      l,
		HasMore:    hasMore,
	}
}
