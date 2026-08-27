// Package ia 买卖建议引擎 — 基于追踪目标、买入成本锚点、技术面与组合状态的操作建议
package ia

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// 纯函数规则层（可独立测试）
// ============================================================================

// AdviceInput 单持仓建议评估输入
type AdviceInput struct {
	PnLRatio      float64 // 浮动收益率（-0.2 = -20%）
	LastPrice     float64
	AvgCost       float64 // 买入成本（卖出建议锚点）
	TargetPrice   float64 // 追踪目标价（0=未设置）
	StopLossPrice float64 // 止损参考价（0=未设置）
	AboveMA20     bool
	AboveMA60     bool
	TechAvailable bool // 是否具备技术面数据（股票/ETF/基金）
	WeightBreach  bool // 集中度超标
	OverAllocated bool // 类别配置超配
	HasMaturity   bool // 存款/理财且有到期日
	MaturityDays  int  // 剩余天数（HasMaturity=true 时有效）
}

// adviceOutcome 规则评估结果
type adviceOutcome struct {
	Action     AdviceAction
	Confidence int
	Reasons    []string
}

// evaluateAdvice 规则引擎（确定性，便于测试与调参）。
//
// 卖出信号（锚定买入成本与追踪目标）：
//   - 达到追踪目标价 → 强烈止盈
//   - 跌破止损参考价 → 止损离场
//   - 浮亏超 20%     → 止损评估线
//   - 技术破位（跌破 MA20 与 MA60）→ 减持
//   - 集中度超标/类别超配且当前盈利 → 再平衡减持
//
// 买入/持有信号：
//   - 距追踪目标仍有 ≥15% 空间且趋势向上 → 持有或逢低加仓
//   - 回调至成本附近（±5%）且 MA60 上方 → 加仓机会
func evaluateAdvice(in AdviceInput) adviceOutcome {
	out := adviceOutcome{Action: AdviceHold, Reasons: []string{}}
	sellScore := 0
	buyScore := 0

	// 存款/理财：持有到期逻辑，不走技术面
	if in.HasMaturity {
		switch {
		case in.MaturityDays <= 7:
			out.Action = AdviceWatch
			out.Confidence = 70
			out.Reasons = append(out.Reasons, fmt.Sprintf("距到期仅 %d 天，请提前安排赎回/转存", in.MaturityDays))
		default:
			out.Action = AdviceHold
			out.Confidence = 60
			out.Reasons = append(out.Reasons, fmt.Sprintf("存款/理财持有到期，剩余 %d 天", in.MaturityDays))
			if in.PnLRatio > 0.5 {
				out.Reasons = append(out.Reasons, "累计收益已超本金 50%，关注到期续作利率")
			}
		}
		return out
	}

	// ---- 卖出侧 ----
	if in.TargetPrice > 0 && in.LastPrice >= in.TargetPrice {
		sellScore += 45
		out.Reasons = append(out.Reasons, fmt.Sprintf("已达追踪目标 %.4f（现价 %.4f），按计划止盈", in.TargetPrice, in.LastPrice))
	}
	if in.StopLossPrice > 0 && in.LastPrice > 0 && in.LastPrice <= in.StopLossPrice {
		sellScore += 40
		out.Reasons = append(out.Reasons, fmt.Sprintf("跌破止损参考价 %.4f，执行纪律离场", in.StopLossPrice))
	}
	if in.PnLRatio <= -0.20 {
		sellScore += 30
		out.Reasons = append(out.Reasons, fmt.Sprintf("浮亏 %s 触及止损评估线（成本 %.4f）", fmtPct(in.PnLRatio), in.AvgCost))
	}
	if in.TechAvailable && !in.AboveMA20 && !in.AboveMA60 {
		sellScore += 25
		out.Reasons = append(out.Reasons, "价格同时跌破 MA20/MA60，中期趋势走弱")
	} else if in.TechAvailable && !in.AboveMA60 {
		sellScore += 10
		out.Reasons = append(out.Reasons, "价格位于 MA60 下方，趋势偏弱")
	}
	if in.WeightBreach && in.PnLRatio > 0 {
		sellScore += 20
		out.Reasons = append(out.Reasons, "持仓集中度超标且处于盈利，建议部分兑现降低集中度")
	}
	if in.OverAllocated && in.PnLRatio > 0 {
		sellScore += 15
		out.Reasons = append(out.Reasons, "该类别实际配比超出目标容忍度且盈利，可再平衡减持")
	}

	// ---- 买入侧 ----
	if in.TechAvailable {
		if in.TargetPrice > in.AvgCost && in.LastPrice < in.TargetPrice {
			space := (in.TargetPrice - in.LastPrice) / in.LastPrice
			if space >= 0.15 && in.AboveMA60 {
				buyScore += 30
				out.Reasons = append(out.Reasons, fmt.Sprintf("距追踪目标仍有 %s 空间且趋势向上", fmtPct(space)))
			}
		}
		if in.PnLRatio >= -0.05 && in.PnLRatio <= 0.02 && in.AboveMA60 {
			buyScore += 25
			out.Reasons = append(out.Reasons, fmt.Sprintf("价格回调至成本 %.4f 附近且中期趋势未破坏，可视资金情况加仓", in.AvgCost))
		}
	}

	// ---- 决策 ----
	switch {
	case sellScore >= 40:
		out.Action = AdviceSell
	case sellScore >= 20:
		out.Action = AdviceReduce
	case buyScore >= 25 && sellScore < 20:
		out.Action = AdviceBuy
	case in.TechAvailable && !in.AboveMA60 && in.PnLRatio < -0.05:
		out.Action = AdviceWatch
		out.Reasons = append(out.Reasons, "趋势与盈利均偏弱，观望等待企稳信号")
	}
	conf := sellScore
	if buyScore > conf {
		conf = buyScore
	}
	if out.Action == AdviceHold || out.Action == AdviceWatch {
		if conf == 0 {
			out.Reasons = append(out.Reasons, "各项指标正常，维持既有计划")
		}
	}
	if conf > 100 {
		conf = 100
	}
	out.Confidence = conf
	return out
}

