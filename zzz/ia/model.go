// Package ia 个人投资分析模块（Investment Analytics）。
//
// 覆盖存款、理财、基金（含 QDII）、股票（个股/指数ETF）等资产类型，
// 提供多账户管理、资金流水、交易记录、份额与成本核算、费用与分红台账、
// 资产配比目标管理、汇率折算及定投计划等基础能力。
package ia

import (
	"time"
)

// ============================================================================
// 枚举定义
// ============================================================================

// AssetType 资产类型
type AssetType string

const (
	AssetTypeDeposit AssetType = "deposit" // 存款（含大额存单）
	AssetTypeWealth  AssetType = "wealth"  // 银行理财
	AssetTypeFund    AssetType = "fund"    // 基金（场外，含 QDII，用 sub_type 细分）
	AssetTypeStock   AssetType = "stock"   // 个股
	AssetTypeETF     AssetType = "etf"     // 指数 ETF
)

// AssetSubType 资产子类（基金细分等）
type AssetSubType string

const (
	AssetSubTypeQDII        AssetSubType = "qdii"         // QDII 基金
	AssetSubTypeEquity      AssetSubType = "equity"       // 股票型基金
	AssetSubTypeBond        AssetSubType = "bond"         // 债券型
	AssetSubTypeHybrid      AssetSubType = "hybrid"       // 混合型
	AssetSubTypeMoneyMarket AssetSubType = "money_market" // 货币基金
	AssetSubTypeIndex       AssetSubType = "index"        // 指数型
	AssetSubTypeNone        AssetSubType = ""             // 不适用
)

// Market 市场
type Market string

const (
	MarketCNSH Market = "CN-SH" // 上交所
	MarketCNSZ Market = "CN-SZ" // 深交所
	MarketHK   Market = "HK"    // 港股
	MarketUS   Market = "US"    // 美股
	MarketOTC  Market = "OTC"   // 场外（基金销售平台）
	MarketBank Market = "BANK"  // 银行（存款/理财）
)

// TradeType 交易流水类型
type TradeType string

const (
	TradeBuy           TradeType = "buy"               // 买入/申购
	TradeSell          TradeType = "sell"              // 卖出/赎回
	TradeDividendCash  TradeType = "dividend_cash"     // 现金分红（amount=税前，tax=税款）
	TradeDividendReinv TradeType = "dividend_reinvest" // 红利再投资
	TradeInterest      TradeType = "interest"          // 利息（存款到期/付息）
	TradeBonusShares   TradeType = "bonus_shares"      // 送股/转增/基金折算（shares=新增份额）
	TradeRightsIssue   TradeType = "rights_issue"      // 配股（缴款买入）
)

// TxStatus 交易确认状态
type TxStatus string

const (
	TxStatusPending   TxStatus = "pending"   // 在途（场外 T+1 确认前）
	TxStatusConfirmed TxStatus = "confirmed" // 已确认
)

// CashFlowType 资金流水类型
type CashFlowType string

const (
	CashFlowDepositIn   CashFlowType = "deposit"      // 入金（银证转入）
	CashFlowWithdrawOut CashFlowType = "withdraw"     // 出金（银证转出）
	CashFlowTransferIn  CashFlowType = "transfer_in"  // 账户间划入（不计收益）
	CashFlowTransferOut CashFlowType = "transfer_out" // 账户间划出（不计收益）
)

// CostMethod 成本核算方法
type CostMethod string

const (
	CostMethodWA   CostMethod = "wa"   // 加权平均
	CostMethodFIFO CostMethod = "fifo" // 先进先出
)

// AccountType 账户类型
type AccountType string

const (
	AccountTypeBroker AccountType = "broker" // 券商
	AccountTypeBank   AccountType = "bank"   // 银行
	AccountTypeFund   AccountType = "fund"   // 基金销售平台
	AccountTypeOther  AccountType = "other"
)

// FeeType 独立费用类型（仅记录净值外扣费用）
type FeeType string

const (
	FeeTypeManagement   FeeType = "management"    // 管理费（外扣部分）
	FeeTypeCustody      FeeType = "custody"       // 托管费
	FeeTypeSalesService FeeType = "sales_service" // 销售服务费
	FeeTypeAccount      FeeType = "account"       // 账户管理费
	FeeTypeGuarantee    FeeType = "guarantee"     // 保证保险费
	FeeTypeOther        FeeType = "other"
)

// PriceSource 价格来源
type PriceSource string

const (
	PriceSourceOfficial  PriceSource = "official"  // 官方行情/净值
	PriceSourceEstimated PriceSource = "estimated" // QDII 实时估值
	PriceSourceManual    PriceSource = "manual"    // 手工录入
	PriceSourceAccrued   PriceSource = "accrued"   // 存款理财计息估值
)

// DCACycle 定投周期
type DCACycle string

const (
	DCADaily    DCACycle = "daily"
	DCAWeekly   DCACycle = "weekly"
	DCABiWeekly DCACycle = "biweekly"
	DCAMonthly  DCACycle = "monthly"
)

// PlanStatus 计划状态
type PlanStatus string

const (
	PlanActive   PlanStatus = "active"
	PlanPaused   PlanStatus = "paused"
	PlanFinished PlanStatus = "finished"
)

// AllocCategory 配置维度类别（资产配比统计口径）
type AllocCategory string

const (
	AllocStock   AllocCategory = "stock"
	AllocETF     AllocCategory = "etf"
	AllocFund    AllocCategory = "fund"
	AllocDeposit AllocCategory = "deposit"
	AllocWealth  AllocCategory = "wealth"
	AllocCash    AllocCategory = "cash"
)

// ============================================================================
// 实体模型
// ============================================================================

// Account 投资账户，对应 zzz_ia_accounts 表
type Account struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Type       string    `json:"type" db:"type"`
	Broker     string    `json:"broker,omitempty" db:"broker"`
	Currency   string    `json:"currency" db:"currency"`
	CostMethod string    `json:"cost_method" db:"cost_method"`
	Status     string    `json:"status" db:"status"`
	Remark     string    `json:"remark,omitempty" db:"remark"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Asset 标的主数据，对应 zzz_ia_assets 表
type Asset struct {
	ID            string    `json:"id" db:"id"`
	Code          string    `json:"code" db:"code"`
	Name          string    `json:"name" db:"name"`
	AssetType     string    `json:"asset_type" db:"asset_type"`
	SubType       string    `json:"sub_type,omitempty" db:"sub_type"`
	Market        string    `json:"market" db:"market"`
	Currency      string    `json:"currency" db:"currency"`
	TrackingIndex string    `json:"tracking_index,omitempty" db:"tracking_index"` // QDII 参考指数代码
	AnnualRate    float64   `json:"annual_rate,omitempty" db:"annual_rate"`       // 约定年化利率（存款/理财），如 0.025
	StartDate     string    `json:"start_date,omitempty" db:"start_date"`         // 起息日 YYYY-MM-DD
	MaturityDate  string    `json:"maturity_date,omitempty" db:"maturity_date"`   // 到期日 YYYY-MM-DD
	LockDays      int       `json:"lock_days,omitempty" db:"lock_days"`           // 封闭期天数（理财）
	RiskLevel     string    `json:"risk_level,omitempty" db:"risk_level"`         // R1-R5
	Issuer        string    `json:"issuer,omitempty" db:"issuer"`                 // 发行方/管理人
	Status        string    `json:"status" db:"status"`                           // active / delisted / matured
	Remark        string    `json:"remark,omitempty" db:"remark"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// AssetResolveResp 标的代码解析结果（自动补全用）
