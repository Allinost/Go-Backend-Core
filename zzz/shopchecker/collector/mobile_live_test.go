package collector

import (
	"math"
	"testing"
)

func TestParseMobileJdPrice(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`{"jdPrice":"15.90","extJdPrice":"15.90","skuName":"x"}`, 15.90},
		{`{"jdPrice":"0","extJdPrice":""}`, 0},
		{`{"jdPrice":"¥5.00","extJdPrice":""}`, 5.00},
		{`{"jdPrice":"","extJdPrice":"99.99"}`, 99.99},
		{`not json`, 0},
		{`{}`, 0},
	}
	for _, c := range cases {
		if got := parseMobileJdPrice(c.in); got != c.want {
			t.Errorf("parseMobileJdPrice(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestParseMobileEffectivePrice(t *testing.T) {
	cases := []struct {
		in       string
		wantP    float64
		wantOrig float64
	}{
		// 国补 + PLUS
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"领国家补贴减￥500","plusDiscountAmount":"22.49","purchaseSubType":"gov-not-claimed-1"}`, 3976.51, 4499},
		// 国补无 PLUS
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"领国家补贴减￥500"}`, 3999, 4499},
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"领国家补贴减 ¥500"}`, 3999, 4499},
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"国补减500"}`, 3999, 4499},
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"国家补贴15%"}`, 3824.15, 4499},
		{`{"jdPrice":"1000.00","extJdPrice":"1000.00","subsidyText":"国家补贴20%，最高减100"}`, 900, 1000},
		{`{"jdPrice":"4499.00","extJdPrice":"4499.00","subsidyText":"领国家补贴减￥500.50","plusDiscountAmount":"22.49"}`, 3976.01, 4499},
		// 无国补但 PLUS 优惠
		{`{"jdPrice":"15.90","extJdPrice":"15.90","plusDiscountAmount":"0.80"}`, 15.10, 15.90},
		{`{"jdPrice":"100.00","extJdPrice":"100.00","plusDiscountAmount":"2.00"}`, 98, 100},
		// 无国补无 PLUS
		{`{"jdPrice":"15.90","extJdPrice":"15.90","subsidyText":""}`, 0, 0},
		{`{"jdPrice":"15.90","extJdPrice":"15.90"}`, 0, 0},
		{`has no subsidy`, 0, 0},
		{`{}`, 0, 0},
	}
	for _, c := range cases {
		p, orig := parseMobileEffectivePrice(c.in)
		if math.Abs(p-c.wantP) > 0.0001 {
			t.Errorf("parseMobileEffectivePrice(%q) price=%v want %v", c.in, p, c.wantP)
		}
		if math.Abs(orig-c.wantOrig) > 0.0001 {
			t.Errorf("parseMobileEffectivePrice(%q) orig=%v want %v", c.in, orig, c.wantOrig)
		}
	}
}

func TestParseSubsidyText(t *testing.T) {
	cases := map[string]float64{
		"领国家补贴减￥500":    500,
		"领国家补贴减 ¥500":   500,
		"领国家补贴减￥500.50": 500.50,
		"补贴减￥1000":      1000,
		"领国家补贴":         0,
		"":              0,
	}
	for in, want := range cases {
		if got := parseSubsidyText(in); got != want {
			t.Errorf("parseSubsidyText(%q)=%v want %v", in, got, want)
		}
	}
}

func TestParseFloatStr(t *testing.T) {
	if parseFloatStr("4499.00") != 4499 {
		t.Error("4499 failed")
	}
	if parseFloatStr("¥15.9") != 15.9 {
		t.Error("¥15.9 failed")
	}
	if parseFloatStr("abc") != 0 {
		t.Error("abc should be 0")
	}
	if parseFloatStr("") != 0 {
		t.Error("empty should be 0")
	}
	if parseFloatStr("0") != 0 {
		t.Error("0 should be 0")
	}
}

func TestParseMobileSkuName(t *testing.T) {
	in := `{"jdPrice":"15.90","skuName":"七鲜 厚切炸猪排225g"}`
	if got := parseMobileSkuName(in); got != "七鲜 厚切炸猪排225g" {
		t.Errorf("got %q", got)
	}
	if got := parseMobileSkuName(`{"jdPrice":"15.90"}`); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

func TestParseMobileStock(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"stockState":33}`, "in_stock"},
		{`{"stockState":39}`, "in_stock"},
		{`{"stockState":40}`, "in_stock"},
		{`{"stockState":36}`, "on_order"},
		{`{"stockState":0}`, "unknown"},
		{`{"stockState":-1}`, "out_of_stock"},
		{`{}`, "unknown"},
		{`not json`, "unknown"},
	}
	for _, c := range cases {
		if got := parseMobileStock(c.in); got != c.want {
			t.Errorf("parseMobileStock(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
