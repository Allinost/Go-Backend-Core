package ia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateAdvice_TargetHitSell(t *testing.T) {
	// 买入成本 10，追踪目标 15，现价 15.2 → 强烈止盈
	out := evaluateAdvice(AdviceInput{
		PnLRatio: 0.5, LastPrice: 15.2, AvgCost: 10, TargetPrice: 15,
		TechAvailable: true, AboveMA20: true, AboveMA60: true,
	})
	assert.Equal(t, AdviceSell, out.Action)
	assert.GreaterOrEqual(t, out.Confidence, 45)
	assert.Contains(t, joinReasons(out), "追踪目标")
}

func TestEvaluateAdvice_StopLossHit(t *testing.T) {
	// 跌破止损参考价
	out := evaluateAdvice(AdviceInput{
		PnLRatio: -0.12, LastPrice: 8.8, AvgCost: 10, StopLossPrice: 9.0,
		TechAvailable: true, AboveMA20: true, AboveMA60: true,
	})
	assert.Equal(t, AdviceSell, out.Action)
	assert.Contains(t, joinReasons(out), "止损参考价")
}

func TestEvaluateAdvice_DeepLossEvaluationLine(t *testing.T) {
	// 浮亏 25% 无止损设置 → 触发止损评估线，叠加趋势破位应达 reduce 及以上
	out := evaluateAdvice(AdviceInput{
		PnLRatio: -0.25, LastPrice: 7.5, AvgCost: 10,
		TechAvailable: true, AboveMA20: false, AboveMA60: false,
	})
	assert.Contains(t, []AdviceAction{AdviceReduce, AdviceSell}, out.Action)
	found := false
	for _, r := range out.Reasons {
		if contains(r, "止损评估线") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestEvaluateAdvice_TrendBreakReduce(t *testing.T) {
	// 盈利但跌破双均线
	out := evaluateAdvice(AdviceInput{
		PnLRatio: 0.05, LastPrice: 10.5, AvgCost: 10,
		TechAvailable: true, AboveMA20: false, AboveMA60: false,
	})
	assert.Equal(t, AdviceReduce, out.Action)
	assert.Contains(t, joinReasons(out), "MA20/MA60")
}

func TestEvaluateAdvice_BuyOnPullbackNearCost(t *testing.T) {
	// 回调至成本附近 + 中期趋势向上
	out := evaluateAdvice(AdviceInput{
		PnLRatio: -0.02, LastPrice: 9.8, AvgCost: 10, TargetPrice: 13,
		TechAvailable: true, AboveMA20: false, AboveMA60: true,
	})
	assert.Equal(t, AdviceBuy, out.Action)
	assert.Contains(t, joinReasons(out), "加仓")
}

func TestEvaluateAdvice_BuyTargetSpace(t *testing.T) {
	// 目标空间充足 + 趋势向上
	out := evaluateAdvice(AdviceInput{
		PnLRatio: 0.08, LastPrice: 10.8, AvgCost: 10, TargetPrice: 14,
		TechAvailable: true, AboveMA20: true, AboveMA60: true,
	})
	assert.Equal(t, AdviceBuy, out.Action)
	assert.Contains(t, joinReasons(out), "空间")
}

func TestEvaluateAdvice_RebalanceTrim(t *testing.T) {
	// 集中度超标且盈利
	out := evaluateAdvice(AdviceInput{
		PnLRatio: 0.2, LastPrice: 12, AvgCost: 10,
		TechAvailable: true, AboveMA20: true, AboveMA60: true,
		WeightBreach: true,
	})
	assert.Equal(t, AdviceReduce, out.Action)
	assert.Contains(t, joinReasons(out), "集中度")
}

func TestEvaluateAdvice_DepositHoldToMaturity(t *testing.T) {
	out := evaluateAdvice(AdviceInput{HasMaturity: true, MaturityDays: 90})
	assert.Equal(t, AdviceHold, out.Action)
	assert.Contains(t, joinReasons(out), "剩余 90 天")

	warn := evaluateAdvice(AdviceInput{HasMaturity: true, MaturityDays: 3})
	assert.Equal(t, AdviceWatch, warn.Action)
	assert.Contains(t, joinReasons(warn), "赎回")
}

func TestEvaluateAdvice_HealthyHold(t *testing.T) {
	out := evaluateAdvice(AdviceInput{
		PnLRatio: 0.1, LastPrice: 11, AvgCost: 10,
		TechAvailable: true, AboveMA20: true, AboveMA60: true,
	})
	assert.Equal(t, AdviceHold, out.Action)
	assert.NotEmpty(t, out.Reasons)
}

func TestSetPositionTargetValidation(t *testing.T) {
	svc := &Service{}
	err := svc.SetPositionTarget(nil, &SetPositionTargetReq{
		AccountID: "a", AssetID: "b", TargetPrice: 0, StopLossPrice: 0,
	})
	assert.Error(t, err, "两项全零应拒绝（无操作意义）")
}

// helpers
func joinReasons(o adviceOutcome) string {
	s := ""
	for _, r := range o.Reasons {
		s += r + "|"
	}
	return s
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
