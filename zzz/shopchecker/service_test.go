package shopchecker

import (
	"testing"
	"time"
)

func TestNormalizeCheckInterval(t *testing.T) {
	if got, err := normalizeCheckInterval(""); err != nil || got != "1h" {
		t.Errorf("空间隔应默认 1h，实际 %q / %v", got, err)
	}
	if got, err := normalizeCheckInterval(" 30m "); err != nil || got != "30m" {
		t.Errorf("30m 应通过并 trim，实际 %q / %v", got, err)
	}
	for _, raw := range []string{"0s", "-5m", "abc"} {
		if _, err := normalizeCheckInterval(raw); err == nil {
			t.Errorf("间隔 %q 应校验失败", raw)
		}
	}
}

func TestShouldNotify(t *testing.T) {
	svc := &Service{}
	cases := []struct {
		name                   string
		oldPrice, newPrice     float64
		targetPrice, dropRatio float64
		want                   bool
	}{
		{"首次达到目标价", 0, 100, 120, 0, true},
		{"达到目标价", 150, 99, 100, 0, true},
		{"价格下降达到比例", 100, 90, 0, 10, true},
		{"价格下降未达比例", 100, 95, 0, 10, false},
		{"价格未下降", 100, 100, 0, 10, false},
		{"无历史价格且未达目标", 0, 100, 50, 10, false},
		{"新价格非法", 100, 0, 0, 0, false},
	}
	for _, c := range cases {
		if got := svc.shouldNotify(c.oldPrice, c.newPrice, c.targetPrice, c.dropRatio); got != c.want {
			t.Errorf("%s：shouldNotify=%v，期望 %v", c.name, got, c.want)
		}
	}
}

func TestComputeNextCheck(t *testing.T) {
	svc := &Service{}
	from := time.Date(2026, 10, 30, 10, 0, 0, 0, time.Local)

	next := svc.computeNextCheck(from, "1h", "")
	lower := from.Add(51 * time.Minute)
	upper := from.Add(69 * time.Minute)
	if next.Before(lower) || next.After(upper) {
		t.Errorf("1h 抖动后应在 %v ~ %v，实际 %v", lower, upper, next)
	}

	fallback := svc.computeNextCheck(from, "bad-interval", "")
	if fallback.Before(lower) || fallback.After(upper) {
		t.Errorf("非法间隔应回退 1h，实际 %v", fallback)
	}

	cronNext := svc.computeNextCheck(from, "1h", "0 2 * * *")
	if cronNext.Hour() != sleepEndHour || !cronNext.After(from) {
		t.Errorf("cron 落在休眠时段应推到 9:00 之后，实际 %v", cronNext)
	}
}

func TestInSleepHours(t *testing.T) {
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local)
	cases := map[int]bool{0: false, 1: true, 8: true, 9: false, 12: false, 23: false}
	for hour, want := range cases {
		if got := inSleepHours(base.Add(time.Duration(hour) * time.Hour)); got != want {
			t.Errorf("%d 点：inSleepHours=%v，期望 %v", hour, got, want)
		}
	}
}

func TestIsLoginFailure(t *testing.T) {
	yes := []string{"需要登录", "登录已过期", "命中反爬拦截", "Passport 跳转", "PLogin required", "session 已失效"}
	for _, msg := range yes {
		if !isLoginFailure(msg) {
			t.Errorf("%q 应判定为登录失效", msg)
		}
	}
	no := []string{"", "网络超时", "商品下架", "价格解析失败"}
	for _, msg := range no {
		if isLoginFailure(msg) {
			t.Errorf("%q 不应判定为登录失效", msg)
		}
	}
}

func TestListProductsReqNormalize(t *testing.T) {
	r := &ListProductsReq{Page: 0, PageSize: 1000}
	r.Normalize()
	if r.Page != 1 || r.PageSize != 20 {
		t.Errorf("分页归一化异常：%+v", r)
	}
}
