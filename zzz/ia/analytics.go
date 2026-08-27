// Package ia 分析报表业务层 — 收益曲线、收益率指标、交易复盘、风险集中度、定投模拟、费用分红统计
package ia

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/Allinost/go-backend-core/zzz/ia/quote"
)

// ============================================================================
// 收益曲线
// ============================================================================

// EquityCurve 组合收益曲线（快照驱动），可选叠加基准指数（归一化基期=100）
func (s *Service) EquityCurve(ctx context.Context, req *AnalyticsReq, benchmark string) (*EquityCurveResp, error) {
	if req.End != "" && !isDateStr(req.End) {
		return nil, NewIAError(ErrInvalidDate)
	}
	snaps, err := s.repo.ListSnapshotsRange(ctx, req.AccountID, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	resp := &EquityCurveResp{
		Scope:  scopeName(req.AccountID),
		Points: []EquityCurvePoint{},
	}
	for _, sn := range snaps {
		resp.Points = append(resp.Points, EquityCurvePoint{
			Date:     sn.SnapDate,
			Value:    round4(sn.TotalValue + sn.CashAmount),
			Cost:     round4(sn.TotalCost),
			PnL:      round4(sn.TotalPnL),
			PnLRatio: round6(safeDiv(sn.TotalPnL, sn.TotalCost)),
		})
	}

	if benchmark != "" && len(resp.Points) > 0 {
		secid, ok := quote.ResolveBenchmarkSecid(benchmark)
		if !ok {
			return nil, NewIAError(ErrInvalidMarket, "未知基准指数：%s", benchmark)
		}
		beg := resp.Points[0].Date
		points, err := s.quotes.Kline(ctx, secid, beg)
		if err != nil {
			return resp, nil // 基准获取失败不阻塞组合曲线
		}
		norm := quote.NormalizeSeries(points, beg)
		for i := range resp.Points {
			if v, ok := norm[resp.Points[i].Date]; ok {
				resp.Benchmark = append(resp.Benchmark, BenchmarkPoint{Date: resp.Points[i].Date, Index: round4(v)})
			}
		}
		resp.BenchmarkName = benchmark
	}
	return resp, nil
}

// ============================================================================
// 区间收益率指标
// ============================================================================

// ReturnsAnalytics 区间收益率：简单收益 + Modified Dietz + 全周期 XIRR（CNY 口径）
func (s *Service) ReturnsAnalytics(ctx context.Context, req *AnalyticsReq) (*ReturnsResp, error) {
	rates, conv := s.cnyConverterOrEmpty(ctx)

	end := req.End
	start := req.Start
	if end == "" {
		end = todayStr()
	}
	if start == "" {
		// 默认回溯一年
		t, _ := time.ParseInLocation(dateLayout, end, time.Local)
		start = t.AddDate(-1, 0, 1).Format(dateLayout)
	}
	if !isDateStr(start) || !isDateStr(end) || start > end {
		return nil, NewIAError(ErrInvalidDate)
	}

	resp := &ReturnsResp{Scope: scopeName(req.AccountID), StartDate: start, EndDate: end}

	// 起止资产值：取窗口前最近快照，无则记 0 并说明
	startSnap, err := s.repo.LatestSnapshotBefore(ctx, req.AccountID, start)
	if err != nil {
		return nil, err
	}
	if startSnap != nil {
		resp.StartValue = round4(startSnap.TotalValue + startSnap.CashAmount)
	} else {
		resp.Note = "起始日无快照，区间收益以首个可得快照为基准"
		snapsFirst, _ := s.repo.ListSnapshotsRange(ctx, req.AccountID, start, end)
		if len(snapsFirst) > 0 {
			resp.StartValue = round4(snapsFirst[0].TotalValue + snapsFirst[0].CashAmount)
			resp.StartDate = snapsFirst[0].SnapDate
		}
	}
	endSnap, err := s.repo.LatestSnapshotBefore(ctx, req.AccountID, addDays(end, 1))
	if err != nil {
		return nil, err
	}
	if endSnap != nil {
		resp.EndValue = round4(endSnap.TotalValue + endSnap.CashAmount)
	} else {
		live, lerr := s.currentAssetValueCNY(ctx, req.AccountID, rates)
		if lerr != nil {
			return nil, lerr
		}
		resp.EndValue = round4(live)
	}

	// 窗口内外部资金流（入金+/出金-，划转不计），统一折算 CNY
	flows, err := s.repo.ListCashFlowsRange(ctx, req.AccountID, start, end)
	if err != nil {
		return nil, err
	}
	timedFlows := make([]timedFlow, 0, len(flows))
	for _, f := range flows {
		if f.FlowType != string(CashFlowDepositIn) && f.FlowType != string(CashFlowWithdrawOut) {
			continue
		}
		sign := 1.0
		if f.FlowType == string(CashFlowWithdrawOut) {
			sign = -1.0
		}
		d, _ := time.ParseInLocation(dateLayout, f.Date, time.Local)
		timedFlows = append(timedFlows, timedFlow{Date: d, Amount: sign * conv(f.Currency, f.Amount)})
	}
	resp.NetExternalFlow = round4(sumFlows(timedFlows))
	resp.FlowCount = len(timedFlows)

	if r := SimplePeriodReturn(resp.StartValue, resp.EndValue, resp.NetExternalFlow); r != nil {
		resp.SimpleReturn = round6(*r)
	}
	st, _ := time.ParseInLocation(dateLayout, resp.StartDate, time.Local)
	et, _ := time.ParseInLocation(dateLayout, resp.EndDate, time.Local)
	if r := ModifiedDietz(resp.StartValue, resp.EndValue, timedFlows, st, et); r != nil {
		resp.MoneyWeighted = float64Ptr(round6(*r))
	}

	// XIRR：自最早一笔出资起的全生命周期口径
	allFlows, err := s.repo.ListCashFlowsRange(ctx, req.AccountID, "", end)
	if err != nil {
		return nil, err
	}
	var dated []DatedFlow
	for _, f := range allFlows {
		if f.FlowType != string(CashFlowDepositIn) && f.FlowType != string(CashFlowWithdrawOut) {
			continue
		}
		sign := 1.0
		if f.FlowType == string(CashFlowWithdrawOut) {
			sign = -1.0
		}
		d, perr := time.ParseInLocation(dateLayout, f.Date, time.Local)
		if perr != nil {
			continue
		}
		dated = append(dated, DatedFlow{Date: d, Amount: -sign * conv(f.Currency, f.Amount)})
	}
	// 终值：今日资产按回收计入（出资为负 → 回收为正）
	dated = append(dated, DatedFlow{Date: et, Amount: resp.EndValue})
	if x := SolveXIRR(dated); x != nil {
		resp.XIRR = float64Ptr(round6(*x))
	} else if len(dated) <= 2 {
		resp.Note = joinNote(resp.Note, "现金流不足，无法计算 XIRR")
	}
	return resp, nil
}

// currentAssetValueCNY 当前资产总值（持仓市值+现金，CNY 口径）
func (s *Service) currentAssetValueCNY(ctx context.Context, accountID string, rates map[string]float64) (float64, error) {
	conv := cnyConverter(rates)
	total := 0.0
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: accountID, Status: "holding"})
	if err != nil {
		return 0, err
	}
	for i := range positions {
		p := &positions[i]
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		total += conv(p.Currency, price*p.Shares)
	}
	balances, err := s.repo.CashBalances(ctx)
	if err != nil {
		return total, err
	}
	for _, b := range balances {
		if accountID != "" && b.AccountID != accountID {
			continue
		}
		total += conv(b.Currency, b.Balance)
	}
	return total, nil
}

