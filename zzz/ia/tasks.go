// Package ia 定时任务层 — 行情刷新、自动计息、每日快照、到期检测
package ia

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/zzz/ia/quote"
)

const (
	autoAccrualRemark = "auto-accrual"
	daysPerYear       = 365.0 // ACT/365 计息基准
)

// AttachQuoteRegistry 注入行情注册表（模块初始化时调用）
func (s *Service) AttachQuoteRegistry(reg *quote.Registry) { s.quotes = reg }

// AttachWebhook 注入 webhook 服务（到期提醒等通知使用，可为 nil）
func (s *Service) AttachWebhook(wh *webhook.Service) { s.webhook = wh }

// ============================================================================
// 行情刷新
// ============================================================================

// RefreshQuotes 拉取全部可行情持仓的最新价/净值：
//   - 股票/ETF：官方实时价 → 写当日 price_history + 批量刷新持仓 last_price
//   - 基金/QDII：官方净值写 nav_date 历史行；估值优先作为市值参考（source=estimated）
func (s *Service) RefreshQuotes(ctx context.Context) (*RefreshResult, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	assets, err := s.repo.ListQuotableAssets(ctx)
	if err != nil {
		return nil, err
	}
	res := &RefreshResult{Scanned: len(assets), Errors: []string{}}

	for _, a := range assets {
		info := quote.AssetInfo{
			Code: a.Code, Market: a.Market, Currency: a.Currency,
			AssetType: a.AssetType, TrackingIndex: a.SubType,
		}
		q, err := s.quotes.Fetch(ctx, info)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s.%s: %v", a.Market, a.Code, err))
			continue
		}

		// 基金类：官方净值写入历史（按净值日期），供收益曲线使用；证券类写当日价格
		if a.AssetType == "fund" && q.Nav > 0 && q.NavDate != "" {
			if err = s.repo.UpsertPrice(ctx, a.ID, q.NavDate, q.Nav, string(PriceSourceOfficial)); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s 净值历史写入失败: %v", a.Code, err))
			}
		} else if a.AssetType != "fund" {
			if err = s.repo.UpsertPrice(ctx, a.ID, todayStr(), q.Price, string(PriceSourceOfficial)); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s 价格历史写入失败: %v", a.Code, err))
			}
		}

		if err = s.repo.UpdatePositionPriceByAsset(ctx, a.ID, q.Price, q.Source); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s 持仓市值更新失败: %v", a.Code, err))
			continue
		}
		res.Updated++

		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(quoteFetchInterval): // 接口礼貌间隔
		}
	}

	// WS 播报刷新结果
	s.notifyRoom("quote", map[string]any{
		"event": "quotes_refreshed", "scanned": res.Scanned,
		"updated": res.Updated, "failed": res.Failed, "at": time.Now().Format(time.RFC3339),
	})

	// 行情刷新后联动检查价格告警（失败不影响刷新结果）
	if res.Updated > 0 {
		go func() {
			actx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if _, err := s.CheckPriceAlerts(actx); err != nil {
				// 仅记录，告警检查有每日兜底任务
			}
		}()
	}
	return res, nil
}

// ============================================================================
// 存款理财自动计息
// ============================================================================

