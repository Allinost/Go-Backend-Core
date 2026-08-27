package ia

import (
	"math"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/zzz/ia/quote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.Local)
}

func TestSolveXIRR_ClassicCase(t *testing.T) {
	// 经典案例：-1000 投入，12 个月后收回 1200 → XIRR ≈ 20%
	flows := []DatedFlow{
		{Date: d(2025, 1, 1), Amount: -1000},
		{Date: d(2026, 1, 1), Amount: 1200},
	}
	x := SolveXIRR(flows)
	require.NotNil(t, x)
	assert.InDelta(t, 0.20, *x, 0.001)
}

func TestSolveXIRR_MultiFlow(t *testing.T) {
	// 定投两笔后回收：验证解存在且 NPV≈0
	flows := []DatedFlow{
		{Date: d(2024, 1, 1), Amount: -10000},
		{Date: d(2024, 7, 1), Amount: -5000},
		{Date: d(2026, 8, 26), Amount: 18000},
	}
	x := SolveXIRR(flows)
	require.NotNil(t, x)
	assert.False(t, math.IsNaN(*x))
	assert.Greater(t, *x, -0.9999)
	// 验证 NPV 收敛
	base := flows[0].Date
	assert.InDelta(t, 0, xirrNPV(flows, base, *x), 1e-6)
}

func TestSolveXIRR_InsufficientData(t *testing.T) {
	// 同号现金流无解
	allNeg := []DatedFlow{{Date: d(2026, 1, 1), Amount: -100}, {Date: d(2026, 2, 1), Amount: -50}}
	assert.Nil(t, SolveXIRR(allNeg))
	assert.Nil(t, SolveXIRR(nil))
}

func TestSimplePeriodReturn(t *testing.T) {
	r := SimplePeriodReturn(1000, 1150, 50)
	require.NotNil(t, r)
	assert.InDelta(t, 0.10, *r, 1e-9)
	assert.Nil(t, SimplePeriodReturn(0, 100, 0))
}

func TestModifiedDietz(t *testing.T) {
	start := d(2026, 1, 1)
	end := d(2026, 1, 31)
	// V0=1000，中途第10天追加500，期末1600
	flows := []timedFlow{{Date: d(2026, 1, 11), Amount: 500}}
	r := ModifiedDietz(1000, 1600, flows, start, end)
	require.NotNil(t, r)
	// w = (30-10)/30；denom = 1000 + 500*2/3
	expected := (1600.0 - 1000.0 - 500.0) / (1000.0 + 500.0*20.0/30.0)
	assert.InDelta(t, expected, *r, 1e-9)
	assert.InDelta(t, 0.075, *r, 1e-9)
}

func TestMaxDrawdown(t *testing.T) {
	values := []float64{100, 120, 90, 110, 60, 80}
	dd := MaxDrawdown(values)
	assert.InDelta(t, 0.5, dd.MaxDrawdown, 1e-9) // 120→60
	assert.Equal(t, 1, dd.PeakIndex)
	assert.Equal(t, 4, dd.TroughIndex)

	// 单调上升无回撤
	up := MaxDrawdown([]float64{1, 2, 3, 4})
	assert.Equal(t, 0.0, up.MaxDrawdown)
}

func TestAnnualVolatility_ConstantSeries(t *testing.T) {
	// 恒定序列波动率为 0
	assert.Equal(t, 0.0, AnnualVolatility([]float64{100, 100, 100, 100}))
	assert.Equal(t, 0.0, AnnualVolatility([]float64{100}))
}

func TestComputeConcentration(t *testing.T) {
	stats := ComputeConcentration([]float64{0.5, 0.3, 0.2})
	assert.InDelta(t, 0.5, stats.Top1, 1e-9)
	assert.InDelta(t, 1.0, stats.Top5, 1e-9)
	assert.InDelta(t, 0.25+0.09+0.04, stats.HHI, 1e-9)
}

func TestGenExecDates_Monthly(t *testing.T) {
	dates := genExecDates(d(2026, 1, 15), d(2026, 4, 20), string(DCAMonthly), 10)
	require.Len(t, dates, 4) // 1/10? no—start 1/15 → 首个为 2/10
	assert.Equal(t, "2026-02-10", dates[0].Format(dateLayout))
	assert.Equal(t, "2026-04-10", dates[len(dates)-1].Format(dateLayout))
}

