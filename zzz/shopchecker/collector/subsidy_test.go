package collector

import (
	"math"
	"testing"
)

func TestParseSubsidyHint(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantAmount float64
		wantRate   float64
		wantCap    float64
	}{
		{"京东腰带减500", "领国家补贴减￥500", 500, 0, 0},
		{"国补减500", "国补减500", 500, 0, 0},
		{"国补15%", "国家补贴15%", 0, 15, 0},
		{"比例优先不误判金额", "国家补贴 20 %", 0, 20, 0},
		{"比例+封顶", "国家补贴20%，最高减100", 100, 20, 100},
		{"补贴后价格不是金额", "国补后 3999 元", 0, 0, 0},
		{"无补贴文案", "京东价 4499", 0, 0, 0},
		{"空文本", "", 0, 0, 0},
	}
	for _, c := range cases {
		got := ParseSubsidyHint(c.text)
		if got.Amount != c.wantAmount || got.Rate != c.wantRate || got.Cap != c.wantCap {
			t.Errorf("%s: ParseSubsidyHint(%q)=%+v want amount=%v rate=%v cap=%v",
				c.name, c.text, got, c.wantAmount, c.wantRate, c.wantCap)
		}
	}
}

func TestComputeSubsidizedPrice(t *testing.T) {
	cases := []struct {
		name       string
		base       float64
		amount     float64
		rate       float64
		cap        float64
		wantFinal  float64
		wantAmount float64
	}{
		{"金额抵扣", 4499, 500, 0, 0, 3999, 500},
		{"比例抵扣", 4499, 0, 15, 0, 3824.15, 674.85},
		{"比例+封顶", 4499, 0, 20, 500, 3999, 500},
		{"金额优先于比例", 1000, 100, 20, 0, 900, 100},
		{"异常补贴超过90%拒绝", 100, 95, 0, 0, 0, 0},
		{"基础价无效", 0, 500, 0, 0, 0, 0},
		{"无补贴信息", 100, 0, 0, 0, 0, 0},
	}
	for _, c := range cases {
		final, amount := ComputeSubsidizedPrice(c.base, c.amount, c.rate, c.cap)
		if math.Abs(final-c.wantFinal) > 0.001 || math.Abs(amount-c.wantAmount) > 0.001 {
			t.Errorf("%s: got (%v,%v) want (%v,%v)", c.name, final, amount, c.wantFinal, c.wantAmount)
		}
	}
}

func TestExtractSubsidyContext(t *testing.T) {
	text := "商品标题 很长的描述文字。领国家补贴减￥500，国补到手价 3999 元。其他营销文案"
	got := ExtractSubsidyContext(text)
	if got == "" || !HasSubsidyHint(got) {
		t.Fatalf("ExtractSubsidyContext()=%q", got)
	}
	if hint := ParseSubsidyHint(got); hint.Amount != 500 {
		t.Errorf("hint=%+v want amount=500", hint)
	}
	if HasSubsidyHint("普通商品页") {
		t.Error("无补贴文本不应被判定为有国补")
	}
}
