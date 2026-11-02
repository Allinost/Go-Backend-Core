package collector

import "testing"

func TestBuildTaobaoCartIndex(t *testing.T) {
	raw := []tbCartRaw{
		{ItemID: "14448135570", SKUID: "6105919976363", Quantity: 1, PayTotal: 87000, Valid: true, Title: "P4800X 375G"},
		{ItemID: "14448135570", SKUID: "5860984206140", Quantity: 1, PayTotal: 169000, PayOrigin: 165000, Valid: true, Title: "P4800X 750G"},
		{ItemID: "1056234176069", SKUID: "6150708661971", Quantity: 1, PayTotal: 230000, PayOrigin: 500000, Valid: true, Title: "B550M 套装"},
		{ItemID: "999", SKUID: "1", Quantity: 3, PayTotal: 30000, Valid: true, Title: "三件装"},
		{ItemID: "888", SKUID: "2", Quantity: 1, PayTotal: 10000, Valid: false, Title: "失效商品"},
	}

	byKey, byItem := buildTaobaoCartIndex(raw)

	if got := byKey["14448135570|6105919976363"]; got == nil || got.CurrentPrice != 870 {
		t.Fatalf("exact sku price = %+v, want 870", got)
	}
	if got := byKey["14448135570|5860984206140"]; got == nil || got.CurrentPrice != 1690 {
		t.Fatalf("second sku price = %+v, want 1690", got)
	}
	if got := byKey["1056234176069|6150708661971"]; got == nil || got.CurrentPrice != 2300 || got.OriginalPrice != 5000 {
		t.Fatalf("tmall price = %+v, want 2300/5000", got)
	}
	if got := byKey["999|1"]; got == nil || got.CurrentPrice != 100 {
		t.Fatalf("quantity price = %+v, want 100", got)
	}
	if got := byKey["888|2"]; got == nil || got.StockStatus != "out_of_stock" {
		t.Fatalf("invalid item stock = %+v, want out_of_stock", got)
	}
	if len(byItem["14448135570"]) != 2 {
		t.Fatalf("byItem entries = %d, want 2", len(byItem["14448135570"]))
	}
	if byKey["14448135570|6105919976363"].PriceSource != "taobao_cart" || byKey["14448135570|6105919976363"].CartStatus != "hit" {
		t.Errorf("unexpected source/status: %+v", byKey["14448135570|6105919976363"])
	}
}

func TestMatchTaobaoCartItem(t *testing.T) {
	byKey, byItem := buildTaobaoCartIndex([]tbCartRaw{
		{ItemID: "1", SKUID: "11", Quantity: 1, PayTotal: 169000, Valid: true},
		{ItemID: "1", SKUID: "12", Quantity: 1, PayTotal: 87000, Valid: true},
		{ItemID: "2", SKUID: "21", Quantity: 1, PayTotal: 50000, Valid: false},
		{ItemID: "2", SKUID: "22", Quantity: 1, PayTotal: 60000, Valid: true},
		{ItemID: "3", SKUID: "31", Quantity: 1, PayTotal: 30000, Valid: true},
	})

	if got := matchTaobaoCartItem(byKey, byItem, "1", "11"); got == nil || got.CurrentPrice != 1690 {
		t.Fatalf("exact match = %+v, want 1690", got)
	}
	if got := matchTaobaoCartItem(byKey, byItem, "1", "99"); got != nil {
		t.Fatalf("unknown sku should miss, got %+v", got)
	}
	if got := matchTaobaoCartItem(byKey, byItem, "3", ""); got == nil || got.CurrentPrice != 300 {
		t.Fatalf("single item match = %+v, want 300", got)
	}
	got := matchTaobaoCartItem(byKey, byItem, "1", "")
	if got == nil || got.CurrentPrice != 870 {
		t.Fatalf("multi sku should pick cheapest, got %+v", got)
	}
	if got.CartNote == "" {
		t.Error("multi sku match should record a note")
	}
	if got := matchTaobaoCartItem(byKey, byItem, "2", ""); got == nil {
		t.Fatal("item 2 should match")
	}
	preferred := matchTaobaoCartItem(byKey, byItem, "2", "")
	if preferred == nil || preferred.CurrentPrice != 600 {
		t.Fatalf("multi sku should prefer in-stock, got %+v", preferred)
	}
}

func TestParseTaobaoAddBagResponse(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"success", `{"ret":["SUCCESS::调用成功"]}`, false},
		{"failure ret", `{"ret":["FAIL_BIZ_CART_LIMIT::购物车已满"]}`, true},
		{"error message", `{"errorMessage":"库存不足"}`, true},
		{"empty", `{}`, true},
		{"invalid json", `not-json`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := parseTaobaoAddBagResponse(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseTaobaoAddBagResponse(%q) err=%v wantErr=%v", tc.raw, err, tc.wantErr)
			}
		})
	}
}

func TestExtractTaobaoSkuProperties(t *testing.T) {
	props := extractTaobaoSkuProperties("https://item.taobao.com/item.htm?id=14448135570&sku_properties=20122%3A118105;1627207%3A44168284707")
	if len(props) != 2 || props["20122"] != "118105" || props["1627207"] != "44168284707" {
		t.Fatalf("props = %#v", props)
	}
	if got := extractTaobaoSkuProperties("https://item.taobao.com/item.htm?id=1"); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestMatchTaobaoSkuFromHTML(t *testing.T) {
	html := `"skus":[{"propPath":"20122:118105;1627207:44168284706","skuId":"5860984206139"},` +
		`{"propPath":"20122:118105;1627207:44168284707","skuId":"6105919976363"}]`

	if got := matchTaobaoSkuFromHTML(html, map[string]string{"20122": "118105", "1627207": "44168284707"}); got != "6105919976363" {
		t.Fatalf("unique match = %q, want 6105919976363", got)
	}
	if got := matchTaobaoSkuFromHTML(html, map[string]string{"20122": "118105"}); got != "" {
		t.Fatalf("ambiguous match = %q, want empty", got)
	}
	if got := matchTaobaoSkuFromHTML(html, map[string]string{"20122": "none"}); got != "" {
		t.Fatalf("no match = %q, want empty", got)
	}
}

func TestCacheTaobaoCartItem(t *testing.T) {
	tbCartCacheMu.Lock()
	old := tbCartCache
	byKey, byItem := buildTaobaoCartIndex(nil)
	tbCartCache = taobaoCartSnapshot{ByKey: byKey, ByItem: byItem}
	tbCartCacheMu.Unlock()
	t.Cleanup(func() {
		tbCartCacheMu.Lock()
		tbCartCache = old
		tbCartCacheMu.Unlock()
	})

	cacheTaobaoCartItem("14448135570", "6105919976363", &PriceResult{
		CurrentPrice: 870,
		CartStatus:   "not_in_cart",
		CartNote:     "购物车列表无此商品，已回退商品页；已加入购物车",
	})

	tbCartCacheMu.Lock()
	item := tbCartCache.ByKey["14448135570|6105919976363"]
	tbCartCacheMu.Unlock()
	if item == nil {
		t.Fatal("cached item not found")
	}
	if item.CartStatus != "hit" || item.CartNote != "已加入购物车" {
		t.Errorf("unexpected cached item: %+v", item)
	}
}
