package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJDCartItemPrice(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantPrice  float64
		wantOrig   float64
		wantAmount float64
	}{
		{
			name:      "price before label",
			text:      "七鲜 香草酸奶 ¥5.21 到手价 ¥7.90 京东价",
			wantPrice: 5.21,
			wantOrig:  7.90,
		},
		{
			name:      "price after label",
			text:      "七鲜 香草酸奶 到手价 ¥5.21 京东价 ¥7.90",
			wantPrice: 5.21,
			wantOrig:  7.90,
		},
		{
			name:      "no label fallback",
			text:      "七鲜 香草酸奶 ¥5.21",
			wantPrice: 5.21,
		},
		{
			name:       "subsidy rate from cart text",
			text:       "华硕套装 ¥1805.26 到手价 ¥2127.00 京东价 国补15%",
			wantPrice:  1805.26,
			wantOrig:   2127.00,
			wantAmount: 321.74,
		},
		{
			name:      "bundle price label and unlabeled original",
			text:      "套装【主板cpu套装】华硕 B550M 重炮手 ¥2019.12套装到手价 ¥2128 已享满200减10等优惠108.88 删除",
			wantPrice: 2019.12,
			wantOrig:  2128,
		},
		{
			name:      "subsidy price label",
			text:      "美商海盗船 32GB ¥795.26补贴价 ¥799 已享0.01元等优惠3.48 删除",
			wantPrice: 795.26,
			wantOrig:  799,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, orig, amount, _ := parseJDCartItemPrice(tc.text)
			if price != tc.wantPrice {
				t.Errorf("price = %v, want %v", price, tc.wantPrice)
			}
			if orig != tc.wantOrig {
				t.Errorf("orig = %v, want %v", orig, tc.wantOrig)
			}
			if tc.wantAmount > 0 && amount != tc.wantAmount {
				t.Errorf("amount = %v, want %v", amount, tc.wantAmount)
			}
		})
	}
}

func TestCartStockStatus(t *testing.T) {
	cases := map[string]string{
		"¥5.42到手价 ¥7.9 删除": "in_stock",
		"商品已下架 暂不支持购买 删除":  "out_of_stock",
		"¥10 到手价 无货 删除":    "out_of_stock",
	}
	for text, want := range cases {
		if got := cartStockStatus(text); got != want {
			t.Errorf("cartStockStatus(%q)=%q want %q", text, got, want)
		}
	}
}

func TestCartItemTitleFromText(t *testing.T) {
	text := "七鲜 香草酸奶100g*3 7天价保 获取AI搭配 ¥5.42到手价 ¥7.9 删除"
	if got := cartItemTitleFromText(text); got != "七鲜 香草酸奶100g*3" {
		t.Errorf("cartItemTitleFromText=%q", got)
	}
}

func TestIsJDRateLimitText(t *testing.T) {
	for _, text := range []string{"抱歉由于访问频繁导致无法搜索", "刷新太频繁", "pc-frequent-pro"} {
		if !isJDRateLimitText(text) {
			t.Errorf("%q should be rate limited", text)
		}
	}
	if isJDRateLimitText("商品已成功加入购物车") {
		t.Error("normal cart text should not be rate limited")
	}
}

func TestBrowserProfileDirStable(t *testing.T) {
	base := t.TempDir()
	t.Setenv("SHOPCHECKER_BROWSER_PROFILE", "")
	t.Setenv("SHOPCHECKER_BROWSER_PROFILE_DIR", base)

	first, err := browserProfileDir("jd")
	if err != nil {
		t.Fatal(err)
	}
	second, err := browserProfileDir("jd")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("profile dir should be stable: %q != %q", first, second)
	}
	if filepath.Dir(first) != base {
		t.Errorf("profile dir %q should be under %q", first, base)
	}
	if _, err := os.Stat(first); err != nil {
		t.Errorf("profile dir should exist: %v", err)
	}
}

func TestBrowserProfileDirAliases(t *testing.T) {
	base := t.TempDir()
	t.Setenv("SHOPCHECKER_BROWSER_PROFILE", "")
	t.Setenv("SHOPCHECKER_BROWSER_PROFILE_DIR", base)

	for _, pair := range [][2]string{
		{"jd", "qixian"},
		{"jd", "jdm"},
		{"taobao", "tmall"},
	} {
		a, err := browserProfileDir(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := browserProfileDir(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Errorf("%s and %s should share profile: %q != %q", pair[0], pair[1], a, b)
		}
	}
}

func TestBrowserProfileDirTempMode(t *testing.T) {
	t.Setenv("SHOPCHECKER_BROWSER_PROFILE", "temp")
	dir, err := browserProfileDir("jd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if filepath.Base(filepath.Dir(dir)) == "" {
		t.Fatal("temp dir should be created")
	}
}