// cnyConverterOrEmpty 加载最新汇率并返回折算函数；失败时退化为仅 CNY 口径
func (s *Service) cnyConverterOrEmpty(ctx context.Context) (map[string]float64, func(ccy string, v float64) float64) {
	rates, err := s.repo.LatestFxRates(ctx)
	if err != nil {
		rates = map[string]float64{}
	}
	return rates, cnyConverter(rates)
}

// ============================================================================
// 加仓减持复盘
// ============================================================================

// TradesReview 加仓减持操作复盘：
//   - 每笔加仓的摊薄效果与买入后表现（当前价 vs 买价）
//   - 每笔减持的已实现盈亏与机会成本（当前价 vs 卖价）
//   - 卖出胜率与盈亏比汇总
func (s *Service) TradesReview(ctx context.Context, req *AnalyticsReq) (*TradesReviewResp, error) {
	assets, err := s.repo.ListReviewedAssets(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	resp := &TradesReviewResp{Scope: scopeName(req.AccountID), Assets: []AssetTradeReview{}}

	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: req.AccountID, Status: "holding"})
	if err != nil {
		return nil, err
	}
	lastPrices := make(map[string]float64)
	for i := range positions {
		p := &positions[i]
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		lastPrices[p.AssetID] += price
	}

	sum := &TradesReviewSummary{}
	gainSumTotal, lossAbsSumTotal := 0.0, 0.0
	for _, a := range assets {
		txs, err := s.repo.ListConfirmedByAcctAsset(ctx, a.AccountID, a.AssetID)
		if err != nil {
			return nil, err
		}
		lots, err := s.repo.RealizedLotsByAsset(ctx, a.AccountID, a.AssetID)
		if err != nil {
			return nil, err
		}
		realizedByTx := make(map[int64]RealizedPnL, len(lots))
		for _, l := range lots {
			realizedByTx[l.TransactionID] = l
		}

		review := AssetTradeReview{
			AccountID: a.AccountID, AssetID: a.AssetID, Code: a.Code, Name: a.Name, Currency: a.Currency,
			Buys: []TradeReviewBuy{}, Sells: []TradeReviewSell{},
		}
		shares, costAmount := 0.0, 0.0
		lastPrice := lastPrices[a.AssetID]

		for i := range txs {
			t := &txs[i]
			switch TradeType(t.TradeType) {
			case TradeBuy, TradeRightsIssue:
				avgBefore := safeDiv(costAmount, shares)
				shares += t.Shares
				costAmount += t.Amount + t.Fee
				avgAfter := safeDiv(costAmount, shares)
				if avgBefore > 0 && t.TradeType == string(TradeBuy) {
					review.Buys = append(review.Buys, TradeReviewBuy{
						TxID: t.ID, Date: t.TradeAt.Format(dateLayout),
						Price: t.Price, Shares: t.Shares, Amount: t.Amount,
						AvgBefore: round6(avgBefore), AvgAfter: round6(avgAfter),
						DilutionRatio: round6(safeDiv(avgAfter-avgBefore, avgBefore)),
						PerfSince:     perfSince(lastPrice, t.Price),
					})
				}
			case TradeDividendReinv:
				shares += t.Shares
				costAmount += t.Amount
			case TradeBonusShares:
				shares += t.Shares
			case TradeSell:
				shares -= t.Shares
				costRemoved := safeDiv(costAmount, shares+t.Shares) * t.Shares
				costAmount -= costRemoved
				lot, hasLot := realizedByTx[t.ID]
				netPnL, retRate := lot.NetPnL, lot.CostPrice
				if hasLot && retRate > 0 {
					retRate = round6(safeDiv(lot.NetPnL, lot.CostPrice*lot.Shares))
				} else {
					retRate = 0
				}
				gap := 0.0
				if lastPrice > 0 && t.Price > 0 {
					gap = round6((lastPrice - t.Price) / t.Price)
				}
				review.Sells = append(review.Sells, TradeReviewSell{
					TxID: t.ID, Date: t.TradeAt.Format(dateLayout),
					Price: t.Price, Shares: t.Shares,
					NetPnL: round4(netPnL), ReturnRate: retRate,
					OpportunityGap: gap, Win: netPnL > 0,
				})
			}
		}

		// 标的级胜率与全局汇总
		acc := &tradesAccum{}
		for _, sl := range review.Sells {
			acc.totalSells++
			sum.TotalSells++
			sum.TotalRealized += sl.NetPnL
			if sl.Win {
				acc.wins++
				acc.winRetSum += sl.ReturnRate
				acc.gainSum += sl.NetPnL
			} else {
				acc.losses++
				acc.lossRetSum += sl.ReturnRate
				acc.lossAbsSum += abs(sl.NetPnL)
			}
		}
		sum.TotalBuys += len(review.Buys)
		if total := len(review.Sells); total > 0 {
			review.WinRate = round6(float64(acc.wins) / float64(total))
		}
		sum.Wins += acc.wins
		sum.Losses += acc.losses
		sum.AvgWinReturn += acc.winRetSum
		sum.AvgLossReturn += acc.lossRetSum
		gainSumTotal += acc.gainSum
		lossAbsSumTotal += acc.lossAbsSum

		resp.Assets = append(resp.Assets, review)
	}

	if sum.Wins+sum.Losses > 0 {
		sum.WinRate = round6(float64(sum.Wins) / float64(sum.Wins+sum.Losses))
	}
	if sum.Wins > 0 {
		sum.AvgWinReturn = round6(sum.AvgWinReturn / float64(sum.Wins))
	}
	if sum.Losses > 0 {
		sum.AvgLossReturn = round6(sum.AvgLossReturn / float64(sum.Losses))
	}
	if lossAbsSumTotal > 0 {
		sum.ProfitFactor = round6(gainSumTotal / lossAbsSumTotal)
	}
	sum.TotalRealized = round4(sum.TotalRealized)
	resp.Summary = *sum
	return resp, nil
}

