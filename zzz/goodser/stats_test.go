package goodser

import (
	"testing"
	"time"
)

func TestNormalizeStatsDays(t *testing.T) {
	cases := map[int]int{
		0:   30,
		-3:  30,
		1:   1,
		30:  30,
		365: 365,
		999: 365,
	}
	for in, want := range cases {
		if got := NormalizeStatsDays(in); got != want {
			t.Errorf("NormalizeStatsDays(%d)=%d，期望 %d", in, got, want)
		}
	}
}

func TestFillTrend(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	points := []StatsTrendPoint{
		{Date: "2026-11-01", OutboundQuantity: 3, ReturnQuantity: 1},
		{Date: "2026-11-03", OutboundQuantity: 2},
	}
	got := fillTrend(points, start, 3)
	if len(got) != 3 {
		t.Fatalf("趋势应补齐 3 天，实际 %d", len(got))
	}
	if got[0].OutboundQuantity != 3 || got[0].ReturnQuantity != 1 {
		t.Errorf("第 1 天数据异常：%+v", got[0])
	}
	if got[1].Date != "2026-11-02" || got[1].OutboundQuantity != 0 {
		t.Errorf("第 2 天应补零：%+v", got[1])
	}
	if got[2].Date != "2026-11-03" || got[2].OutboundQuantity != 2 {
		t.Errorf("第 3 天数据异常：%+v", got[2])
	}
}