// AccrueInterest 存款/理财每日自动计息（幂等）。
// 规则：应计利息 = 本金 × 年化利率 / 365；每持仓每日至多一笔 auto-accrual 流水；
// 利息计入收益不改变份额成本；同时将持仓估值更新为本金+累计利息（source=accrued）。
func (s *Service) AccrueInterest(ctx context.Context) (*AccrueResult, error) {
	today := todayStr()
	assets, err := s.repo.ListInterestAssets(ctx, today)
	if err != nil {
		return nil, err
	}
	res := &AccrueResult{Scanned: len(assets), Errors: []string{}}

	for i := range assets {
		it := &assets[i]
		exists, err := s.repo.InterestAutoExistsToday(ctx, it.AccountID, it.AssetID, today)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s 幂等检查失败: %v", it.Name, err))
			continue
		}
		if exists {
			res.Skipped++
		} else {
			interest := round4(it.CostAmount * it.AnnualRate / daysPerYear)
			if interest >= 0.0001 {
				tradeAt, _ := time.ParseInLocation(dateLayout, today, time.Local)
				if _, err = s.repo.CreateTransaction(ctx, &Transaction{
					AccountID: it.AccountID, AssetID: it.AssetID, TradeType: string(TradeInterest),
					Amount: interest, TradeAt: tradeAt, Status: string(TxStatusConfirmed),
					Remark: autoAccrualRemark,
				}); err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("%s 计息流水写入失败: %v", it.Name, err))
					continue
				}
				if err = s.replay(ctx, it.AccountID, it.AssetID); err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("%s 重放失败: %v", it.Name, err))
					continue
				}
				res.Accrued++
				res.TotalInterest += interest
			} else {
				res.Skipped++
			}
		}

		// 估值更新：单位价值 = (本金 + 累计利息) / 份额，使市值包含应计利息
		totalInt, err := s.repo.SumAssetInterest(ctx, it.AccountID, it.AssetID)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s 累计利息查询失败: %v", it.Name, err))
			continue
		}
		unitValue := round6((it.CostAmount + totalInt) / it.Shares)
		if unitValue > 0 {
			if err = s.updatePosPrice(ctx, it.AccountID, it.AssetID, unitValue, string(PriceSourceAccrued)); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s 估值更新失败: %v", it.Name, err))
			}
		}
	}
	return res, nil
}

// updatePosPrice 更新单持仓最新价与来源
func (s *Service) updatePosPrice(ctx context.Context, accountID, assetID string, price float64, source string) error {
	const q = `UPDATE zzz_ia_positions SET last_price = ?, last_price_source = ?, last_price_at = NOW()
		WHERE account_id = ? AND asset_id = ? AND status = 'holding'`
	_, err := s.repo.db.ExecContext(ctx, q, price, source, accountID, assetID)
	return err
}

// ============================================================================
// 每日组合快照
// ============================================================================

