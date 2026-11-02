package goodser

import (
	"context"
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
	}, nil
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
