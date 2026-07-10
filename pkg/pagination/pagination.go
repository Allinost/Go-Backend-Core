// Package pagination v1alpha1 分页参数解析、响应封装和查询构建。
// 提供 Offset/Page/Cursor 三种分页模式及对应的 SQL 查询构建器。
// 在 v1 之前不保证向后兼容。
package pagination

// Mode 分页模式类型
type Mode string

const (
	ModeOffset Mode = "offset" // 偏移量分页
	ModePage   Mode = "page"   // 页码分页
	ModeCursor Mode = "cursor" // 游标分页
)

// Params 分页请求参数
type Params struct {
	Mode   Mode   // 分页模式
	Page   int    // 页码（Page 模式）
	Size   int    // 每页大小
	Cursor string // 游标值（Cursor 模式）
	Limit  int    // 限制条数（Cursor 模式）
}

// Result 分页响应结果
type Result struct {
	Items      interface{} `json:"items"`                 // 数据列表
	Total      int64       `json:"total,omitempty"`       // 总记录数
	Page       int         `json:"page,omitempty"`        // 当前页码
	Size       int         `json:"size,omitempty"`        // 每页大小
	Limit      int         `json:"limit,omitempty"`       // 限制条数
	Offset     int         `json:"offset,omitempty"`      // 偏移量
	Cursor     string      `json:"cursor,omitempty"`      // 当前游标
	NextCursor string      `json:"next_cursor,omitempty"` // 下一页游标
	HasMore    bool        `json:"has_more"`              // 是否还有更多数据
}

// ParseOffset 解析页码和大小，返回 offset 偏移量和 limit 限制数
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

// ParsePage 解析并校验页码和每页大小
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

// ParseCursor 解析并校验游标分页的 limit 值
func ParseCursor(cursor string, limit int) (l int) {
	if limit < 1 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	return limit
}

// NewOffsetResult 构建偏移量分页响应
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

// NewPageResult 构建页码分页响应
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

// NewCursorResult 构建游标分页响应
func NewCursorResult(items interface{}, nextCursor string, limit int, hasMore bool) Result {
	l := ParseCursor(nextCursor, limit)
	return Result{
		Items:      items,
		NextCursor: nextCursor,
		Limit:      l,
		HasMore:    hasMore,
	}
}