// ============================================================================
// 服务层
// ============================================================================

// SetPositionTarget 设置/清除持仓追踪目标
func (s *Service) SetPositionTarget(ctx context.Context, req *SetPositionTargetReq) error {
	if req.TargetPrice < 0 || req.StopLossPrice < 0 {
		return NewIAError(ErrInvalidRatio)
	}
	if req.TargetPrice == 0 && req.StopLossPrice == 0 {
		return NewIAError(ErrInvalidRatio, "target_price 与 stop_loss_price 至少设置一项（传 0 表示清除对应项）")
	}
	return s.repo.SetPositionTarget(ctx, req.AccountID, req.AssetID, round6(req.TargetPrice), round6(req.StopLossPrice))
}

// TradeAdvice 为全部持仓生成买卖操作建议。
// 技术面：股票/ETF 取东财 K 线；场外基金用 price_history 净值序列近似；
// 存款/理财按到期日给出持有建议。卖出建议以买入成本(avg_cost)为锚点输出。
func (s *Service) TradeAdvice(ctx context.Context, accountID string) (*TradeAdviceResp, error) {
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: accountID, Status: "holding"})
	if err != nil {
		return nil, err
	}
	resp := &TradeAdviceResp{
		Scope: scopeName(accountID), GeneratedAt: time.Now().Format(time.RFC3339),
		Items: []TradeAdviceItem{},
	}

	// 集中度与超配信息（获取失败时静默降级为无告警）
	risk, _ := s.RiskMetrics(ctx, &AnalyticsReq{AccountID: accountID}, 0.3)
	breachAssets := make(map[string]bool)
	for i := range risk.Concentration {
		if risk.Concentration[i].Breach {
			breachAssets[risk.Concentration[i].AssetID] = true
		}
	}
	review, _ := s.AllocationReview(ctx, accountID)
	overCats := make(map[string]bool)
	for _, it := range review.Items {
		if it.Breach {
			overCats[it.Category] = true
		}
	}

	today := todayStr()
	for i := range positions {
		p := &positions[i]
		item := TradeAdviceItem{
			AccountID: p.AccountID, AssetID: p.AssetID,
			Code: p.Asset.Code, Name: p.Asset.Name, AssetType: p.Asset.AssetType,
			CostPrice: p.AvgCost, LastPrice: p.LastPrice,
			PnLRatio:      safeDiv(p.LastPrice-p.AvgCost, p.AvgCost),
			TargetPrice:   p.TargetPrice,
			StopLossPrice: p.StopLossPrice,
			MaturityDays:  -1,
		}
		if item.LastPrice <= 0 {
			item.LastPrice = p.AvgCost
			item.PnLRatio = 0
		}

		in := AdviceInput{
			PnLRatio: item.PnLRatio, LastPrice: item.LastPrice, AvgCost: p.AvgCost,
			TargetPrice: p.TargetPrice, StopLossPrice: p.StopLossPrice,
			WeightBreach:  breachAssets[p.AssetID],
			OverAllocated: overCats[string(allocCategoryOf(p.Asset.AssetType))],
		}

		// 到期天数（存款/理财）
		if p.Asset.AssetType == string(AssetTypeDeposit) || p.Asset.AssetType == string(AssetTypeWealth) {
			if p.Asset.MaturityDate != "" {
				d0, e1 := time.ParseInLocation(dateLayout, today, time.Local)
				d1, e2 := time.ParseInLocation(dateLayout, p.Asset.MaturityDate, time.Local)
				if e1 == nil && e2 == nil {
					in.HasMaturity = true
					in.MaturityDays = int(d1.Sub(d0).Hours() / 24)
					item.MaturityDays = in.MaturityDays
				}
			}
		}

		// 技术面（股票/ETF：K线；基金：净值历史近似）
		if isTechEligible(p.Asset.AssetType) {
			a20, a60, ok := s.techFlags(ctx, p.Asset)
			if ok {
				in.TechAvailable = true
				in.AboveMA20, in.AboveMA60 = a20, a60
				f, g := a20, a60
				item.AboveMA20, item.AboveMA60 = &f, &g
			}
		}

		// 目标推进进度
		if p.TargetPrice > p.AvgCost && p.AvgCost > 0 && item.LastPrice > 0 {
			item.TargetProgress = round6(clamp01((item.LastPrice - p.AvgCost) / (p.TargetPrice - p.AvgCost)))
		}

		outcome := evaluateAdvice(in)
		item.Action = string(outcome.Action)
		item.Confidence = outcome.Confidence
		item.Reasons = outcome.Reasons
		resp.Items = append(resp.Items, item)

		switch outcome.Action {
		case AdviceBuy:
			resp.BuyCount++
		case AdviceReduce:
			resp.ReduceCount++
		case AdviceSell:
			resp.SellCount++
		default:
			resp.HoldCount++
		}
	}
	return resp, nil
}

