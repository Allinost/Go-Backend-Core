package collector

import "testing"

func TestExtractPDDURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://mobile.yangkeduo.com/goods1.html?ps=nFuVE9KiDb", "https://mobile.yangkeduo.com/goods1.html?ps=nFuVE9KiDb"},
		{"【拼多多】https://mobile.yangkeduo.com/goods1.html?ps=nFuVE9KiDb 点击打开", "https://mobile.yangkeduo.com/goods1.html?ps=nFuVE9KiDb"},
		{"https://www.jd.com", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractPDDURL(c.in); got != c.want {
			t.Errorf("extractPDDURL(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractPDDGoodsIDFromURL(t *testing.T) {
	re := pddGoodsIDRe.FindStringSubmatch("https://mobile.yangkeduo.com/goods1.html?goods_id=1017448128382&page_from=0")
	if len(re) < 2 || re[1] != "1017448128382" {
		t.Fatalf("goods_id 解析失败: %v", re)
	}
}

func TestNormalizePDDNumber(t *testing.T) {
	cases := []struct {
		in    string
		cents bool
		want  float64
	}{
		{"2790", true, 27.90},
		{"27.90", true, 27.90},
		{"99", true, 99},
		{"19.9", false, 19.9},
	}
	for _, c := range cases {
		if got := normalizePDDNumber(c.in, c.cents); got != c.want {
			t.Errorf("normalizePDDNumber(%q,%v)=%v want %v", c.in, c.cents, got, c.want)
		}
	}
}

func TestExtractPDDPriceCents(t *testing.T) {
	html := `{"minOnSaleGroupPrice":2790,"linePrice":3990,"goodsName":"测试商品"}`
	price, orig := extractPDDPrice(html)
	if price != 27.90 || orig != 39.90 {
		t.Errorf("extractPDDPrice()=(%v,%v) want (27.9,39.9)", price, orig)
	}
}