// perfSince 买入后表现：(现价-买价)/买价
func perfSince(lastPrice, buyPrice float64) float64 {
	if lastPrice <= 0 || buyPrice <= 0 {
		return 0
	}
	return round6((lastPrice - buyPrice) / buyPrice)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func sumFlows(flows []timedFlow) float64 {
	s := 0.0
	for _, f := range flows {
		s += f.Amount
	}
	return s
}

// ============================================================================
// 风险与集中度
// ============================================================================

// RiskMetrics 最大回撤、年化波动率（快照序列）与持仓集中度
func (s *Service) RiskMetrics(ctx context.Context, req *AnalyticsReq, threshold float64) (*RiskMetricsResp, error) {
	_, conv := s.cnyConverterOrEmpty(ctx)

	if threshold <= 0 || threshold >= 1 {
		threshold = 0.3
	}
	resp := &RiskMetricsResp{
		Scope:                  scopeName(req.AccountID),
		ConcentrationThreshold: threshold,
		Concentration:          []ConcentrationItem{},
	}

	// 快照序列风险指标
	snaps, err := s.repo.ListSnapshotsRange(ctx, req.AccountID, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	if len(snaps) >= 2 {
		values := make([]float64, 0, len(snaps))
		for _, sn := range snaps {
			values = append(values, sn.TotalValue+sn.CashAmount)
		}
		dd := MaxDrawdown(values)
		resp.MaxDrawdown = round6(dd.MaxDrawdown)
		if dd.PeakIndex >= 0 && dd.TroughIndex < len(snaps) {
			resp.MDDPeakDate = snaps[dd.PeakIndex].SnapDate
			resp.MDDTroughDate = snaps[dd.TroughIndex].SnapDate
		}
		resp.AnnualVolatility = round6(AnnualVolatility(values))
		resp.SampleDays = len(values)
		resp.StartDate = snaps[0].SnapDate
		resp.EndDate = snaps[len(snaps)-1].SnapDate
	} else {
		resp.Note = "快照不足 2 份，无法计算回撤与波动率"
	}

	// 集中度
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: req.AccountID, Status: "holding"})
	if err != nil {
		return nil, err
	}
	type posVal struct {
		asset Asset
		val   float64
	}
	pvs := make([]posVal, 0, len(positions))
	total := 0.0
	for i := range positions {
		p := &positions[i]
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		v := conv(p.Currency, price*p.Shares)
		pvs = append(pvs, posVal{asset: *p.Asset, val: v})
		total += v
	}
	sort.Slice(pvs, func(i, j int) bool { return pvs[i].val > pvs[j].val })
	weights := make([]float64, 0, len(pvs))
	for _, pv := range pvs {
		w := 0.0
		if total > 0 {
			w = pv.val / total
		}
		weights = append(weights, w)
		resp.Concentration = append(resp.Concentration, ConcentrationItem{
			AssetID: pv.asset.ID, Code: pv.asset.Code, Name: pv.asset.Name,
			Value: round4(pv.val), Weight: round6(w), Breach: w > threshold,
		})
	}
	stats := ComputeConcentration(weights)
	resp.Top1Weight = round6(stats.Top1)
	resp.Top5Weight = round6(stats.Top5)
	resp.HHI = round6(stats.HHI)
	resp.TotalValue = round4(total)
	return resp, nil
}