// GenerateSnapshots 生成当日组合快照（各账户一行 + 全局一行，CNY 口径）
func (s *Service) GenerateSnapshots(ctx context.Context) (*SnapshotResult, error) {
	rates, err := s.repo.LatestFxRates(ctx)
	if err != nil {
		return nil, err
	}
	toCNY := cnyConverter(rates)

	type snapAgg struct {
		cost, value, pnl, cash float64
		byType                 map[string][2]float64 // category → {cost, value}
		byCcy                  map[string]float64    // currency → value（原币）
	}
	snaps := make(map[string]*snapAgg)
	getSnap := func(acct string) *snapAgg {
		if snaps[acct] == nil {
			snaps[acct] = &snapAgg{byType: make(map[string][2]float64), byCcy: make(map[string]float64)}
		}
		return snaps[acct]
	}

	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{Status: "holding"})
	if err != nil {
		return nil, err
	}
	for i := range positions {
		p := &positions[i]
		a := getSnap(p.AccountID)
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		v := toCNY(p.Currency, price*p.Shares)
		c := toCNY(p.Currency, p.CostAmount)
		a.cost += c
		a.value += v
		if cat := allocCategoryOf(p.Asset.AssetType); cat != "" {
			cur := a.byType[string(cat)]
			a.byType[string(cat)] = [2]float64{cur[0] + c, cur[1] + v}
		}
		a.byCcy[p.Currency] += price * p.Shares
	}

	balances, err := s.repo.CashBalances(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		getSnap(b.AccountID).cash += toCNY(b.Currency, b.Balance)
	}
	incomes, err := s.repo.IncomeAggs(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range incomes {
		a := getSnap(it.AccountID)
		a.pnl += toCNY(it.Currency, it.DividendNet+it.InterestIncome-it.TaxPaid)
	}
	realized, err := s.repo.RealizedAggs(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range realized {
		getSnap(it.AccountID).pnl += toCNY(it.Currency, it.NetPnL)
	}
	fees, err := s.repo.FeeSums(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range fees {
		getSnap(f.AccountID).pnl -= toCNY(f.Currency, f.Total)
	}

	date := todayStr()
	res := &SnapshotResult{Date: date}
	write := func(accountID string, agg *snapAgg) {
		agg.pnl += agg.value - agg.cost // 价格变动损益并入总盈亏
		byTypeJSON, _ := json.Marshal(agg.byType)
		byCcyJSON, _ := json.Marshal(agg.byCcy)
		row := &PortfolioSnapshot{
			SnapDate: date, AccountID: accountID,
			TotalCost: round4(agg.cost), TotalValue: round4(agg.value),
			TotalPnL: round4(agg.pnl), CashAmount: round4(agg.cash),
			ByType: string(byTypeJSON), ByCurrency: string(byCcyJSON),
		}
		if err = s.repo.SnapshotUpsert(ctx, row); err != nil {
			res.Failed++
		}
	}
	// 全局行 = 各账户聚合折叠
	g := &snapAgg{byType: make(map[string][2]float64), byCcy: make(map[string]float64)}
	keys := make([]string, 0, len(snaps))
	for k := range snaps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := snaps[k]
		write(k, v)
		g.cost += v.cost
		g.value += v.value
		g.pnl += v.pnl
		g.cash += v.cash
		for cat, cv := range v.byType {
			cur := g.byType[cat]
			g.byType[cat] = [2]float64{cur[0] + cv[0], cur[1] + cv[1]}
		}
		for ccy, val := range v.byCcy {
			g.byCcy[ccy] += val
		}
	}
	write("", g)
	res.Accounts = len(keys) + 1
	return res, nil
}

// ============================================================================
// 到期检测与告警
// ============================================================================

// MaturityAlertDays 到期提醒提前天数
const MaturityAlertDays = 7

// CheckMaturities 检测 N 天内到期的存款/理财并推送 webhook 提醒
func (s *Service) CheckMaturities(ctx context.Context) (*MaturityCheckResult, error) {
	rows, err := s.repo.ListMaturingAssets(ctx, MaturityAlertDays)
	if err != nil {
		return nil, err
	}
	res := &MaturityCheckResult{WithinDays: MaturityAlertDays, Count: len(rows), Items: []MaturingAssetView{}}
	if len(rows) == 0 {
		return res, nil
	}

	// 取持仓市值用于展示
	posValues := make(map[string]float64)
	positions, perr := s.repo.ListPositions(ctx, &ListPositionsReq{Status: "holding"})
	if perr == nil {
		for i := range positions {
			p := &positions[i]
			price := p.LastPrice
			if price <= 0 {
				price = p.AvgCost
			}
			posValues[p.AssetID] += price * p.Shares
		}
	}
	for i := range rows {
		res.Items = append(res.Items, MaturingAssetView{
			Code: rows[i].Code, Name: rows[i].Name, AssetType: rows[i].AssetType,
			MaturityDate: rows[i].MaturityDate, DaysLeft: rows[i].DaysLeft,
			Value: round4(posValues[rows[i].AssetID]),
		})
	}

	if s.webhook != nil {
		content := buildMaturityMessage(res.Items)
		_, sendErr := s.webhook.Send(ctx, &webhook.SendReq{
			Source: "zzz-ia", Title: "投资到期提醒", Content: content, MsgType: "markdown",
		})
		if sendErr == nil {
			res.Notified = true
		}
	}
	return res, nil
}

// buildMaturityMessage 构造到期提醒消息正文
func buildMaturityMessage(items []MaturingAssetView) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 投资到期提醒\n\n以下资产将在 %d 天内到期：\n\n", MaturityAlertDays))
	sb.WriteString("| 名称 | 类型 | 到期日 | 剩余天数 | 参考市值 |\n|---|---|---|---|---|\n")
	for _, it := range items {
		typeName := map[string]string{"deposit": "存款", "wealth": "理财"}[it.AssetType]
		if typeName == "" {
			typeName = it.AssetType
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %.2f |\n",
			it.Name, typeName, it.MaturityDate, it.DaysLeft, it.Value))
	}
	return sb.String()
}

// ============================================================================
// 内部工具
// ============================================================================

// quoteFetchInterval 行情拉取礼貌间隔
const quoteFetchInterval = 150 * time.Millisecond

// cnyConverter 生成 CNY 折算函数（缺汇率时该部分不计入）
func cnyConverter(rates map[string]float64) func(ccy string, v float64) float64 {
	return func(ccy string, v float64) float64 {
		if ccy == "CNY" || v == 0 {
			return v
		}
		if r, ok := rates["CNY->"+ccy]; ok && r > 0 {
			return v * r
		}
		return 0
	}
}
