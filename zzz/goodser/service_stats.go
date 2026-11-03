package goodser

import (
	"context"
	"sort"
	"time"
)

// GetStatsOverview 汇总目录库存、订单、趋势与系列分布。
func (s *Service) GetStatsOverview(ctx context.Context, inventoryID string, days int) (*StatsOverview, error) {
	inventory, err := s.repo.GetInventory(ctx, inventoryID)
	if err != nil {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	days = NormalizeStatsDays(days)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))

	stock, err := s.repo.StatsStock(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	stock.StockValue = roundMoney(stock.StockValue)

	orders, err := s.repo.StatsOrderCounts(ctx, inventoryID, start)
	if err != nil {
		return nil, err
	}

	series, err := s.repo.StatsSeries(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	for i := range series {
		series[i].StockValue = roundMoney(series[i].StockValue)
	}

	trend, err := s.repo.StatsTrend(ctx, inventoryID, start)
	if err != nil {
		return nil, err
	}

	top, err := s.repo.StatsTopProducts(ctx, inventoryID, start, 10)
	if err != nil {
		return nil, err
	}

	profitRows, err := s.repo.StatsProfit(ctx, inventoryID, start)
	if err != nil {
		return nil, err
	}
	profit := buildStatsProfit(profitRows, 10)

	return &StatsOverview{
		InventoryID:   inventory.ID,
		InventoryName: inventory.Name,
		Days:          days,
		GeneratedAt:   now,
		Stock:         *stock,
		Orders:        *orders,
		Trend:         fillTrend(trend, start, days),
		Series:        nonNilSeries(series),
		TopProducts:   nonNilTopProducts(top),
		Profit:        profit,
	}, nil
}

// buildStatsProfit 按净出库量计算整体收益与单品排行。
// 售价 = 期望价（为 0 时取市场价），成本 = 成本价（为 0 时取原价）。
func buildStatsProfit(rows []StatsProfitRow, limit int) StatsProfit {
	items := make([]StatsProfitProduct, 0, len(rows))
	var totalRevenue, totalCost float64
	for _, row := range rows {
		qty := row.OutboundQty - row.ReturnQty
		if qty <= 0 {
			continue
		}
		sale := row.ExpectedPrice
		if sale <= 0 {
			sale = row.MarketPrice
		}
		cost := row.CostPrice
		if cost <= 0 {
			cost = row.OriginalPrice
		}
		revenue := roundMoney(sale * float64(qty))
		costTotal := roundMoney(cost * float64(qty))
		profit := roundMoney(revenue - costTotal)
		margin := 0.0
		if revenue > 0 {
			margin = profit / revenue
		}
		items = append(items, StatsProfitProduct{
			ProductID:   row.ProductID,
			ProductName: row.ProductName,
			ProductCode: row.ProductCode,
			Quantity:    qty,
			Revenue:     revenue,
			Cost:        costTotal,
			Profit:      profit,
			MarginRate:  margin,
		})
		totalRevenue += revenue
		totalCost += costTotal
	}
	out := StatsProfit{
		Revenue:     roundMoney(totalRevenue),
		Cost:        roundMoney(totalCost),
		Profit:      roundMoney(totalRevenue - totalCost),
		TopByProfit: []StatsProfitProduct{},
		TopByMargin: []StatsProfitProduct{},
	}
	if out.Revenue > 0 {
		out.MarginRate = out.Profit / out.Revenue
	}
	if limit < 1 {
		limit = 10
	}
	byProfit := append([]StatsProfitProduct(nil), items...)
	sort.SliceStable(byProfit, func(i, j int) bool {
		if byProfit[i].Profit == byProfit[j].Profit {
			return byProfit[i].Quantity > byProfit[j].Quantity
		}
		return byProfit[i].Profit > byProfit[j].Profit
	})
	if len(byProfit) > limit {
		byProfit = byProfit[:limit]
	}
	out.TopByProfit = byProfit
	if out.TopByProfit == nil {
		out.TopByProfit = []StatsProfitProduct{}
	}

	byMargin := make([]StatsProfitProduct, 0, len(items))
	for _, item := range items {
		if item.Revenue > 0 {
			byMargin = append(byMargin, item)
		}
	}
	sort.SliceStable(byMargin, func(i, j int) bool {
		if byMargin[i].MarginRate == byMargin[j].MarginRate {
			return byMargin[i].Profit > byMargin[j].Profit
		}
		return byMargin[i].MarginRate > byMargin[j].MarginRate
	})
	if len(byMargin) > limit {
		byMargin = byMargin[:limit]
	}
	out.TopByMargin = byMargin
	if out.TopByMargin == nil {
		out.TopByMargin = []StatsProfitProduct{}
	}
	return out
}

// fillTrend 补齐区间内没有流水的日期，方便前端直接绘图。
func fillTrend(points []StatsTrendPoint, start time.Time, days int) []StatsTrendPoint {
	byDate := make(map[string]StatsTrendPoint, len(points))
	for _, point := range points {
		byDate[point.Date] = point
	}
	out := make([]StatsTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		point := byDate[date]
		point.Date = date
		out = append(out, point)
	}
	return out
}

func nonNilSeries(items []StatsSeriesRow) []StatsSeriesRow {
	if items == nil {
		return []StatsSeriesRow{}
	}
	return items
}

func nonNilTopProducts(items []StatsTopProduct) []StatsTopProduct {
	if items == nil {
		return []StatsTopProduct{}
	}
	return items
}