// ============================================================================
// 定投模拟
// ============================================================================

// DCASimulate 定投 vs 一次性投入模拟（基于标的历史净值/价格）
func (s *Service) DCASimulate(ctx context.Context, req *DCASimulateReq) (*DCASimulateResp, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	if !validDCACycles[req.Cycle] {
		return nil, NewIAError(ErrInvalidDCACycle)
	}
	if (req.Cycle == string(DCAWeekly) || req.Cycle == string(DCABiWeekly)) && req.ExecDay > 7 {
		return nil, NewIAError(ErrInvalidDCACycle, "周定投执行日应为 1-7")
	}
	if !isDateStr(req.Start) {
		return nil, NewIAError(ErrInvalidDate)
	}
	end := req.End
	if end == "" {
		end = todayStr()
	}
	if !isDateStr(end) || end < req.Start {
		return nil, NewIAError(ErrInvalidDate, "结束日期需不早于开始日期")
	}

	asset, err := s.repo.GetAsset(ctx, req.AssetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewIAError(ErrAssetNotFound)
		}
		return nil, err
	}
	points, err := s.fetchHistoryPoints(ctx, asset.Code, asset.Market, req.Start)
	if err != nil {
		return nil, NewIAError(ErrInsufficientData, "%v", err)
	}
	st, _ := time.ParseInLocation(dateLayout, req.Start, time.Local)
	et, _ := time.ParseInLocation(dateLayout, end, time.Local)
	return SimulateDCA(points, req.Cycle, req.ExecDay, req.Amount, st, et)
}

