package collector

import "testing"

func TestExtractTaobaoSKU(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://item.taobao.com/item.htm?id=1056234176069&skuId=5939384915119", "5939384915119"},
		{"https://detail.tmall.com/item.htm?id=1&sku_id=222", "222"},
		{"https://item.taobao.com/item.htm?id=1056234176069", ""},
		{"https://e.tb.cn/h.jfEWxYST3TT5314?tk=ZI8nTAhZVyo", ""},
	}
	for _, c := range cases {
		if got := extractTaobaoSKU(c.in); got != c.want {
			t.Errorf("extractTaobaoSKU(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestTbItemURLWithSKU(t *testing.T) {
	got := tbItemURL("1056234176069", "5939384915119", true)
	want := "https://main.m.taobao.com/detail/index.html?id=1056234176069&skuId=5939384915119"
	if got != want {
		t.Errorf("tbItemURL()=%q want %q", got, want)
	}
}
