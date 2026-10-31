package collector

import (
	"context"
	"testing"
	"time"
)

// 真实分享链接解析联调（需外网；go test -short 时跳过）。
func TestExampleLinksResolveLive(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过在线联调")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	jd := NewJDCollector()
	jdCases := []struct {
		name string
		in   string
		want string
	}{
		{"京东国补套装", "https://3.cn/39-HkHqJ?jkl=@JC6Rw0uTS15R@", "100086179820"},
		{"七鲜酸奶", "https://3.cn/39HkO-zr?jkl=@HBKQo8rzVzoA@", "100274527878"},
	}
	for _, c := range jdCases {
		if got := resolveSkuID(ctx, jd.client, c.in); got != c.want {
			t.Errorf("%s: resolveSkuID=%q want %q", c.name, got, c.want)
		}
	}

	tb := NewTaobaoCollector()
	tbCases := []struct {
		name string
		in   string
		want string
	}{
		{"淘宝 DELL U2", "https://e.tb.cn/h.jghkhvd3Y8ugBkM?tk=4oh0TA6EmBn", "14448135570"},
		{"天猫 5800X 套装", "https://e.tb.cn/h.jfEWxYST3TT5314?tk=ZI8nTAhZVyo", "1056234176069"},
	}
	for _, c := range tbCases {
		if got := tb.resolveItemID(ctx, c.in); got != c.want {
			t.Errorf("%s: resolveItemID=%q want %q", c.name, got, c.want)
		}
	}

	pdd := NewPDDCollector()
	if got := pdd.resolveGoodsID(ctx, "https://mobile.yangkeduo.com/goods1.html?ps=nFuVE9KiDb"); got != "1017448128382" {
		t.Errorf("拼多多 resolveGoodsID=%q want 1017448128382", got)
	}
}