// fetchHistoryPoints 获取标的日K线历史（股票/ETF 走东财；场外基金暂不支持模拟并提示）
func (s *Service) fetchHistoryPoints(ctx context.Context, code, market, beg string) ([]quote.KLinePoint, error) {
	switch market {
	case string(MarketCNSH), string(MarketCNSZ), string(MarketHK), string(MarketUS):
	default:
		return nil, NewIAError(ErrInsufficientData, "场外基金暂不支持K线模拟，可使用 ETF 对应标的")
	}
	secids := quote.SecidCandidates(market, code)
	if len(secids) == 0 {
		return nil, NewIAError(ErrInvalidMarket)
	}
	var lastErr error
	for _, secid := range secids {
		points, err := s.quotes.Kline(ctx, secid, beg)
		if err == nil {
			return points, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// ============================================================================
// 费用分析
// ============================================================================

// FeesReport 费用分析：交易费税 + 独立费用，年度分布与侵蚀度
func (s *Service) FeesReport(ctx context.Context, req *AnalyticsReq) (*FeesReportResp, error) {
	rates, conv := s.cnyConverterOrEmpty(ctx)
	tradeRows, err := s.repo.TradeFeeSums(ctx, req.AccountID, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	standaloneRows, err := s.repo.StandaloneFeeAggs(ctx, req.AccountID, req.Start, req.End)
	if err != nil {
		return nil, err
	}

	resp := &FeesReportResp{
		Scope:            scopeName(req.AccountID),
		StartDate:        req.Start,
		EndDate:          req.End,
		StandaloneByType: make(map[string]float64),
		ByYear:           []FeeYearRow{},
	}
	yearMap := make(map[int]*FeeYearRow)
	getYear := func(y int) *FeeYearRow {
		if yearMap[y] == nil {
			yearMap[y] = &FeeYearRow{Year: y}
		}
		return yearMap[y]
	}

	standaloneTotalCNY := 0.0
	for _, r := range standaloneRows {
		cny := conv("CNY", r.Total) // 独立费用表无币种关联资产时默认 CNY 记账
		resp.StandaloneByType[r.FeeType] += round4(cny)
		resp.TotalStandalone += cny
		standaloneTotalCNY += cny
		row := getYear(r.Year)
		row.StandaloneFees += cny
		row.Total += cny
	}
	_ = standaloneTotalCNY
	for _, r := range tradeRows {
		row := getYear(r.Year)
		row.TradeFees = round4(r.Fee)
		row.TradeTax = round4(r.Tax)
		row.Total += row.TradeFees + row.TradeTax
		resp.TotalTradeFees += r.Fee
		resp.TotalTradeTax += r.Tax
	}
	for _, row := range yearMap {
		resp.ByYear = append(resp.ByYear, *row)
	}
	sort.Slice(resp.ByYear, func(i, j int) bool { return resp.ByYear[i].Year < resp.ByYear[j].Year })
	resp.Grand = round4(resp.TotalStandalone + resp.TotalTradeFees + resp.TotalTradeTax)

	// 侵蚀度：近 12 月费用 ÷ 资产基数（最新快照值或当前市值）
	base, err := s.analyticsBaseValue(ctx, req.AccountID, rates)
	if err == nil {
		resp.BaseValue = round4(base)
	}
	trailingStart := addDays(todayStr(), -365)
	tf, _ := s.repo.TradeFeeSums(ctx, req.AccountID, trailingStart, todayStr())
	sf, _ := s.repo.StandaloneFeeAggs(ctx, req.AccountID, trailingStart, todayStr())
	trailing := 0.0
	for _, r := range tf {
		trailing += r.Fee + r.Tax
	}
	for _, r := range sf {
		trailing += conv("CNY", r.Total)
	}
	if base > 0 {
		resp.DragRatio = round6(trailing / base)
	}
	return resp, nil
}

// analyticsBaseValue 费用侵蚀度分母：优先最新全局/账户快照总值，否则当前市值
func (s *Service) analyticsBaseValue(ctx context.Context, accountID string, rates map[string]float64) (float64, error) {
	snap, err := s.repo.LatestSnapshotBefore(ctx, accountID, addDays(todayStr(), 1))
	if err == nil && snap != nil && snap.TotalValue > 0 {
		return snap.TotalValue + snap.CashAmount, nil
	}
	return s.currentAssetValueCNY(ctx, accountID, rates)
}

// ============================================================================
// 分红统计
// ============================================================================

// DividendReport 分红统计：标的排行、年度分布、税负与股息率
func (s *Service) DividendReport(ctx context.Context, req *AnalyticsReq) (*DividendReportResp, error) {
	rows, err := s.repo.DividendAggs(ctx, req.AccountID, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	_, conv := s.cnyConverterOrEmpty(ctx)

	resp := &DividendReportResp{
		Scope:   scopeName(req.AccountID),
		ByAsset: []DividendAssetRow{},
	}
	assetMap := make(map[string]*DividendAssetRow)
	for _, r := range rows {
		item := assetMap[r.AssetID+"|"+r.AccountID]
		if item == nil {
			item = &DividendAssetRow{
				AccountID: r.AccountID, AssetID: r.AssetID, Code: r.Code, Name: r.Name,
				Years: make(map[string]float64),
			}
			assetMap[r.AssetID+"|"+r.AccountID] = item
		}
		item.Gross += r.Gross
		item.Tax += r.Tax
		item.Net += r.Net
		item.Count += r.Count
		item.Years[itoa(r.Year)] += r.Net
		resp.TotalGross += conv(r.Currency, r.Gross)
		resp.TotalTax += conv(r.Currency, r.Tax)
		resp.TotalNet += conv(r.Currency, r.Net)
	}
	for _, item := range assetMap {
		resp.ByAsset = append(resp.ByAsset, *item)
	}
	sort.Slice(resp.ByAsset, func(i, j int) bool { return resp.ByAsset[i].Net > resp.ByAsset[j].Net })
	resp.TotalGross = round4(resp.TotalGross)
	resp.TotalTax = round4(resp.TotalTax)
	resp.TotalNet = round4(resp.TotalNet)

	if n, err := s.repo.ReinvestCount(ctx, req.AccountID, req.Start, req.End); err == nil {
		resp.ReinvestCount = n
	}
	// YOC：近 12 月税后分红 / 当前持仓市值（CNY）
	trailingStart := addDays(todayStr(), -365)
	trailingRows, _ := s.repo.DividendAggs(ctx, req.AccountID, trailingStart, todayStr())
	trailingNet := 0.0
	for _, r := range trailingRows {
		trailingNet += conv(r.Currency, r.Net)
	}
	marketValue := 0.0
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: req.AccountID, Status: "holding"})
	if err == nil {
		for i := range positions {
			p := &positions[i]
			price := p.LastPrice
			if price <= 0 {
				price = p.AvgCost
			}
			marketValue += conv(p.Currency, price*p.Shares)
		}
	}
	if marketValue > 0 {
		resp.YieldOnCost = round6(trailingNet / marketValue)
	}
	return resp, nil
}

// ============================================================================
// 内部工具
// ============================================================================

// scopeName 范围名称
func scopeName(accountID string) string {
	if accountID == "" {
		return "global"
	}
	return accountID
}

// addDays 日期字符串加 N 天
func addDays(date string, days int) string {
	t, err := time.ParseInLocation(dateLayout, date, time.Local)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, days).Format(dateLayout)
}

// joinNote 合并说明文本
func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

// float64Ptr 浮点指针
func float64Ptr(v float64) *float64 { return &v }

// itoa 整数转字符串
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := [12]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
