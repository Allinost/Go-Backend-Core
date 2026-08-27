// Package ia 分析报表数学函数层 — 收益率求解、风险指标、定投模拟（纯函数，可独立测试）
package ia

import (
	"fmt"
	"math"
	"time"

	"github.com/Allinost/go-backend-core/zzz/ia/quote"
)

// ============================================================================
// 现金流与收益率
// ============================================================================

// DatedFlow 带日期现金流（XIRR 用）
type DatedFlow struct {
	Date   time.Time
	Amount float64 // 出资为负、回收为正
}

// xirrYears 计算自首笔现金流起的天数折年（ACT/365）
func xirrYears(base, t time.Time) float64 {
	return t.Sub(base).Hours() / 24 / daysPerYear
}

// xirrNPV 给定利率下的净现值：Σ amt_i × (1+r)^(-years_i)
func xirrNPV(flows []DatedFlow, base time.Time, r float64) float64 {
	sum := 0.0
	for _, f := range flows {
		y := xirrYears(base, f.Date)
		sum += f.Amount * math.Pow(1+r, -y)
	}
	return sum
}

// xirrNPVDerivative 净现值对 r 的导数
func xirrNPVDerivative(flows []DatedFlow, base time.Time, r float64) float64 {
	sum := 0.0
	for _, f := range flows {
		y := xirrYears(base, f.Date)
		if y == 0 {
			continue
		}
		sum += -y * f.Amount * math.Pow(1+r, -y-1)
	}
	return sum
}

// SolveXIRR 求解年化资金加权收益率（牛顿迭代，失败回退二分法）。
// 要求现金流同时包含正负号；无解或数据不足返回 nil。
func SolveXIRR(flows []DatedFlow) *float64 {
	hasPos, hasNeg := false, false
	for _, f := range flows {
		if f.Amount > 0 {
			hasPos = true
		}
		if f.Amount < 0 {
			hasNeg = true
		}
	}
	if !hasPos || !hasNeg || len(flows) < 2 {
		return nil
	}

	base := flows[0].Date
	for _, f := range flows {
		if f.Date.Before(base) {
			base = f.Date
		}
	}

	// 牛顿迭代
	r := 0.1
	for i := 0; i < 100; i++ {
		npv := xirrNPV(flows, base, r)
		if math.Abs(npv) < 1e-8 {
			return &r
		}
		d := xirrNPVDerivative(flows, base, r)
		if d == 0 {
			break
		}
		next := r - npv/d
		if next <= -0.9999 {
			next = (r - 0.9999) / 2
		}
		if math.Abs(next-r) < 1e-10 {
			r = next
			return &r
		}
		r = next
	}
	if math.Abs(xirrNPV(flows, base, r)) < 1e-6 {
		return &r
	}

	// 二分法兜底：f 在 (-0.9999, 1000) 上关于 r 单调递减
	lo, hi := -0.9999, 1000.0
	fLo := xirrNPV(flows, base, lo)
	if fLo < 0 {
		return nil // 全区间无解
	}
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		v := xirrNPV(flows, base, mid)
		if math.Abs(v) < 1e-9 {
			return &mid
		}
		if v > 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	mid := (lo + hi) / 2
	if math.Abs(xirrNPV(flows, base, mid)) < 1e-4 {
		return &mid
	}
	return nil
}

// ModifiedDietz 修正迪茨收益率（区间资金加权近似）：
//
//	r = (V1 − V0 − F) / (V0 + Σ Fᵢ × wᵢ)，wᵢ = (总天数 − 第 i 笔距起始天数) / 总天数
type timedFlow struct {
	Date   time.Time
	Amount float64
}

func ModifiedDietz(v0, v1 float64, flows []timedFlow, start, end time.Time) *float64 {
	totalDays := end.Sub(start).Hours() / 24
	if totalDays <= 0 || v0 <= 0 {
		return nil
	}
	netFlow := 0.0
	weighted := 0.0
	for _, f := range flows {
		netFlow += f.Amount
		daysFromStart := f.Date.Sub(start).Hours() / 24
		w := (totalDays - daysFromStart) / totalDays
		weighted += f.Amount * w
	}
	denom := v0 + weighted
	if denom <= 0 {
		return nil
	}
	r := (v1 - v0 - netFlow) / denom
	return &r
}

// SimplePeriodReturn 简单区间收益率：(V1 − V0 − F) / V0
func SimplePeriodReturn(v0, v1, netFlow float64) *float64 {
	if v0 <= 0 {
		return nil
	}
	r := (v1 - v0 - netFlow) / v0
	return &r
}

// ============================================================================
// 风险指标
// ============================================================================

// DrawdownPoint 回撤定位点
type DrawdownPoint struct {
	MaxDrawdown float64
	PeakIndex   int
	TroughIndex int
}

// MaxDrawdown 最大回撤及峰谷位置（values 为时序资产值）
func MaxDrawdown(values []float64) DrawdownPoint {
	res := DrawdownPoint{}
	peak := math.Inf(-1)
	peakIdx := -1
	for i, v := range values {
		if v > peak {
			peak = v
			peakIdx = i
		}
		if peak > 0 {
			dd := (peak - v) / peak
			if dd > res.MaxDrawdown {
				res.MaxDrawdown = dd
				res.PeakIndex = peakIdx
				res.TroughIndex = i
			}
		}
	}
	return res
}