type AssetResolveResp struct {
	// 基础信息
	Code      string `json:"code"`                // 证券代码
	Name      string `json:"name"`                // 证券名称
	Market    string `json:"market"`              // CN-SH / CN-SZ / HK / US
	Currency  string `json:"currency"`            // CNY / HKD / USD
	AssetType string `json:"asset_type"`          // stock / etf / fund
	SubType   string `json:"sub_type,omitempty"`  // equity / index / qdii 等

	// 行情数据
	Price        float64 `json:"price,omitempty"`         // 最新价
	ChangePct    float64 `json:"change_pct,omitempty"`    // 涨跌幅 %
	ChangeAmt    float64 `json:"change_amt,omitempty"`    // 涨跌额
	PrevClose    float64 `json:"prev_close,omitempty"`    // 昨收
	Open         float64 `json:"open,omitempty"`          // 开盘价
	High         float64 `json:"high,omitempty"`          // 最高价
	Low          float64 `json:"low,omitempty"`           // 最低价
	Volume       float64 `json:"volume,omitempty"`        // 成交量（手）
	Turnover     float64 `json:"turnover,omitempty"`      // 成交额
	TurnoverRate float64 `json:"turnover_rate,omitempty"` // 换手率 %
	HighLimit    float64 `json:"high_limit,omitempty"`    // 涨停价
	LowLimit     float64 `json:"low_limit,omitempty"`     // 跌停价

	// 估值指标
	PE        float64 `json:"pe,omitempty"`         // 市盈率(TTM)
	PEDynamic float64 `json:"pe_dynamic,omitempty"` // 市盈率(动态)
	PB        float64 `json:"pb,omitempty"`         // 市净率

	// 市值
	TotalMVYi float64 `json:"total_mv_yi,omitempty"` // 总市值（亿元）
	FloatMVYi float64 `json:"float_mv_yi,omitempty"` // 流通市值（亿元）
}

