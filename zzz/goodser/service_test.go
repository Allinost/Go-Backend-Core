package goodser

import "testing"

func TestResolveInboundQty(t *testing.T) {
	if got, err := resolveInboundQty(nil); err != nil || got != 1 {
		t.Errorf("缺省数量应为 1，实际 %d / %v", got, err)
	}
	five := 5
	if got, err := resolveInboundQty(&five); err != nil || got != 5 {
		t.Errorf("数量 5 应通过，实际 %d / %v", got, err)
	}
	for _, v := range []int{0, -1} {
		q := v
		_, err := resolveInboundQty(&q)
		if err == nil {
			t.Errorf("数量 %d 应校验失败", v)
			continue
		}
		if ge, ok := err.(*GoodserError); !ok || ge.Code != ErrInvalidQuantity {
			t.Errorf("数量 %d 应返回 ErrInvalidQuantity，实际 %v", v, err)
		}
	}
}

func TestImageKeyFromURL(t *testing.T) {
	cases := map[string]string{
		"": "",
		"https://rustfs.local/app-files/goodser/images/a.png?X-Amz-Signature=x": "goodser/images/a.png",
		"https://rustfs.local/app-files/other/a.png":                            "",
		"://bad-url": "",
	}
	for in, want := range cases {
		if got := imageKeyFromURL(in); got != want {
			t.Errorf("imageKeyFromURL(%q)=%q，期望 %q", in, got, want)
		}
	}
}

func TestQueryProductsReqNormalize(t *testing.T) {
	r := &QueryProductsReq{}
	r.Normalize()
	if r.Page != 1 || r.PageSize != 20 || r.SortOrder != "asc" {
		t.Errorf("缺省归一化异常：%+v", r)
	}
	r = &QueryProductsReq{Page: 3, PageSize: 500, SortOrder: "desc"}
	r.Normalize()
	if r.Page != 3 || r.PageSize != 20 || r.SortOrder != "desc" {
		t.Errorf("边界归一化异常：%+v", r)
	}
}

func TestPaginatedReqOffset(t *testing.T) {
	r := &PaginatedReq{Page: 3, PageSize: 20}
	if got := r.Offset(); got != 40 {
		t.Errorf("Offset 应为 40，实际 %d", got)
	}
}
