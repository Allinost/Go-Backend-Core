package ia

import (
	"math"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/zzz/ia/quote"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeScores(t *testing.T) {
	got := normalizeScores([]float64{10, 20, 30})
	assert.InDeltaSlice(t, []float64{0, 0.5, 1}, got, 1e-9)
	// 单值序列全零
	assert.Equal(t, []float64{0}, normalizeScores([]float64{42}))
	assert.Empty(t, normalizeScores(nil))
}

func TestReverseNormAndClamp01(t *testing.T) {
	assert.InDeltaSlice(t, []float64{1, 0.5, 0}, reverseNorm([]float64{0, 0.5, 1}), 1e-9)
	assert.Equal(t, 0.0, clamp01(-1))
	assert.Equal(t, 1.0, clamp01(2))
	assert.Equal(t, 0.5, clamp01(0.5))
}

func TestPercentileInRange(t *testing.T) {
	assert.InDelta(t, 0.75, percentileInRange(100, 200, 175), 1e-9)
	assert.InDelta(t, 0.5, percentileInRange(100, 100, 150), 1e-9) // 退化区间
	assert.InDelta(t, 0.0, percentileInRange(100, 200, 50), 1e-9)  // 越界截断
	assert.InDelta(t, 1.0, percentileInRange(100, 200, 300), 1e-9)
}

func TestMovingAverageAndReturnPct(t *testing.T) {
	closes := []float64{10, 11, 12, 13, 14}
	assert.InDelta(t, 13.0, movingAverage(closes, 3), 1e-9, "最近3个均值=(12+13+14)/3")
	assert.InDelta(t, 12.0, movingAverage(closes, 99), 1e-9, "不足取全量")
	assert.InDelta(t, (14.0/10-1)*100, returnPct(closes, 4), 1e-9)
	assert.Equal(t, 0.0, returnPct(closes, 10), "样本不足")
}

func TestSecidOfRow(t *testing.T) {
	assert.Equal(t, "1.600519", secidOfRow(quote.ScreenRow{Code: "600519"}))
	assert.Equal(t, "1.688981", secidOfRow(quote.ScreenRow{Code: "688981"}), "科创板归沪市")
	assert.Equal(t, "0.000858", secidOfRow(quote.ScreenRow{Code: "000858"}))
	assert.Equal(t, "0.300750", secidOfRow(quote.ScreenRow{Code: "300750"}))
}

func TestAnalyzeIndex(t *testing.T) {
	// 构造 260 日线性上升序列：10 → 269
	points := make([]quote.KLinePoint, 0, 260)
	base := time.Date(2025, 8, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 260; i++ {
		points = append(points, quote.KLinePoint{
			Date:  base.AddDate(0, 0, i).Format(dateLayout),
			Close: float64(i + 10),
		})
	}
	m := analyzeIndex("1.TEST", points)
	assert.Equal(t, 269.0, m.Latest)
	assert.True(t, m.AboveMA20 && m.AboveMA60 && m.AboveMA120 && m.AboveMA250)
	assert.Equal(t, "多头排列", m.Trend)
	assert.InDelta(t, 1.0, m.Percentile, 1e-9, "单边上涨最新价即最高点")
	assert.Positive(t, m.AnnReturn)
	assert.Equal(t, 0.0, m.MaxDrawdown, "单边上涨无回撤")
	assert.Equal(t, 260, m.Samples)
}

func TestAnalyzeIndex_Downtrend(t *testing.T) {
	points := make([]quote.KLinePoint, 0, 60)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 60; i++ {
		points = append(points, quote.KLinePoint{
			Date:  base.AddDate(0, 0, i).Format(dateLayout),
			Close: float64(200 - i),
		})
	}
	m := analyzeIndex("1.DOWN", points)
	assert.False(t, m.AboveMA20 && m.AboveMA60)
	assert.Equal(t, "空头排列", m.Trend)
	assert.InDelta(t, 0.0, m.Percentile, 1e-9)
}

func TestTruncateStr(t *testing.T) {
	assert.Equal(t, "abc", truncateStr("abc", 10))
	got := truncateStr("abcdefghij", 4)
	assert.Equal(t, "abcd...", got)
}

func TestAnnualizedReturnFromKline(t *testing.T) {
	// 两年翻倍 → 年化 ≈ 41.42%
	start := time.Date(2024, 8, 26, 0, 0, 0, 0, time.Local)
	points := []quote.KLinePoint{
		{Date: start.Format(dateLayout), Close: 100},
		{Date: start.AddDate(2, 0, 0).Format(dateLayout), Close: 200},
	}
	r := annualizedReturn(points)
	assert.False(t, math.IsNaN(r))
	assert.InDelta(t, math.Sqrt(2)-1, r, 0.001)
}
