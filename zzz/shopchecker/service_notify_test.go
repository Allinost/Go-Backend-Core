package shopchecker

import "testing"

func TestShouldNotifyTargetOnlyOnCrossing(t *testing.T) {
	svc := &Service{}
	cases := []struct {
		name                   string
		oldPrice, newPrice     float64
		targetPrice, dropRatio float64
		want                   bool
	}{
		{"首次达到目标价通知", 0, 95, 100, 0, true},
		{"刚跌破目标价通知", 110, 95, 100, 0, true},
		{"已在目标价下方且价格不变不重复通知", 90, 90, 100, 0, false},
		{"已在目标价下方继续降价仍通知", 90, 85, 100, 0, true},
		{"未达到目标价且未降价不通知", 120, 120, 100, 0, false},
		{"达到目标价但降幅不足仍通知", 110, 99, 100, 20, true},
	}
	for _, c := range cases {
		if got := svc.shouldNotify(c.oldPrice, c.newPrice, c.targetPrice, c.dropRatio); got != c.want {
			t.Errorf("%s: shouldNotify=%v want %v", c.name, got, c.want)
		}
	}
}

func TestIsSupportedPlatform(t *testing.T) {
	for _, p := range []string{"jd", "jdm", "qixian", "taobao", "tmall", "pdd"} {
		if !isSupportedPlatform(p) {
			t.Errorf("平台 %q 应受支持", p)
		}
	}
	if isSupportedPlatform("unknown") {
		t.Error("unknown 不应受支持")
	}
}
