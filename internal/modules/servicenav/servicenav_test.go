package servicenav

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSort_Valid(t *testing.T) {
	cases := []struct {
		sortBy    string
		sortOrder string
		wantBy    string
		wantOrder string
	}{
		{"name", "asc", "name", "ASC"},
		{"name", "desc", "name", "DESC"},
		{"created_at", "asc", "created_at", "ASC"},
		{"created_at", "desc", "created_at", "DESC"},
	}
	for _, c := range cases {
		gotBy, gotOrder := sanitizeSort(c.sortBy, c.sortOrder)
		assert.Equal(t, c.wantBy, gotBy, "sortBy")
		assert.Equal(t, c.wantOrder, gotOrder, "sortOrder")
	}
}

func TestSanitizeSort_FallbackToNameAsc(t *testing.T) {
	// 非法字段回退到 name，方向合法则保留
	gotBy, gotOrder := sanitizeSort("name; DROP TABLE zzz_servicenav_item", "desc")
	assert.Equal(t, "name", gotBy)
	assert.Equal(t, "DESC", gotOrder)

	gotBy, gotOrder = sanitizeSort("", "")
	assert.Equal(t, "name", gotBy)
	assert.Equal(t, "ASC", gotOrder)

	gotBy, gotOrder = sanitizeSort("created_at", "RANDOM")
	assert.Equal(t, "created_at", gotBy)
	assert.Equal(t, "ASC", gotOrder)
}

func TestListItemsReq_Normalize(t *testing.T) {
	req := &ListItemsReq{}
	req.Normalize()
	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.PageSize)
	assert.Equal(t, "name", req.SortBy)
	assert.Equal(t, "asc", req.SortOrder)
	assert.Equal(t, 0, req.Offset())

	req = &ListItemsReq{Page: 3, PageSize: 50, SortBy: "created_at", SortOrder: "desc"}
	req.Normalize()
	assert.Equal(t, 3, req.Page)
	assert.Equal(t, 50, req.PageSize)
	assert.Equal(t, 100, req.Offset())
}

func TestNewService_NotNil(t *testing.T) {
	svc := NewService(nil)
	assert.NotNil(t, svc)
}

func TestNewHandler_NotNil(t *testing.T) {
	h := NewHandler(nil)
	assert.NotNil(t, h)
}