// AnnualVolatility 年化波动率：日收益率标准差 × √252
func AnnualVolatility(values []float64) float64 {
	if len(values) < 3 {
		return 0
	}
	rets := make([]float64, 0, len(values)-1)
	for i := 1; i < len(values); i++ {
		if values[i-1] > 0 {
			rets = append(rets, values[i]/values[i-1]-1)
		}
	}
	if len(rets) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	variance := 0.0
	for _, r := range rets {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(rets) - 1)
	return math.Sqrt(variance) * math.Sqrt(252)
}

// ConcentrationStats 集中度统计
type ConcentrationStats struct {
	Top1 float64
	Top5 float64
	HHI  float64
}

// ComputeConcentration 由降序权重序列计算集中度指标
func ComputeConcentration(weightsDesc []float64) ConcentrationStats {
	stats := ConcentrationStats{}
	if len(weightsDesc) > 0 {
		stats.Top1 = weightsDesc[0]
	}
	for i, w := range weightsDesc {
		if i < 5 {
			stats.Top5 += w
		}
		stats.HHI += w * w
	}
	return stats
}

// ============================================================================
// 定投模拟
// ============================================================================

// genExecDates 生成的执行日期序列（start 起至 end 止）
func genExecDates(start, end time.Time, cycle string, execDay int) []time.Time {
	start, end = truncateDate(start), truncateDate(end)
	if end.Before(start) {
		return nil
	}
	var out []time.Time
	switch DCACycle(cycle) {
	case DCADaily:
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			out = append(out, d)
		}
	case DCAWeekly, DCABiWeekly:
		step := 7
		if DCACycle(cycle) == DCABiWeekly {
			step = 14
		}
		// 锚定首个 ≥ start 且匹配 execDay 的日期，此后按 step 递增
		first := start
		targetW := execDay
		for weekdayNum(first) != targetW && !first.After(end) {
			first = first.AddDate(0, 0, 1)
		}
		for d := first; !d.After(end); d = d.AddDate(0, 0, step) {
			out = append(out, d)
		}
	case DCAMonthly:
		y, m, _ := start.Date()
		for {
			cand := time.Date(y, m, execDay, 0, 0, 0, 0, start.Location())
			if cand.Before(start) {
				cand = cand.AddDate(0, 1, 0)
			}
			if cand.After(end) {
				break
			}
			out = append(out, cand)
			m++
			if m > 12 {
				m = 1
				y++
			}
		}
	default:
		return nil
	}
	return out
}

// navOnOrBefore 取日期当天或之前最近的收盘净值（points 升序），未找到返回 false
func navOnOrBefore(points []quote.KLinePoint, date time.Time) (float64, bool) {
	ds := date.Format(dateLayout)
	found := 0.0
	for _, p := range points {
		if p.Date <= ds && p.Close > 0 {
			found = p.Close
		}
		if p.Date > ds {
			break
		}
	}
	return found, found > 0
}

// SimulateDCA 定投 vs 一次性投入模拟。
// 定投：每个执行日以当日（或之前最近）收盘净值买入固定金额；
// 一次性：全部本金按首个执行日净值买入。
func SimulateDCA(points []quote.KLinePoint, cycle string, execDay int, amount float64, start, end time.Time) (*DCASimulateResp, error) {
	execDates := genExecDates(start, end, cycle, execDay)
	if len(execDates) == 0 {
		return nil, NewIAError(ErrInsufficientData, "执行日期序列为空")
	}

	shares := 0.0
	invested := 0.0
	count := 0
	firstNav := 0.0
	for _, d := range execDates {
		nav, ok := navOnOrBefore(points, d)
		if !ok {
			continue
		}
		if firstNav == 0 {
			firstNav = nav
		}
		shares += amount / nav
		invested += amount
		count++
	}
	if count == 0 || firstNav == 0 {
		return nil, NewIAError(ErrInsufficientData, "区间内无可用的历史净值")
	}

	finalNav, ok := navOnOrBefore(points, end)
	if !ok {
		finalNav = firstNav
	}
	finalValue := shares * finalNav
	lumpSumValue := invested / firstNav * finalNav

	resp := &DCASimulateResp{
		Cycle:          cycle,
		StartDate:      start.Format(dateLayout),
		EndDate:        end.Format(dateLayout),
		ExecCount:      count,
		TotalInvested:  round4(invested),
		SharesAccum:    round6(shares),
		FinalNav:       round6(finalNav),
		FinalValue:     round4(finalValue),
		ReturnRate:     round6(safeDiv(finalValue-invested, invested)),
		LumpSumValue:   round4(lumpSumValue),
		LumpReturnRate: round6(safeDiv(lumpSumValue-invested, invested)),
	}
	resp.Advantage = round6(resp.ReturnRate - resp.LumpReturnRate)
	return resp, nil
}

// fmtPct 百分比格式化辅助（消息文本用）
func fmtPct(v float64) string {
	return fmt.Sprintf("%.2f%%", v*100)
}
