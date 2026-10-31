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