func TestGenExecDates_Weekly(t *testing.T) {
	// 2026-08-24 是周一；区间两周应有两个周一
	dates := genExecDates(d(2026, 8, 24), d(2026, 9, 7), string(DCAWeekly), 1)
	require.Len(t, dates, 3)
	for _, dt := range dates {
		assert.Equal(t, 1, weekdayNum(dt))
	}
}

func TestGenExecDates_Daily(t *testing.T) {
	dates := genExecDates(d(2026, 8, 24), d(2026, 8, 27), string(DCADaily), 1)
	assert.Len(t, dates, 4)
}

func TestNavOnOrBefore(t *testing.T) {
	points := []quote.KLinePoint{
		{Date: "2026-01-05", Close: 10},
		{Date: "2026-01-06", Close: 11},
		{Date: "2026-01-09", Close: 12},
	}
	v, ok := navOnOrBefore(points, d(2026, 1, 7))
	assert.True(t, ok)
	assert.Equal(t, 11.0, v) // 1/7 无数据取 1/6

	v, ok = navOnOrBefore(points, d(2026, 1, 4))
	assert.False(t, ok)

	v, _ = navOnOrBefore(points, d(2026, 1, 20))
	assert.Equal(t, 12.0, v)
}

func TestSimulateDCA_vsLumpSum(t *testing.T) {
	// 构造净值序列：先跌后涨 → 定投应优于一次性投入
	points := []quote.KLinePoint{
		{Date: "2026-01-05", Close: 100},
		{Date: "2026-02-05", Close: 80},
		{Date: "2026-03-05", Close: 60},
		{Date: "2026-04-05", Close: 80},
		{Date: "2026-05-05", Close: 100},
	}
	resp, err := SimulateDCA(points, string(DCAMonthly), 5, 1000, d(2026, 1, 1), d(2026, 5, 31))
	require.NoError(t, err)

	// 定投 5 笔共 5000：1/5@100、2/5@80、3/5@60、4/5@80、5/5@100
	assert.Equal(t, 5, resp.ExecCount)
	assert.InDelta(t, 5000, resp.TotalInvested, 1e-6)
	expectedShares := 1000.0/100 + 1000.0/80 + 1000.0/60 + 1000.0/80 + 1000.0/100
	assert.InDelta(t, expectedShares, resp.SharesAccum, 1e-4)
	assert.InDelta(t, expectedShares*100, resp.FinalValue, 1e-4)
	// 先跌后涨定投收益率为正、优于一次性（首日全仓收益率=0）
	assert.Positive(t, resp.ReturnRate)
	assert.Positive(t, resp.Advantage)
}

func TestSimulateDCA_UpwardMarket_LumpWins(t *testing.T) {
	points := []quote.KLinePoint{
		{Date: "2026-01-05", Close: 100},
		{Date: "2026-02-05", Close: 110},
		{Date: "2026-03-05", Close: 120},
		{Date: "2026-04-05", Close: 130},
	}
	resp, err := SimulateDCA(points, string(DCAMonthly), 5, 1000, d(2026, 1, 1), d(2026, 4, 30))
	require.NoError(t, err)
	// 单边上涨行情中一次性买入更优
	assert.Negative(t, resp.Advantage)
}

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "-7", itoa(-7))
	assert.Equal(t, "2026", itoa(2026))
}

func TestAddDaysAndJoinNote(t *testing.T) {
	assert.Equal(t, "2026-09-25", addDays("2026-08-26", 30))
	assert.Equal(t, "bad-date", addDays("bad-date", 5), "非法日期应原样返回")
	assert.Equal(t, "b", joinNote("", "b"))
	assert.Equal(t, "a；b", joinNote("a", "b"))
	assert.Equal(t, "a", joinNote("a", ""))
}

func TestResolveBenchmarkSecid(t *testing.T) {
	v, ok := quote.ResolveBenchmarkSecid("csi300")
	assert.True(t, ok)
	assert.Equal(t, "1.000300", v)

	v, ok = quote.ResolveBenchmarkSecid("1.000905")
	assert.True(t, ok)
	assert.Equal(t, "1.000905", v)

	_, ok = quote.ResolveBenchmarkSecid("unknown")
	assert.False(t, ok)
}
