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

func TestResolveOrderType(t *testing.T) {
	cases := []struct {
		in      *string
		want    OrderType
		wantErr bool
	}{
		{nil, OrderTypeOutbound, false},
		{strPtr(""), OrderTypeOutbound, false},
		{strPtr("outbound"), OrderTypeOutbound, false},
		{strPtr("reserve"), OrderTypeReserve, false},
		{strPtr("unknown"), "", true},
	}
	for _, c := range cases {
		got, err := resolveOrderType(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveOrderType(%v) 应失败", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("resolveOrderType(%v)=%v/%v，期望 %v", c.in, got, err, c.want)
		}
	}
}

func TestResolveReturnReason(t *testing.T) {
	cases := map[string]ReturnReason{
		"":                ReturnReasonReturnRefund,
		"refund_only":     ReturnReasonRefundOnly,
		"return_refund":   ReturnReasonReturnRefund,
		"shipping_damage": ReturnReasonShippingDamage,
		"other":           ReturnReasonOther,
	}
	for in, want := range cases {
		got, err := resolveReturnReason(in)
		if err != nil || got != want {
			t.Errorf("resolveReturnReason(%q)=%v/%v，期望 %v", in, got, err, want)
		}
	}
	if _, err := resolveReturnReason("bad"); err == nil {
		t.Error("非法退货原因应失败")
	}
}

func TestProductRecalcTotal(t *testing.T) {
	p := &Product{Quantity: 3, ReservedQuantity: 2}
	p.recalcTotal()
	if p.TotalQuantity != 5 {
		t.Errorf("TotalQuantity 应为 5，实际 %d", p.TotalQuantity)
	}
}

func TestGenerateOrderNo(t *testing.T) {
	out := generateOrderNo(OrderTypeOutbound)
	if len(out) < 10 || out[:3] != "OUT" {
		t.Errorf("出库单号格式异常: %s", out)
	}
	rsv := generateOrderNo(OrderTypeReserve)
	if len(rsv) < 10 || rsv[:3] != "RSV" {
		t.Errorf("预留单号格式异常: %s", rsv)
	}
	if out == rsv {
		t.Error("出库单号与预留单号不应相同")
	}
}
