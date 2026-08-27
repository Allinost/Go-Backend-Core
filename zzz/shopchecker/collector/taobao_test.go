package collector

import (
	"context"
	"testing"
	"time"
)

func TestExtractTaobaoURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE", "https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE"},
		{"【淘宝】7天无理由退换 https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE HU071 「Apple/苹果 iPhone 17e」点击链接直接打开",
			"https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE"},
		{"点击链接 https://e.tb.cn/h.xxx?tk=yyy，或者淘宝搜索直接打开",
			"https://e.tb.cn/h.xxx?tk=yyy"},
		{"https://item.taobao.com/item.htm?id=1027760304544", "https://item.taobao.com/item.htm?id=1027760304544"},
		{"无链接文本", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractTaobaoURL(c.in); got != c.want {
			t.Errorf("extractTaobaoURL(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractTaobaoItemID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://item.taobao.com/item.htm?id=1027760304544&price=4499&sourceType=item", "1027760304544"},
		{"https://item.taobao.com/item.htm?spm=a2159r.13376460.0.0&id=923784928316&price=96.22", "923784928316"},
		{"https://main.m.taobao.com/detail/index.html?id=829135658048", "829135658048"},
		{"https://detail.tmall.com/item.htm?id=829135658048", "829135658048"},
		{"https://item.taobao.com/i234567.htm", "234567"},
		{"https://shop.taobao.com/shop/view_item.htm?id=123456789", "123456789"},
		{"https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE", ""},
		{"https://www.jd.com/product/100012043978.html", ""},
	}
	for _, c := range cases {
		if got := extractTaobaoItemID(c.in); got != c.want {
			t.Errorf("extractTaobaoItemID(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestIsTaobaoShortLink(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=x", true},
		{"https://m.tb.cn/h.5abcdef", true},
		{"https://tb.cn/xYzAbC", true},
		{"https://item.taobao.com/item.htm?id=1", false},
		{"https://detail.tmall.com/item.htm?id=1", false},
		{"not-a-url", false},
	}
	for _, c := range cases {
		if got := isTaobaoShortLink(c.in); got != c.want {
			t.Errorf("isTaobaoShortLink(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

// 真实 e.tb.cn 跳转页 HTML 片段（脱敏）
const tbShortLinkHTMLSample = `<html><head><script>
      	var bizCode="tbshare";
    	var shortName = "h.8Pz6TAsvVLBXFPH";
    	var url = 'https://item.taobao.com/item.htm?id=1027760304544&price=4499&sourceType=item&shareurl=true&short_name=h.8Pz6TAsvVLBXFPH';
    	var invalidUrl = "http://m.tb.cn/scanError.htm";
</script></head><body></body></html>`

func TestResolveShortLinkTargetRegex(t *testing.T) {
	m := tbShortTargetRe.FindStringSubmatch(tbShortLinkHTMLSample)
	if len(m) < 2 {
		t.Fatal("未匹配到 var url")
	}
	want := "https://item.taobao.com/item.htm?id=1027760304544&price=4499&sourceType=item&shareurl=true&short_name=h.8Pz6TAsvVLBXFPH"
	if m[1] != want {
		t.Errorf("target=%q want %q", m[1], want)
	}
	if id := extractTaobaoItemID(m[1]); id != "1027760304544" {
		t.Errorf("id=%q want 1027760304544", id)
	}
}

func TestParseTaobaoLabeledPrices(t *testing.T) {
	cases := []struct {
		name        string
		found       map[string]string
		wantSubsidy float64
		wantFinal   float64
		wantDaily   float64
	}{
		{"国补500后3999", map[string]string{"国补领后价": "3999", "日常价": "4499"}, 3999, 0, 4499},
		{"补贴后到手价", map[string]string{"补贴后到手价": "3999"}, 3999, 0, 0},
		{"百亿补贴价", map[string]string{"百亿补贴价": "84", "原价": "96.22"}, 84, 0, 96.22},
		{"仅券后", map[string]string{"券后": "14.9"}, 0, 14.9, 0},
		{"到手价优先于主价格", map[string]string{"到手价": "88", "券后": "90"}, 0, 88, 0},
		{"空标签", map[string]string{}, 0, 0, 0},
		{"nil标签", nil, 0, 0, 0},
	}
	for _, c := range cases {
		s, f, d := parseTaobaoLabeledPrices(c.found)
		if s != c.wantSubsidy || f != c.wantFinal || d != c.wantDaily {
			t.Errorf("%s: got (%v,%v,%v) want (%v,%v,%v)",
				c.name, s, f, d, c.wantSubsidy, c.wantFinal, c.wantDaily)
		}
	}
}

func TestParseTaobaoScrapeJSON(t *testing.T) {
	found, main, skuName, soldOut := parseTaobaoScrapeJSON(
		`{"found":{"国补领后价":"3999","日常价":"4499"},"main":"4499","skuName":"Apple/苹果 iPhone 17e","soldOut":false}`)
	if found == nil || len(found) != 2 {
		t.Errorf("found=%v", found)
	}
	if main != 4499 {
		t.Errorf("main=%v want 4499", main)
	}
	if skuName != "Apple/苹果 iPhone 17e" {
		t.Errorf("skuName=%q", skuName)
	}
	if soldOut {
		t.Error("soldOut should be false")
	}

	_, _, _, _ = parseTaobaoScrapeJSON(`not json`)
}

func TestCleanTaobaoTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"【淘宝】Apple/苹果 iPhone 17e-淘宝网", "【淘宝】Apple/苹果 iPhone 17e"},
		{"电竞桌卧室简易折叠桌台式电脑桌家用-天猫", "电竞桌卧室简易折叠桌台式电脑桌家用"},
		{"alice机械键盘用2.25U 2.75U分裂空格pbt原厂高度MOA人体工学白色-淘宝网", "alice机械键盘用2.25U 2.75U分裂空格pbt原厂高度MOA人体工学白色"},
		{"登录-淘宝网", ""},
		{"安全验证", ""},
		{"商品详情", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanTaobaoTitle(c.in); got != c.want {
			t.Errorf("cleanTaobaoTitle(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractTaobaoPrice(t *testing.T) {
	cases := []struct {
		name      string
		html      string
		wantPrice float64
		wantOrig  float64
	}{
		{"priceInfo JSON", `"priceInfo":{"price":"3999","priceOrig":"4499"}`, 3999, 4499},
		{"裸 price JSON", `data={"price":"14.90"}`, 14.9, 0},
		{"promotionPrice", `"promotionPrice":{"price":"84.00"},"priceInfo":{"price":"96.22"}`, 84, 96.22},
		{"DOM tb-rmb", `<em class="tb-rmb">14.90</em><del>19.90</del>`, 14.9, 19.9},
		{"无价格", `<html>没有价格</html>`, 0, 0},
		{"原价小于现价时忽略", `"price":"100","priceOrig":"50"`, 100, 0},
	}
	for _, c := range cases {
		p, o := extractTaobaoPrice(c.html)
		if p != c.wantPrice || o != c.wantOrig {
			t.Errorf("%s: got (%v,%v) want (%v,%v)", c.name, p, o, c.wantPrice, c.wantOrig)
		}
	}
}

func TestExtractTaobaoStock(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`<div>已售罄</div>`, "out_of_stock"},
		{`<div>该宝贝已下架</div>`, "out_of_stock"},
		{`<span>现货</span>`, "in_stock"},
		{`<html>x</html>`, "unknown"},
	}
	for _, c := range cases {
		if got := extractTaobaoStock(c.in); got != c.want {
			t.Errorf("extractTaobaoStock(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestIsTaobaoLoginRedirect(t *testing.T) {
	cases := []struct {
		url   string
		title string
		want  bool
	}{
		{"https://login.taobao.com/havanaone/login/login.htm", "登录", true},
		{"https://item.taobao.com/item.htm?id=1", "商品详情页", false},
		{"https://login.m.taobao.com/login.htm?redirectURL=https%3a%2f%2fh5api.m.taobao.com%2F_____tmd_____",
			"", true},
		{"https://main.m.taobao.com/detail/index.html?id=1", "", false},
		{"https://example.com/page", "安全验证", true},
	}
	for _, c := range cases {
		if got := isTaobaoLoginRedirect(c.url, c.title); got != c.want {
			t.Errorf("isTaobaoLoginRedirect(%q,%q)=%v want %v", c.url, c.title, got, c.want)
		}
	}
}

// TestTaobaoResolveItemIDLive 真实短链解析联调（需外网；go test -short 时跳过）
func TestTaobaoResolveItemIDLive(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过在线联调")
	}
	c := NewTaobaoCollector()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"iPhone17e 国补", "【淘宝】7天无理由退换 https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE HU071 「Apple/苹果 iPhone 17e」", "1027760304544"},
		{"电竞桌 平台补贴", "【淘宝】假一赔四 https://e.tb.cn/h.8O875mIqrkuHwEk?tk=N4z5T3ctPjS HU926 「电竞桌卧室简易折叠桌台式电脑桌家用碳纤维桌子工作台书桌办公桌」", "923784928316"},
		{"机械键盘 原价", "【淘宝】7天无理由退货 https://e.tb.cn/h.89uYzVvdQbw0WNB?tk=vYBJT3cFKsV tG-#22>lD 「alice机械键盘用2.25U 2.75U分裂空格pbt原厂高度MOA人体工学白色」", "829135658048"},
	}
	ctx := context.Background()
	for _, tc := range cases {
		if got := c.resolveItemID(ctx, tc.input); got != tc.want {
			t.Errorf("%s: resolveItemID=%q want %q", tc.name, got, tc.want)
		}
	}
}

// TestTaobaoFetchPriceLive 真实价格采集端到端联调（需外网与登录态；go test -short 时跳过）
func TestTaobaoFetchPriceLive(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过在线联调")
	}
	c := NewTaobaoCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	result, err := c.FetchPrice(ctx, "https://e.tb.cn/h.89uYzVvdQbw0WNB?tk=vYBJT3cFKsV")
	if err != nil {
		t.Logf("FetchPrice 返回错误（游客态预期为需要登录）: %v", err)
		return
	}
	t.Logf("采集结果: title=%q price=%v orig=%v stock=%s url=%s",
		result.Title, result.CurrentPrice, result.OriginalPrice, result.StockStatus, result.ProductURL)
	if result.CurrentPrice <= 0 {
		t.Error("价格应大于 0")
	}
}