// CashFlow 资金流水（入金/出金/划转），对应 zzz_ia_cash_flows 表
type CashFlow struct {
	ID            int64     `json:"id" db:"id"`
	AccountID     string    `json:"account_id" db:"account_id"`
	FlowType      string    `json:"flow_type" db:"flow_type"`
	Amount        float64   `json:"amount" db:"amount"`
	Currency      string    `json:"currency" db:"currency"`
	FlowDate      string    `json:"flow_date" db:"flow_date"`                             // YYYY-MM-DD
	CounterAcctID string    `json:"counter_account_id,omitempty" db:"counter_account_id"` // 划转对手方账户
	Remark        string    `json:"remark,omitempty" db:"remark"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Transaction 交易流水，对应 zzz_ia_transactions 表。
// amount 语义：成交金额 = price × shares（不含费税）；fee 单独计；dividend_cash 时 amount 为税前金额、tax 为红利税。
type Transaction struct {
	ID          int64     `json:"id" db:"id"`
	AccountID   string    `json:"account_id" db:"account_id"`
	AssetID     string    `json:"asset_id" db:"asset_id"`
	TradeType   string    `json:"trade_type" db:"trade_type"`
	Shares      float64   `json:"shares" db:"shares"`
	Price       float64   `json:"price" db:"price"`
	Amount      float64   `json:"amount" db:"amount"`
	Fee         float64   `json:"fee" db:"fee"`
	FeeDetail   string    `json:"fee_detail,omitempty" db:"fee_detail"` // JSON 明细
	Tax         float64   `json:"tax" db:"tax"`
	TradeAt     time.Time `json:"trade_at" db:"trade_at"`
	SettleDate  string    `json:"settle_date,omitempty" db:"settle_date"`   // 确认日期 YYYY-MM-DD
	Status      string    `json:"status" db:"status"`                       // pending / confirmed
	Method      string    `json:"method,omitempty" db:"method"`             // 卖出结算成本法快照
	RealizedPnL float64   `json:"realized_pnl,omitempty" db:"realized_pnl"` // 卖出已实现收益冗余
	Remark      string    `json:"remark,omitempty" db:"remark"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// FeeRecord 独立费用记录，对应 zzz_ia_fees 表
type FeeRecord struct {
	ID         int64     `json:"id" db:"id"`
	AccountID  string    `json:"account_id" db:"account_id"`
	AssetID    string    `json:"asset_id,omitempty" db:"asset_id"`
	FeeType    string    `json:"fee_type" db:"fee_type"`
	Amount     float64   `json:"amount" db:"amount"`
	Currency   string    `json:"currency" db:"currency"`
	ChargeDate string    `json:"charge_date" db:"charge_date"` // YYYY-MM-DD
	Period     string    `json:"period,omitempty" db:"period"` // 计费周期，如 2026-Q3
	Remark     string    `json:"remark,omitempty" db:"remark"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Position 实时持仓，对应 zzz_ia_positions 表（由交易重放引擎维护，只读；
// target_price/stop_loss_price 为用户设置的追踪目标，重放不覆盖）
type Position struct {
	ID              int64      `json:"id" db:"id"`
	AccountID       string     `json:"account_id" db:"account_id"`
	AssetID         string     `json:"asset_id" db:"asset_id"`
	Shares          float64    `json:"shares" db:"shares"`
	AvailableShares float64    `json:"available_shares" db:"available_shares"` // 可用份额（扣除锁定）
	LockedShares    float64    `json:"locked_shares" db:"locked_shares"`
	AvgCost         float64    `json:"avg_cost" db:"avg_cost"`
	CostAmount      float64    `json:"cost_amount" db:"cost_amount"`
	LastPrice       float64    `json:"last_price,omitempty" db:"last_price"`
	LastPriceAt     *time.Time `json:"last_price_at,omitempty" db:"last_price_at"`
	LastPriceSrc    string     `json:"last_price_source,omitempty" db:"last_price_source"`
	TargetPrice     float64    `json:"target_price,omitempty" db:"target_price"`       // 追踪目标价（0=未设置）
	StopLossPrice   float64    `json:"stop_loss_price,omitempty" db:"stop_loss_price"` // 止损参考价（0=未设置）
	TargetSetAt     *time.Time `json:"target_set_at,omitempty" db:"target_set_at"`
	Currency        string     `json:"currency" db:"currency"`
	Status          string     `json:"status" db:"status"` // holding / closed
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`

	// 动态计算字段（不入库）
	Asset       *Asset  `json:"asset,omitempty" db:"-"`
	MarketValue float64 `json:"market_value,omitempty" db:"-"`
	UnrealPnL   float64 `json:"unrealized_pnl,omitempty" db:"-"`
	PnLRatio    float64 `json:"pnl_ratio,omitempty" db:"-"`
}

// RealizedPnL 已实现收益台账，对应 zzz_ia_realized_pnl 表（由交易重放引擎重建）
type RealizedPnL struct {
	ID               int64   `json:"id" db:"id"`
	AccountID        string  `json:"account_id" db:"account_id"`
	AssetID          string  `json:"asset_id" db:"asset_id"`
	TransactionID    int64   `json:"transaction_id" db:"transaction_id"`                   // 卖出流水
	LotTransactionID int64   `json:"lot_transaction_id,omitempty" db:"lot_transaction_id"` // FIFO 匹配买入批次
	Method           string  `json:"method" db:"method"`
	Shares           float64 `json:"shares" db:"shares"`
	SellPrice        float64 `json:"sell_price" db:"sell_price"`
	CostPrice        float64 `json:"cost_price" db:"cost_price"`
	GrossPnL         float64 `json:"gross_pnl" db:"gross_pnl"`
	Fee              float64 `json:"fee" db:"fee"`
	Tax              float64 `json:"tax" db:"tax"`
	NetPnL           float64 `json:"net_pnl" db:"net_pnl"`
	RealizedAt       string  `json:"realized_at" db:"realized_at"` // YYYY-MM-DD
}

// DCAPlan 定投计划，对应 zzz_ia_dca_plans 表
type DCAPlan struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	AccountID    string    `json:"account_id" db:"account_id"`
	AssetID      string    `json:"asset_id" db:"asset_id"`
	Amount       float64   `json:"amount" db:"amount"`
	Cycle        string    `json:"cycle" db:"cycle"`       // daily / weekly / biweekly / monthly
	ExecDay      int       `json:"exec_day" db:"exec_day"` // 周几(1-7) 或 几号(1-28)
	StartDate    string    `json:"start_date" db:"start_date"`
	EndDate      string    `json:"end_date,omitempty" db:"end_date"`
	NextExecDate string    `json:"next_exec_date" db:"next_exec_date"`
	Status       string    `json:"status" db:"status"` // active / paused / finished
	AutoBuy      bool      `json:"auto_buy" db:"auto_buy"`
	Remark       string    `json:"remark,omitempty" db:"remark"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	Account *Account `json:"account,omitempty" db:"-"`
	Asset   *Asset   `json:"asset,omitempty" db:"-"`
}

// FxRate 汇率记录，对应 zzz_ia_fx_rates 表。rate = 1 单位 quote_ccy 兑换 base_ccy 的数量。
type FxRate struct {
	ID       int64   `json:"id" db:"id"`
	BaseCcy  string  `json:"base_ccy" db:"base_ccy"`   // 如 CNY
	QuoteCcy string  `json:"quote_ccy" db:"quote_ccy"` // 如 USD
	Rate     float64 `json:"rate" db:"rate"`           // 1 USD = 7.16 CNY 则 rate=7.16
	RateDate string  `json:"rate_date" db:"rate_date"` // YYYY-MM-DD
	Source   string  `json:"source" db:"source"`       // manual / auto
}

// AllocationItem 资产配比目标项，对应 zzz_ia_allocations 表。
// account_id 为空表示全局方案；同 scope 下 category 唯一，全部 target_ratio 之和应为 100%。
type AllocationItem struct {
	ID          string    `json:"id" db:"id"`
	AccountID   string    `json:"account_id,omitempty" db:"account_id"` // 空=全局
	Category    string    `json:"category" db:"category"`               // stock/etf/fund/deposit/wealth/cash
	TargetRatio float64   `json:"target_ratio" db:"target_ratio"`       // 0-1，如 0.4
	Tolerance   float64   `json:"tolerance" db:"tolerance"`             // 允许偏差阈值，如 0.05
	Status      string    `json:"status" db:"status"`                   // active / inactive
	Remark      string    `json:"remark,omitempty" db:"remark"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================================
// 请求 DTO
// ============================================================================

// CreateAccountReq 创建账户请求
type CreateAccountReq struct {
	Name       string `json:"name" binding:"required,max=100"`
	Type       string `json:"type" binding:"required"`
	Broker     string `json:"broker,omitempty"`
	Currency   string `json:"currency,omitempty"`    // 默认 CNY
	CostMethod string `json:"cost_method,omitempty"` // 默认 wa
	Remark     string `json:"remark,omitempty"`
}

// UpdateAccountReq 更新账户请求
type UpdateAccountReq struct {
	Name       *string `json:"name,omitempty"`
	Type       *string `json:"type,omitempty"`
	Broker     *string `json:"broker,omitempty"`
	Currency   *string `json:"currency,omitempty"`
	CostMethod *string `json:"cost_method,omitempty"`
	Status     *string `json:"status,omitempty"`
	Remark     *string `json:"remark,omitempty"`
}

// ListAccountsReq 账户列表请求
type ListAccountsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

func (r *ListAccountsReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListAccountsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateAssetReq 创建标的请求
type CreateAssetReq struct {
	Code          string  `json:"code" binding:"required,max=32"`
	Name          string  `json:"name" binding:"required,max=100"`
	AssetType     string  `json:"asset_type" binding:"required"`
	SubType       string  `json:"sub_type,omitempty"`
	Market        string  `json:"market" binding:"required"`
	Currency      string  `json:"currency,omitempty"`
	TrackingIndex string  `json:"tracking_index,omitempty"`
	AnnualRate    float64 `json:"annual_rate,omitempty"`
	StartDate     string  `json:"start_date,omitempty"`
	MaturityDate  string  `json:"maturity_date,omitempty"`
	LockDays      int     `json:"lock_days,omitempty"`
	RiskLevel     string  `json:"risk_level,omitempty"`
	Issuer        string  `json:"issuer,omitempty"`
	Remark        string  `json:"remark,omitempty"`
}

// UpdateAssetReq 更新标的请求
type UpdateAssetReq struct {
	Code          *string  `json:"code,omitempty"`
	Name          *string  `json:"name,omitempty"`
	AssetType     *string  `json:"asset_type,omitempty"`
	SubType       *string  `json:"sub_type,omitempty"`
	Market        *string  `json:"market,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
	TrackingIndex *string  `json:"tracking_index,omitempty"`
	AnnualRate    *float64 `json:"annual_rate,omitempty"`
	StartDate     *string  `json:"start_date,omitempty"`
	MaturityDate  *string  `json:"maturity_date,omitempty"`
	LockDays      *int     `json:"lock_days,omitempty"`
	RiskLevel     *string  `json:"risk_level,omitempty"`
	Issuer        *string  `json:"issuer,omitempty"`
	Status        *string  `json:"status,omitempty"`
	Remark        *string  `json:"remark,omitempty"`
}

// ListAssetsReq 标的列表请求
type ListAssetsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AssetType string `form:"asset_type"`
	Market    string `form:"market"`
	Status    string `form:"status"`
	Keyword   string `form:"keyword"`
}

func (r *ListAssetsReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListAssetsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateCashFlowReq 创建资金流水请求
type CreateCashFlowReq struct {
	AccountID     string  `json:"account_id" binding:"required"`
	FlowType      string  `json:"flow_type" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Currency      string  `json:"currency,omitempty"`
	FlowDate      string  `json:"flow_date,omitempty"` // 默认今天
	CounterAcctID string  `json:"counter_account_id,omitempty"`
	Remark        string  `json:"remark,omitempty"`
}

// ListCashFlowsReq 资金流水的表请求
type ListCashFlowsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AccountID string `form:"account_id"`
	FlowType  string `form:"flow_type"`
	Start     string `form:"start"` // YYYY-MM-DD
	End       string `form:"end"`
}

func (r *ListCashFlowsReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListCashFlowsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateTransactionReq 创建交易流水请求。
// buy/sell/rights_issue：price 与 shares 必填；dividend_cash：amount=税前分红额；dividend_reinvest：price=除息净值；bonus_shares：shares=新增份额；
// interest：amount=利息额。status 缺省 confirmed，场外申购在途可传 pending。
// 买入时可携带 target_price/stop_loss_price 一并设置持仓追踪目标。
type CreateTransactionReq struct {
	AccountID     string             `json:"account_id" binding:"required"`
	AssetID       string             `json:"asset_id" binding:"required"`
	TradeType     string             `json:"trade_type" binding:"required"`
	Shares        float64            `json:"shares,omitempty"`
	Price         float64            `json:"price,omitempty"`
	Amount        float64            `json:"amount,omitempty"`
	Fee           float64            `json:"fee,omitempty"`
	FeeDetail     map[string]float64 `json:"fee_detail,omitempty"`
	Tax           float64            `json:"tax,omitempty"`
	TradeAt       string             `json:"trade_at,omitempty"` // RFC3339 或 YYYY-MM-DD HH:MM:SS，默认当前时间
	SettleDate    string             `json:"settle_date,omitempty"`
	Status        string             `json:"status,omitempty"`
	Remark        string             `json:"remark,omitempty"`
	TargetPrice   float64            `json:"target_price,omitempty"`    // 买入追踪目标价（可选）
	StopLossPrice float64            `json:"stop_loss_price,omitempty"` // 止损参考价（可选）
}

// ListTransactionsReq 交易列表请求
type ListTransactionsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AccountID string `form:"account_id"`
	AssetID   string `form:"asset_id"`
	TradeType string `form:"trade_type"`
	Status    string `form:"status"`
	Start     string `form:"start"`
	End       string `form:"end"`
}

func (r *ListTransactionsReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListTransactionsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateFeeReq 创建独立费用请求
type CreateFeeReq struct {
	AccountID  string  `json:"account_id" binding:"required"`
	AssetID    string  `json:"asset_id,omitempty"`
	FeeType    string  `json:"fee_type" binding:"required"`
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	Currency   string  `json:"currency,omitempty"`
	ChargeDate string  `json:"charge_date,omitempty"`
	Period     string  `json:"period,omitempty"`
	Remark     string  `json:"remark,omitempty"`
}

// ListFeesReq 费用列表请求
type ListFeesReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AccountID string `form:"account_id"`
	AssetID   string `form:"asset_id"`
	FeeType   string `form:"fee_type"`
	Start     string `form:"start"`
	End       string `form:"end"`
}

func (r *ListFeesReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListFeesReq) Offset() int { return (r.Page - 1) * r.PageSize }

// ListPositionsReq 持仓列表请求
type ListPositionsReq struct {
	AccountID string `form:"account_id"`
	AssetType string `form:"asset_type"` // 按标的类型过滤（联表 assets）
	Status    string `form:"status"`     // 默认 holding
}

// CreateDCAPlanReq 创建定投计划
type CreateDCAPlanReq struct {
	Name      string  `json:"name" binding:"required,max=100"`
	AccountID string  `json:"account_id" binding:"required"`
	AssetID   string  `json:"asset_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	Cycle     string  `json:"cycle" binding:"required"`
	ExecDay   int     `json:"exec_day" binding:"required,min=1,max=28"`
	StartDate string  `json:"start_date,omitempty"`
	EndDate   string  `json:"end_date,omitempty"`
	AutoBuy   bool    `json:"auto_buy,omitempty"`
	Remark    string  `json:"remark,omitempty"`
}

// UpdateDCAPlanReq 更新定投计划
type UpdateDCAPlanReq struct {
	Name    *string  `json:"name,omitempty"`
	Amount  *float64 `json:"amount,omitempty"`
	Cycle   *string  `json:"cycle,omitempty"`
	ExecDay *int     `json:"exec_day,omitempty"`
	EndDate *string  `json:"end_date,omitempty"`
	Status  *string  `json:"status,omitempty"`
	AutoBuy *bool    `json:"auto_buy,omitempty"`
	Remark  *string  `json:"remark,omitempty"`
}

// ListDCAPlansReq 定投计划列表
type ListDCAPlansReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AccountID string `form:"account_id"`
	AssetID   string `form:"asset_id"`
	Status    string `form:"status"`
}

func (r *ListDCAPlansReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListDCAPlansReq) Offset() int { return (r.Page - 1) * r.PageSize }

// UpsertFxRateReq 录入/更新汇率
type UpsertFxRateReq struct {
	BaseCcy  string  `json:"base_ccy" binding:"required,max=8"`
	QuoteCcy string  `json:"quote_ccy" binding:"required,max=8"`
	Rate     float64 `json:"rate" binding:"required,gt=0"`
	RateDate string  `json:"rate_date,omitempty"`
	Source   string  `json:"source,omitempty"`
}

// ListFxRatesReq 汇率查询
type ListFxRatesReq struct {
	BaseCcy  string `form:"base_ccy"`
	QuoteCcy string `form:"quote_ccy"`
	Start    string `form:"start"`
	End      string `form:"end"`
}

// CreateAllocationReq 创建配置目标项
type CreateAllocationReq struct {
	AccountID   string  `json:"account_id,omitempty"` // 空=全局
	Category    string  `json:"category" binding:"required"`
	TargetRatio float64 `json:"target_ratio" binding:"required,gt=0,lt=1"`
	Tolerance   float64 `json:"tolerance,omitempty"`
	Remark      string  `json:"remark,omitempty"`
}

// UpdateAllocationReq 更新配置目标项
type UpdateAllocationReq struct {
	TargetRatio *float64 `json:"target_ratio,omitempty"`
	Tolerance   *float64 `json:"tolerance,omitempty"`
	Status      *string  `json:"status,omitempty"`
	Remark      *string  `json:"remark,omitempty"`
}

// ListAllocationsReq 配置列表
type ListAllocationsReq struct {
	AccountID string `form:"account_id"` // 空=全局
	Status    string `form:"status"`
}

// ============================================================================
// 响应 DTO
// ============================================================================

// normalizePage 分页参数规范化
func normalizePage(page, pageSize *int) {
	if *page < 1 {
		*page = 1
	}
	if *pageSize < 1 || *pageSize > 100 {
		*pageSize = 20
	}
}

// PaginatedResp 分页响应
type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

// PositionView 持仓视图（含动态估值字段）
type PositionView struct {
	Position
	MarketValue   float64 `json:"market_value"`   // 市值（无最新价时按成本兜底）
	UnrealizedPnL float64 `json:"unrealized_pnl"` // 浮动盈亏
	PnLRatio      float64 `json:"pnl_ratio"`      // 浮动收益率
	PriceValued   bool    `json:"price_valued"`   // 是否有真实行情估值（false=按成本兜底）
}

// SetPositionTargetReq 设置持仓追踪目标（买入后可随时调整）
type SetPositionTargetReq struct {
	AccountID     string  `json:"account_id" binding:"required"`
	AssetID       string  `json:"asset_id" binding:"required"`
	TargetPrice   float64 `json:"target_price"`    // 追踪目标价，0=清除
	StopLossPrice float64 `json:"stop_loss_price"` // 止损参考价，0=清除
}

// AdviceAction 建议动作
type AdviceAction string

const (
	AdviceBuy    AdviceAction = "buy"    // 建议加仓/买入
	AdviceHold   AdviceAction = "hold"   // 继续持有
	AdviceWatch  AdviceAction = "watch"  // 观望/关注
	AdviceReduce AdviceAction = "reduce" // 建议部分减持
	AdviceSell   AdviceAction = "sell"   // 建议卖出/止盈离场
)

// TradeAdviceItem 单持仓买卖建议
type TradeAdviceItem struct {
	AccountID      string   `json:"account_id"`
	AssetID        string   `json:"asset_id"`
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	AssetType      string   `json:"asset_type"`
	Action         string   `json:"action"`     // buy/hold/watch/reduce/sell
	Confidence     int      `json:"confidence"` // 建议置信度 0-100
	Reasons        []string `json:"reasons"`    // 触发规则说明
	CostPrice      float64  `json:"cost_price"` // 买入成本（卖出建议锚点）
	LastPrice      float64  `json:"last_price"`
	PnLRatio       float64  `json:"pnl_ratio"`
	TargetPrice    float64  `json:"target_price,omitempty"`    // 追踪目标
	StopLossPrice  float64  `json:"stop_loss_price,omitempty"` // 止损参考
	TargetProgress float64  `json:"target_progress,omitempty"` // 向目标推进进度 (last-cost)/(target-cost)
	AboveMA20      *bool    `json:"above_ma20,omitempty"`      // 技术面（可得时返回）
	AboveMA60      *bool    `json:"above_ma60,omitempty"`
	MaturityDays   int      `json:"maturity_days,omitempty"` // 存款理财剩余天数（-1=不适用）
}

// TradeAdviceResp 买卖建议响应
type TradeAdviceResp struct {
	Scope       string            `json:"scope"`
	GeneratedAt string            `json:"generated_at"`
	Items       []TradeAdviceItem `json:"items"`
	BuyCount    int               `json:"buy_count"`
	HoldCount   int               `json:"hold_count"`
	ReduceCount int               `json:"reduce_count"`
	SellCount   int               `json:"sell_count"`
}

// OverviewSummary 汇总概览（单币种口径）
type OverviewSummary struct {
	TotalCost      float64                    `json:"total_cost"`
	TotalValue     float64                    `json:"total_value"`
	TotalPnL       float64                    `json:"total_pnl"`
	PnLRatio       float64                    `json:"pnl_ratio"`
	RealizedPnL    float64                    `json:"realized_pnl"`
	DividendIncome float64                    `json:"dividend_income"`
	TaxPaid        float64                    `json:"tax_paid"`
	InterestIncome float64                    `json:"interest_income"`
	FeeTotal       float64                    `json:"fee_total"`
	CashBalance    float64                    `json:"cash_balance"`              // 入金-出金净额
	ByCurrency     map[string]OverviewSummary `json:"by_currency,omitempty"`     // 仅顶层汇总填充
	TotalValueCNY  float64                    `json:"total_value_cny,omitempty"` // 汇率折算后（可得的币种）
}

// OverviewResp 总览响应
type OverviewResp struct {
	Accounts map[string]OverviewSummary `json:"accounts"` // 按 account_id 分组汇总
	Grand    OverviewSummary            `json:"grand"`    // 全局汇总
}

// AllocationReviewItem 配置回顾项：实际 vs 目标
type AllocationReviewItem struct {
	Category    string  `json:"category"`
	TargetRatio float64 `json:"target_ratio"`
	ActualRatio float64 `json:"actual_ratio"`
	ValueCNY    float64 `json:"value_cny"`
	Deviation   float64 `json:"deviation"` // actual - target
	Breach      bool    `json:"breach"`    // 偏差超出容忍阈值
}

// AllocationReviewResp 配置回顾响应
type AllocationReviewResp struct {
	AccountID string                 `json:"account_id,omitempty"` // 空=全局
	Items     []AllocationReviewItem `json:"items"`
	Uncovered []string               `json:"uncovered"` // 有实际持仓但未设置目标的类别
}

// ============================================================================
// 快照与定时任务 DTO
// ============================================================================

// PortfolioSnapshot 组合每日快照，对应 zzz_ia_portfolio_snapshots 表（account_id 空=全局）
type PortfolioSnapshot struct {
	ID         int64   `json:"id" db:"id"`
	SnapDate   string  `json:"snap_date" db:"snap_date"`
	AccountID  string  `json:"account_id,omitempty" db:"account_id"`
	TotalCost  float64 `json:"total_cost" db:"total_cost"`
	TotalValue float64 `json:"total_value" db:"total_value"`
	TotalPnL   float64 `json:"total_pnl" db:"total_pnl"`
	CashAmount float64 `json:"cash_amount" db:"cash_amount"`
	ByType     string  `json:"by_type,omitempty" db:"by_type"`         // JSON: 类别 → {cost,value}（CNY 口径）
	ByCurrency string  `json:"by_currency,omitempty" db:"by_currency"` // JSON: 币种 → value（原币口径）
}

// PriceHistory 价格/净值历史视图
type PriceHistory struct {
	AssetID string  `json:"asset_id"`
	NavDate string  `json:"nav_date"`
	Nav     float64 `json:"nav"`
	Source  string  `json:"source"`
}

// RefreshResult 行情刷新结果统计
type RefreshResult struct {
	Scanned int      `json:"scanned"` // 扫描的标的数
	Updated int      `json:"updated"` // 成功更新市值数
	Failed  int      `json:"failed"`  // 失败数
	Errors  []string `json:"errors,omitempty"`
}

// AccrueResult 自动计息结果统计
type AccrueResult struct {
	Scanned       int      `json:"scanned"`        // 计息持仓数
	Accrued       int      `json:"accrued"`        // 当日新计提笔数
	Skipped       int      `json:"skipped"`        // 幂等跳过数
	TotalInterest float64  `json:"total_interest"` // 当日计提利息总额
	Errors        []string `json:"errors,omitempty"`
}

// SnapshotResult 快照生成结果统计
type SnapshotResult struct {
	Date     string `json:"date"`
	Accounts int    `json:"accounts"` // 含全局行
	Failed   int    `json:"failed"`
}

// MaturityCheckResult 到期检测结果统计
type MaturityCheckResult struct {
	WithinDays int                 `json:"within_days"`
	Count      int                 `json:"count"`
	Notified   bool                `json:"notified"` // 是否已推送 webhook
	Items      []MaturingAssetView `json:"items,omitempty"`
}

// MaturingAssetView 即将到期资产视图
type MaturingAssetView struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	AssetType    string  `json:"asset_type"`
	MaturityDate string  `json:"maturity_date"`
	DaysLeft     int     `json:"days_left"`
	Value        float64 `json:"value"`
}

// ============================================================================
// 分析报表 DTO（P4）
// ============================================================================

// AnalyticsReq 分析类接口通用请求：account_id 为空 = 全局口径
type AnalyticsReq struct {
	AccountID string `form:"account_id"`
	Start     string `form:"start"` // YYYY-MM-DD
	End       string `form:"end"`
}

// EquityCurvePoint 收益曲线点
type EquityCurvePoint struct {
	Date     string  `json:"date"`
	Value    float64 `json:"value"`     // 总资产（含现金，CNY）
	Cost     float64 `json:"cost"`      // 持仓成本
	PnL      float64 `json:"pnl"`       // 累计盈亏
	PnLRatio float64 `json:"pnl_ratio"` // 盈亏/成本
}

// BenchmarkPoint 基准归一化点（基期=100）
type BenchmarkPoint struct {
	Date  string  `json:"date"`
	Index float64 `json:"index"`
}

// EquityCurveResp 收益曲线响应
type EquityCurveResp struct {
	Scope         string             `json:"scope"` // global / 账户ID
	Points        []EquityCurvePoint `json:"points"`
	BenchmarkName string             `json:"benchmark_name,omitempty"`
	Benchmark     []BenchmarkPoint   `json:"benchmark,omitempty"` // 与组合曲线日期对齐
}

// ReturnsResp 区间收益指标响应
type ReturnsResp struct {
	Scope           string   `json:"scope"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	StartValue      float64  `json:"start_value"`
	EndValue        float64  `json:"end_value"`                // 含现金
	NetExternalFlow float64  `json:"net_external_flow"`        // 入金-出金（划转不计）
	SimpleReturn    float64  `json:"simple_return"`            // 简单区间收益率
	MoneyWeighted   *float64 `json:"money_weighted,omitempty"` // Modified Dietz
	XIRR            *float64 `json:"xirr,omitempty"`           // 年化资金加权收益率
	FlowCount       int      `json:"flow_count"`
	Note            string   `json:"note,omitempty"` // 数据不足等说明
}

// TradeReviewBuy 加仓批次复盘
type TradeReviewBuy struct {
	TxID          int64   `json:"tx_id"`
	Date          string  `json:"date"`
	Price         float64 `json:"price"`
	Shares        float64 `json:"shares"`
	Amount        float64 `json:"amount"`
	AvgBefore     float64 `json:"avg_before"`
	AvgAfter      float64 `json:"avg_after"`
	DilutionRatio float64 `json:"dilution_ratio"` // (新均价-旧均价)/旧均价，负=摊薄
	PerfSince     float64 `json:"perf_since"`     // 当前价相对买价涨跌
}

// TradeReviewSell 减持批次复盘
type TradeReviewSell struct {
	TxID           int64   `json:"tx_id"`
	Date           string  `json:"date"`
	Price          float64 `json:"price"`
	Shares         float64 `json:"shares"`
	NetPnL         float64 `json:"net_pnl"`
	ReturnRate     float64 `json:"return_rate"`     // 相对卖出成本价
	OpportunityGap float64 `json:"opportunity_gap"` // (当前价-卖价)/卖价，正=卖早了
	Win            bool    `json:"win"`
}

// AssetTradeReview 单标的交易复盘
type AssetTradeReview struct {
	AccountID string            `json:"account_id"`
	AssetID   string            `json:"asset_id"`
	Code      string            `json:"code"`
	Name      string            `json:"name"`
	Currency  string            `json:"currency"`
	Buys      []TradeReviewBuy  `json:"buys"`
	Sells     []TradeReviewSell `json:"sells"`
	WinRate   float64           `json:"win_rate"`
}

// TradesReviewSummary 复盘汇总
type TradesReviewSummary struct {
	TotalBuys     int     `json:"total_buys"`
	TotalSells    int     `json:"total_sells"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	WinRate       float64 `json:"win_rate"`
	AvgWinReturn  float64 `json:"avg_win_return"`
	AvgLossReturn float64 `json:"avg_loss_return"`
	ProfitFactor  float64 `json:"profit_factor"` // 总盈利/总亏损绝对值
	TotalRealized float64 `json:"total_realized"`
}

// tradesAccum 复盘内部累计器（非序列化）
type tradesAccum struct {
	wins, losses          int
	winRetSum, lossRetSum float64
	gainSum, lossAbsSum   float64
	totalSells            int
}

// TradesReviewResp 加仓减持复盘响应
type TradesReviewResp struct {
	Scope   string              `json:"scope"`
	Assets  []AssetTradeReview  `json:"assets"`
	Summary TradesReviewSummary `json:"summary"`
}

// ConcentrationItem 集中度条目
type ConcentrationItem struct {
	AssetID string  `json:"asset_id"`
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Weight  float64 `json:"weight"`
	Breach  bool    `json:"breach"`
}

// RiskMetricsResp 风险与集中度响应
type RiskMetricsResp struct {
	Scope                  string              `json:"scope"`
	StartDate              string              `json:"start_date"`
	EndDate                string              `json:"end_date"`
	SampleDays             int                 `json:"sample_days"`
	MaxDrawdown            float64             `json:"max_drawdown"`
	MDDPeakDate            string              `json:"mdd_peak_date,omitempty"`
	MDDTroughDate          string              `json:"mdd_trough_date,omitempty"`
	AnnualVolatility       float64             `json:"annual_volatility"`
	TotalValue             float64             `json:"total_value"`
	ConcentrationThreshold float64             `json:"concentration_threshold"`
	Concentration          []ConcentrationItem `json:"concentration"`
	Top1Weight             float64             `json:"top1_weight"`
	Top5Weight             float64             `json:"top5_weight"`
	HHI                    float64             `json:"hhi"` // Herfindahl 指数，Σ权重²
	Note                   string              `json:"note,omitempty"`
}

// DCASimulateReq 定投模拟请求
type DCASimulateReq struct {
	AssetID string  `form:"asset_id" binding:"required"`
	Cycle   string  `form:"cycle" binding:"required"` // daily/weekly/biweekly/monthly
	ExecDay int     `form:"exec_day" binding:"required,min=1,max=28"`
	Amount  float64 `form:"amount" binding:"required,gt=0"`
	Start   string  `form:"start" binding:"required"` // YYYY-MM-DD
	End     string  `form:"end"`
}

// DCASimulateResp 定投模拟响应：定投 vs 一次性投入对比
type DCASimulateResp struct {
	AssetID        string  `json:"asset_id"`
	Cycle          string  `json:"cycle"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	ExecCount      int     `json:"exec_count"`
	TotalInvested  float64 `json:"total_invested"`
	SharesAccum    float64 `json:"shares_accum"`
	FinalNav       float64 `json:"final_nav"`
	FinalValue     float64 `json:"final_value"`
	ReturnRate     float64 `json:"return_rate"`
	LumpSumValue   float64 `json:"lump_sum_value"`
	LumpReturnRate float64 `json:"lump_return_rate"`
	Advantage      float64 `json:"advantage"` // 定投收益率 - 一次性收益率
}

// FeesReportResp 费用分析响应
type FeesReportResp struct {
	Scope            string             `json:"scope"`
	StartDate        string             `json:"start_date,omitempty"`
	EndDate          string             `json:"end_date,omitempty"`
	TotalTradeFees   float64            `json:"total_trade_fees"`
	TotalTradeTax    float64            `json:"total_trade_tax"`
	StandaloneByType map[string]float64 `json:"standalone_by_type"`
	TotalStandalone  float64            `json:"total_standalone"`
	Grand            float64            `json:"grand"`
	ByYear           []FeeYearRow       `json:"by_year"`
	BaseValue        float64            `json:"base_value"` // 侵蚀度分母（最新快照总值或当前市值）
	DragRatio        float64            `json:"drag_ratio"` // 费用/资产基数
}

// FeeYearRow 年度费用行
type FeeYearRow struct {
	Year           int     `json:"year"`
	TradeFees      float64 `json:"trade_fees"`
	TradeTax       float64 `json:"trade_tax"`
	StandaloneFees float64 `json:"standalone_fees"`
	Total          float64 `json:"total"`
}

// DividendAssetRow 分红标的排行行
type DividendAssetRow struct {
	AccountID string             `json:"account_id"`
	AssetID   string             `json:"asset_id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Gross     float64            `json:"gross"`
	Tax       float64            `json:"tax"`
	Net       float64            `json:"net"`
	Count     int                `json:"count"`
	Years     map[string]float64 `json:"years"` // 年度 → 税后净额（原币）
}

// DividendReportResp 分红统计响应
type DividendReportResp struct {
	Scope         string             `json:"scope"`
	TotalGross    float64            `json:"total_gross"`
	TotalTax      float64            `json:"total_tax"`
	TotalNet      float64            `json:"total_net"`
	ReinvestCount int                `json:"reinvest_count"`
	YieldOnCost   float64            `json:"yield_on_cost"` // 近12月税后分红 / 当前持仓市值(CNY)
	ByAsset       []DividendAssetRow `json:"by_asset"`
}

// ============================================================================
// 价格告警（P5）
// ============================================================================

// AlertType 价格告警类型
type AlertType string

const (
	AlertTakeProfit AlertType = "take_profit"  // 止盈：最新价 ≥ 目标价
	AlertStopLoss   AlertType = "stop_loss"    // 止损：最新价 ≤ 目标价
	AlertTargetHit  AlertType = "target_price" // 到价：最新价进入目标价 ±1% 区间
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertActive    AlertStatus = "active"
	AlertTriggered AlertStatus = "triggered"
	AlertDisabled  AlertStatus = "disabled"
)

// PriceAlert 价格告警规则，对应 zzz_ia_price_alerts 表
type PriceAlert struct {
	ID             string     `json:"id" db:"id"`
	AccountID      string     `json:"account_id" db:"account_id"`
	AssetID        string     `json:"asset_id" db:"asset_id"`
	AlertType      string     `json:"alert_type" db:"alert_type"`
	TargetPrice    float64    `json:"target_price" db:"target_price"`
	Status         string     `json:"status" db:"status"`
	TriggeredAt    *time.Time `json:"triggered_at,omitempty" db:"triggered_at"`
	TriggeredPrice float64    `json:"triggered_price,omitempty" db:"triggered_price"`
	Note           string     `json:"note,omitempty" db:"note"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`

	Asset *Asset `json:"asset,omitempty" db:"-"`
}

// CreateAlertReq 创建价格告警
type CreateAlertReq struct {
	AccountID   string  `json:"account_id" binding:"required"`
	AssetID     string  `json:"asset_id" binding:"required"`
	AlertType   string  `json:"alert_type" binding:"required"`
	TargetPrice float64 `json:"target_price" binding:"required,gt=0"`
	Note        string  `json:"note,omitempty"`
}

// UpdateAlertReq 更新价格告警
type UpdateAlertReq struct {
	TargetPrice *float64 `json:"target_price,omitempty"`
	Status      *string  `json:"status,omitempty"` // active(重新武装)/disabled
	Note        *string  `json:"note,omitempty"`
}

// ListAlertsReq 告警列表
type ListAlertsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	AccountID string `form:"account_id"`
	AssetID   string `form:"asset_id"`
	Status    string `form:"status"`
}

func (r *ListAlertsReq) Normalize()  { normalizePage(&r.Page, &r.PageSize) }
func (r *ListAlertsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// AlertTriggeredView 触发的告警视图
type AlertTriggeredView struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	AlertType   string  `json:"alert_type"`
	TargetPrice float64 `json:"target_price"`
	LastPrice   float64 `json:"last_price"`
	AccountID   string  `json:"account_id"`
}

// AlertCheckResult 告警检查结果
type AlertCheckResult struct {
	Checked   int                  `json:"checked"` // active 规则数
	Triggered int                  `json:"triggered"`
	Items     []AlertTriggeredView `json:"items,omitempty"`
	Notified  bool                 `json:"notified"`
}

// ============================================================================
// 对账单导入（P5）
// ============================================================================

// ImportTxReq CSV 对账单导入请求
type ImportTxReq struct {
	AccountID   string `json:"account_id" form:"account_id" binding:"required"`
	CSV         string `json:"csv" form:"csv"`                   // 直接传 CSV 文本（与二选一）
	AutoCreate  bool   `json:"auto_create" form:"auto_create"`   // 缺失标的时自动创建（默认资产类型 stock/CNY）
	DefaultType string `json:"default_type" form:"default_type"` // 自动创建时的默认类型，默认 stock
}

// ImportResult 导入结果统计
type ImportResult struct {
	TotalRows   int      `json:"total_rows"`             // 数据行数（不含表头）
	Created     int      `json:"created"`                // 新建交易数
	Duplicates  int      `json:"duplicates"`             // 查重跳过数
	Failed      int      `json:"failed"`                 // 失败行数
	AssetsAdded []string `json:"assets_added,omitempty"` // 自动创建的标的 code
	Errors      []string `json:"errors,omitempty"`
}

// ============================================================================
// 个股筛选与指数分析（P6）
// ============================================================================

// DynamicPotentialReq 动态潜力筛选请求
type DynamicPotentialReq struct {
	Top      int `form:"top"`       // 返回条数，默认 20，最大 50
	PoolSize int `form:"pool_size"` // 候选池大小（按主力净流入预选），默认 50，最大 100
	Pages    int `form:"pages"`     // 快照拉取页数（每页100只），默认 2
}

// PotentialStock 动态潜力个股条目
type PotentialStock struct {
	Code            string             `json:"code"`
	Name            string             `json:"name"`
	Price           float64            `json:"price"`
	ChangePct       float64            `json:"change_pct"`        // 当日涨跌幅 %
	TurnoverRate    float64            `json:"turnover_rate"`     // 换手率 %
	MainInflowRatio float64            `json:"main_inflow_ratio"` // 主力净流入/流通市值
	R5              float64            `json:"r5"`                // 近5日涨幅 %
	R20             float64            `json:"r20"`               // 近20日涨幅 %
	R60             float64            `json:"r60"`               // 近60日涨幅 %
	AboveMA20       bool               `json:"above_ma20"`
	AboveMA60       bool               `json:"above_ma60"`
	VolAnnual       float64            `json:"vol_annual"`  // 年化波动率
	Score           float64            `json:"score"`       // 综合潜力分 0-100
	ScoreParts      map[string]float64 `json:"score_parts"` // 分项得分（便于调参理解）
}

// DynamicPotentialResp 动态潜力筛选响应
type DynamicPotentialResp struct {
	GeneratedAt string           `json:"generated_at"`
	Universe    int              `json:"universe"` // 快照池总数
	Analyzed    int              `json:"analyzed"` // 完成动量分析的候选数
	Items       []PotentialStock `json:"items"`
}

// QualityStockReq 绩优个股筛选请求
type QualityStockReq struct {
	Top     int     `form:"top"`       // 返回条数，默认 30，最大 100
	Pages   int     `form:"pages"`     // 快照页数，默认 3
	MinROE  float64 `form:"min_roe"`   // ROE 下限 %，默认 10
	MaxPE   float64 `form:"max_pe"`    // PE-TTM 上限，默认 35
	MaxPB   float64 `form:"max_pb"`    // PB 上限，默认 8
	MinMVYi float64 `form:"min_mv_yi"` // 总市值下限（亿元），默认 50
}

// QualityStock 绩优个股条目
type QualityStock struct {
	Code         string             `json:"code"`
	Name         string             `json:"name"`
	Price        float64            `json:"price"`
	ChangePct    float64            `json:"change_pct"`
	PETTM        float64            `json:"pe_ttm"`
	PEDynamic    float64            `json:"pe_dynamic"`
	PB           float64            `json:"pb"`
	ROE          float64            `json:"roe"`
	ProfitGrowth float64            `json:"profit_growth"`
	TotalMVYi    float64            `json:"total_mv_yi"`
	Score        float64            `json:"score"` // 质量分 0-100
	ScoreParts   map[string]float64 `json:"score_parts"`
}

// QualityStockResp 绩优个股筛选响应
type QualityStockResp struct {
	GeneratedAt string         `json:"generated_at"`
	Universe    int            `json:"universe"`
	Filtered    int            `json:"filtered"` // 通过硬性过滤的数量
	Items       []QualityStock `json:"items"`
}

// IndexAnalysisReq 指数分析请求
type IndexAnalysisReq struct {
	Indexes string `form:"indexes"` // 逗号分隔别名或 secid，默认 NASDAQ100,NASDAQ,SP500,STAR50,CSI300
	Years   int    `form:"years"`   // 回看年数 1-10，默认 3
}

// IndexMetrics 单指数分析指标
type IndexMetrics struct {
	Secid       string  `json:"secid"`
	Name        string  `json:"name"`
	Latest      float64 `json:"latest"`
	ChangeYTD   float64 `json:"change_ytd"` // 年初至今涨幅 %
	MA20        float64 `json:"ma20"`
	MA60        float64 `json:"ma60"`
	MA120       float64 `json:"ma120"`
	MA250       float64 `json:"ma250"`
	Trend       string  `json:"trend"` // 多头排列/多头震荡/空头震荡/空头排列
	AboveMA20   bool    `json:"above_ma20"`
	AboveMA60   bool    `json:"above_ma60"`
	AboveMA120  bool    `json:"above_ma120"`
	AboveMA250  bool    `json:"above_ma250"`
	WindowHigh  float64 `json:"window_high"`
	WindowLow   float64 `json:"window_low"`
	Percentile  float64 `json:"percentile"`   // 当前点位在窗口高低区间分位 0-1
	AnnReturn   float64 `json:"ann_return"`   // 窗口年化收益
	AnnVol      float64 `json:"ann_vol"`      // 年化波动率
	MaxDrawdown float64 `json:"max_drawdown"` // 窗口最大回撤
	Samples     int     `json:"samples"`
	Error       string  `json:"error,omitempty"` // 单指数获取失败原因
}

// IndexAnalysisResp 指数分析响应
type IndexAnalysisResp struct {
	Years       int            `json:"years"`
	GeneratedAt string         `json:"generated_at"`
	Items       []IndexMetrics `json:"items"`
}

// FxRefreshResult 汇率刷新结果
type FxRefreshResult struct {
	Updated []string           `json:"updated"` // 更新的币对 CNY->USD
	Rates   map[string]float64 `json:"rates"`   // 最新汇率
	Errors  []string           `json:"errors,omitempty"`
}
