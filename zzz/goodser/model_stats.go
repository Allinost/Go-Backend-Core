package goodser

import "time"

// StatsOverview 库存目录统计总览，供店主控制台、n8n / OpenClaw 使用。
type StatsOverview struct {
	InventoryID   string            `json:"inventory_id"`
	InventoryName string            `json:"inventory_name"`
	Days          int               `json:"days"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Stock         StatsStock        `json:"stock"`
	Orders        StatsOrderCounts  `json:"orders"`
	Trend         []StatsTrendPoint `json:"trend"`
	Series        []StatsSeriesRow  `json:"series"`
	TopProducts   []StatsTopProduct `json:"top_products"`
	Profit        StatsProfit       `json:"profit"`
}

// StatsProfit 统计区间内的收益汇总与单品排行。
// 售价取期望价（为 0 时取市场价），成本取成本价（为 0 时取原价）。
type StatsProfit struct {
	Revenue     float64              `json:"revenue"`
	Cost        float64              `json:"cost"`
	Profit      float64              `json:"profit"`
	MarginRate  float64              `json:"margin_rate"`
	TopByProfit []StatsProfitProduct `json:"top_by_profit"`
	TopByMargin []StatsProfitProduct `json:"top_by_margin"`
}

// StatsProfitProduct 单品收益明细，MarginRate 为 0-1 的收益率。
type StatsProfitProduct struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	Revenue     float64 `json:"revenue"`
	Cost        float64 `json:"cost"`
	Profit      float64 `json:"profit"`
	MarginRate  float64 `json:"margin_rate"`
}

// StatsProfitRow 收益统计的数据库行，价格用于在服务层计算。
type StatsProfitRow struct {
	ProductID     string
	ProductName   string
	ProductCode   string
	OutboundQty   int
	ReturnQty     int
	ExpectedPrice float64
	MarketPrice   float64
	CostPrice     float64
	OriginalPrice float64
}

// StatsStock 库存数量与价值汇总。
// Available 为可售库存（quantity），Reserved 为已锁定库存（reserved_quantity），
// Total = Available + Reserved，与 D1 预留模型保持一致。
type StatsStock struct {
	TotalProducts int     `json:"total_products"`
	Available     int     `json:"available"`
	Reserved      int     `json:"reserved"`
	Total         int     `json:"total"`
	StockValue    float64 `json:"stock_value"`
	OutOfStock    int     `json:"out_of_stock"`
}

// StatsOrderCounts 统计区间内的订单与退货单数量。
type StatsOrderCounts struct {
	PendingOutbound   int `json:"pending_outbound"`
	ConfirmedOutbound int `json:"confirmed_outbound"`
	CancelledOutbound int `json:"cancelled_outbound"`
	PendingReserves   int `json:"pending_reserves"`
	PendingReturns    int `json:"pending_returns"`
	ConfirmedReturns  int `json:"confirmed_returns"`
}

// StatsTrendPoint 单日出库确认量 / 退货回库量。
type StatsTrendPoint struct {
	Date             string `json:"date"`
	OutboundQuantity int    `json:"outbound_quantity"`
	ReturnQuantity   int    `json:"return_quantity"`
}

// StatsSeriesRow 按系列聚合的库存分布。
type StatsSeriesRow struct {
	SeriesID     *string `json:"series_id"`
	SeriesName   string  `json:"series_name"`
	ProductCount int     `json:"product_count"`
	Available    int     `json:"available"`
	Reserved     int     `json:"reserved"`
	Total        int     `json:"total"`
	StockValue   float64 `json:"stock_value"`
}

// StatsTopProduct 统计区间内出库确认量最高的商品。
type StatsTopProduct struct {
	ProductID        string `json:"product_id"`
	ProductName      string `json:"product_name"`
	ProductCode      string `json:"product_code"`
	OutboundQuantity int    `json:"outbound_quantity"`
}

// NormalizeStatsDays 约束统计区间，默认 30 天，范围 1-365 天。
func NormalizeStatsDays(days int) int {
	if days < 1 {
		return 30
	}
	if days > 365 {
		return 365
	}
	return days
}
