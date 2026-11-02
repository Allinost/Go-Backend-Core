package goodser

import (
	"context"
	"database/sql"
	"time"
)

// stockValueExpr 计算单件商品库存价值：优先预期价，其次市场价。
const stockValueExpr = `COALESCE(NULLIF(p.expected_price, 0), p.market_price, 0)`

// StatsStock 汇总目录库存数量与价值。
func (r *Repository) StatsStock(ctx context.Context, inventoryID string) (*StatsStock, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(p.quantity), 0),
		        COALESCE(SUM(p.reserved_quantity), 0),
		        COALESCE(SUM(p.quantity + p.reserved_quantity), 0),
		        COALESCE(SUM((p.quantity + p.reserved_quantity) * `+stockValueExpr+`), 0),
		        COALESCE(SUM(CASE WHEN p.quantity + p.reserved_quantity = 0 THEN 1 ELSE 0 END), 0)
		 FROM `+tableProducts+` p
		 WHERE p.inventory_id = ?`, inventoryID)
	var out StatsStock
	if err := row.Scan(&out.TotalProducts, &out.Available, &out.Reserved, &out.Total, &out.StockValue, &out.OutOfStock); err != nil {
		return nil, err
	}
	return &out, nil
}

// StatsOrderCounts 汇总区间内订单与退货单状态数量。
func (r *Repository) StatsOrderCounts(ctx context.Context, inventoryID string, since time.Time) (*StatsOrderCounts, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT
		    COALESCE(SUM(CASE WHEN type = 'outbound' AND status = 'pending' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN type = 'outbound' AND status IN ('confirmed', 'partially_returned', 'returned') THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN type = 'outbound' AND status = 'cancelled' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN type = 'reserve' AND status = 'pending' THEN 1 ELSE 0 END), 0)
		 FROM `+tableOutboundOrders+`
		 WHERE inventory_id = ? AND created_at >= ?`, inventoryID, since)
	var out StatsOrderCounts
	if err := row.Scan(&out.PendingOutbound, &out.ConfirmedOutbound, &out.CancelledOutbound, &out.PendingReserves); err != nil {
		return nil, err
	}

	row = r.db.QueryRowContext(ctx,
		`SELECT
		    COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'confirmed' THEN 1 ELSE 0 END), 0)
		 FROM `+tableReturns+`
		 WHERE inventory_id = ? AND created_at >= ?`, inventoryID, since)
	if err := row.Scan(&out.PendingReturns, &out.ConfirmedReturns); err != nil {
		return nil, err
	}
	return &out, nil
}

// StatsTrend 按天汇总出库确认量与退货回库量。
func (r *Repository) StatsTrend(ctx context.Context, inventoryID string, since time.Time) ([]StatsTrendPoint, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DATE(created_at) AS day,
		        COALESCE(SUM(CASE WHEN reason = ? THEN -reserved_delta ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN reason = ? THEN quantity_delta ELSE 0 END), 0)
		 FROM `+tableStockMovements+`
		 WHERE inventory_id = ? AND created_at >= ?
		 GROUP BY DATE(created_at)
		 ORDER BY day`, StockReasonOutboundConfirm, StockReasonReturnRestock, inventoryID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StatsTrendPoint
	for rows.Next() {
		var (
			day   time.Time
			point StatsTrendPoint
		)
		if err := rows.Scan(&day, &point.OutboundQuantity, &point.ReturnQuantity); err != nil {
			return nil, err
		}
		point.Date = day.Format("2006-01-02")
		items = append(items, point)
	}
	return items, rows.Err()
}

// StatsSeries 按系列聚合库存。
func (r *Repository) StatsSeries(ctx context.Context, inventoryID string) ([]StatsSeriesRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.series_id, COALESCE(d.name, ''),
		        COUNT(*),
		        COALESCE(SUM(p.quantity), 0),
		        COALESCE(SUM(p.reserved_quantity), 0),
		        COALESCE(SUM(p.quantity + p.reserved_quantity), 0),
		        COALESCE(SUM((p.quantity + p.reserved_quantity) * `+stockValueExpr+`), 0)
		 FROM `+tableProducts+` p
		 LEFT JOIN `+tableDictionaries+` d ON d.id = p.series_id AND d.kind = 'series'
		 WHERE p.inventory_id = ?
		 GROUP BY p.series_id, d.name
		 ORDER BY SUM(p.quantity + p.reserved_quantity) DESC, COUNT(*) DESC`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StatsSeriesRow
	for rows.Next() {
		var (
			item       StatsSeriesRow
			seriesID   sql.NullString
			seriesName string
		)
		if err := rows.Scan(&seriesID, &seriesName, &item.ProductCount, &item.Available, &item.Reserved, &item.Total, &item.StockValue); err != nil {
			return nil, err
		}
		if seriesID.Valid && seriesID.String != "" {
			item.SeriesID = &seriesID.String
			item.SeriesName = seriesName
		} else {
			item.SeriesName = "未设置"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// StatsTopProducts 统计区间内出库确认量最高的商品。
func (r *Repository) StatsTopProducts(ctx context.Context, inventoryID string, since time.Time, limit int) ([]StatsTopProduct, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT m.product_id, COALESCE(p.name, ''), COALESCE(p.code, ''),
		        COALESCE(SUM(-m.reserved_delta), 0) AS qty
		 FROM `+tableStockMovements+` m
		 LEFT JOIN `+tableProducts+` p ON p.id = m.product_id
		 WHERE m.inventory_id = ? AND m.created_at >= ? AND m.reason = ?
		 GROUP BY m.product_id, p.name, p.code
		 HAVING SUM(-m.reserved_delta) > 0
		 ORDER BY SUM(-m.reserved_delta) DESC
		 LIMIT ?`, inventoryID, since, StockReasonOutboundConfirm, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StatsTopProduct
	for rows.Next() {
		var item StatsTopProduct
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.ProductCode, &item.OutboundQuantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