// isTechEligible 是否适用技术面分析
func isTechEligible(assetType string) bool {
	return assetType == string(AssetTypeStock) || assetType == string(AssetTypeETF) || assetType == string(AssetTypeFund)
}

// techFlags 计算标的 MA20/MA60 相对位置。
// 股票/ETF（沪深港美）走东财 K 线；场外基金回退 price_history 官方净值序列近似。
func (s *Service) techFlags(ctx context.Context, asset *Asset) (aboveMA20, aboveMA60 bool, ok bool) {
	var closes []float64
	switch asset.AssetType {
	case string(AssetTypeStock), string(AssetTypeETF):
		if s.quotes == nil {
			return false, false, false
		}
		points, err := s.fetchHistoryPoints(ctx, asset.Code, asset.Market, begForDays(90))
		if err != nil || len(points) < 21 {
			return false, false, false
		}
		closes = klineCloses(points)
	case string(AssetTypeFund):
		rows, err := s.repo.ListPriceHistory(ctx, asset.ID, 80)
		if err != nil || len(rows) < 21 {
			return false, false, false
		}
		// rows 降序 → 反转为升序
		closes = make([]float64, 0, len(rows))
		for i := len(rows) - 1; i >= 0; i-- {
			closes = append(closes, rows[i].Nav)
		}
	default:
		return false, false, false
	}
	last := closes[len(closes)-1]
	return last >= movingAverage(closes, 20), last >= movingAverage(closes, 60), true
}
