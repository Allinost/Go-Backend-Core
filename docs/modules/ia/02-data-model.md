# 投资分析数据模型

## 1. 数据库

- **数据库**: MySQL `main` 连接池（`database.DB.MySQL["main"]`）
- **表名**: `zzz_ia_{表名}`，共 **13 张**
- **建表方式**: `migrations/mysql/008_legacy_modules_schema.sql` 统一 `CREATE TABLE IF NOT EXISTS`（P8 收敛，模块启动不再建表）
- **列兼容**: 目标价列已包含在 `migrations/mysql/008_legacy_modules_schema.sql` 的建表 DDL 中（P8 统一处理）
- **ORM**: 不用 GORM；repository 为原生 `database/sql` + 手写 SQL，model 用 `db:`/`json:` tag

---

## 2. 表结构

### 2.1 账户与标的

**`zzz_ia_accounts`** — 账户

| 字段 | 说明 |
|------|------|
| id (PK) | UUID |
| name / type / currency | 名称；类型 broker/bank/fund/other；币种 |
| cost_method | 成本核算方法：`wa`（加权平均）/ `fifo` |
| status | 状态 |

**`zzz_ia_assets`** — 标的

| 字段 | 说明 |
|------|------|
| id (PK) | UUID |
| code + market | 证券代码 + 市场，**唯一** |
| asset_type / sub_type | deposit/wealth/fund/stock/etf；子类型如 qdii |
| annual_rate / start_date / maturity_date / lock_days | 存款理财：年化利率、起息/到期日、锁定期 |
| risk_level | R1–R5 |
| issuer / tracking_index | 发行人 / 跟踪指数 |

**`zzz_ia_cash_flows`** — 资金流水

| 字段 | 说明 |
|------|------|
| flow_type | deposit / withdraw / transfer_in / transfer_out |
| amount / counter_account_id | 金额 / 对方账户（划转时） |

### 2.2 交易与持仓

**`zzz_ia_transactions`** — 交易记录

| 字段 | 说明 |
|------|------|
| trade_type | buy / sell / dividend_cash / dividend_reinvest / interest / bonus_shares / rights_issue |
| shares / price / amount / fee / tax | 量价与费用 |
| fee_detail | JSON 明细 |
| status | `pending`（在途）/ `confirmed` |
| method / realized_pnl | 成本法 / 该笔卖出的已实现收益 |

**`zzz_ia_positions`** — 持仓（重放推导，`module.go:347`）

| 字段 | 说明 |
|------|------|
| shares / available / locked | 持有 / 可用 / 锁定份额 |
| avg_cost / cost_amount | 单位成本 / 总成本 |
| last_price / last_price_at / last_price_source | 最新价（手工 or 行情） |
| **target_price / stop_loss_price / target_set_at** | 追踪目标价/止损价（**重放不覆盖**，存量列由 migrate 补齐） |
| status | holding / closed |
| （动态）asset / market_value / unreal_pnl / pnl_ratio | `db:"-"`，查询时计算 |

**`zzz_ia_realized_pnl`** — 已实现收益

| 字段 | 说明 |
|------|------|
| transaction_id / lot_transaction_id | 卖出交易 / FIFO 批次来源交易 |
| method | wa / fifo |
| gross_pnl / net_pnl | 毛收益 / 净收益 |

### 2.3 计划、汇率与配比

**`zzz_ia_dca_plans`** — 定投计划：cycle（daily/weekly/biweekly/monthly）、exec_day、next_exec_date、status、auto_buy

**`zzz_ia_fx_rates`** — 汇率：base_ccy/quote_ccy/rate/rate_date，唯一键 `(pair, date)`；`rate` = 1 单位 quote 兑换 base 所需 CNY 数

**`zzz_ia_allocations`** — 资产配比目标：account_id 空 = 全局、category（stock/etf/fund/deposit/wealth/cash）、target_ratio（0–1）、tolerance 容差

### 2.4 快照、费用与告警

**`zzz_ia_portfolio_snapshots`** — 每日快照：snap_date、account_id 空 = 全局、total_cost/value/pnl、cash_amount、by_type/by_currency（JSON）

**`zzz_ia_fees`** — 独立费用：fee_type（management/custody/sales_service/account/guarantee/other）、charge_date、period

**`zzz_ia_price_alerts`** — 价格告警：alert_type（take_profit/stop_loss/target_price）、target_price、status（active/triggered/disabled）、triggered_at/triggered_price

**`zzz_ia_price_history`** — 价格/净值历史：asset_id、nav_date、nav、source；唯一键 `(asset_id, nav_date)`，由行情刷新与快照写入

### 2.5 ER 概览

```
accounts ──< cash_flows
accounts ──< transactions >── assets
accounts ──< positions >── assets ──< price_history
transactions ──< realized_pnl          (卖出收益, FIFO 记录批次)
accounts ──< portfolio_snapshots       (account_id 空 = 全局)
assets ──< dca_plans                   (定投计划)
assets ──< price_alerts
全局 ──< allocations                   (account_id 空 = 全局)
全局 ──< fx_rates
全局 ──< fees
```

---

## 3. Go 数据模型要点

- 所有实体定义在 `model.go`（1256 行），tag 为 `db:"..."` + `json:"..."`，**无 gorm tag**
- 关键实体行号：`Account`(159)、`Asset`(173)、`CashFlow`(229)、`Transaction`(243)、`FeeRecord`(265)、`Position`(280)、`RealizedPnL`(308)、`DCAPlan`(326)、`FxRate`(348)、`AllocationItem`(359)、`PortfolioSnapshot`(755)、`PriceAlert`(1058)
- 主键由 `github.com/google/uuid` 生成

---

## 4. DTO

### 4.1 主要枚举（`model.go`）

| 枚举 | 行号 | 取值 |
|------|------|------|
| `AssetType` | 17 | deposit / wealth / fund / stock / etf |
| `Market` | 41 | CN-SH / CN-SZ / HK / US 等 |
| `TradeType` | 53 | buy / sell / dividend_cash / dividend_reinvest / interest / bonus_shares / rights_issue |
| `TxStatus` | 66 | pending / confirmed |
| `CashFlowType` | 74 | deposit / withdraw / transfer_in / transfer_out |
| `CostMethod` | 84 | wa / fifo |
| `AccountType` | 92 | broker / bank / fund / other |
| `DCACycle` | 124 | daily / weekly / biweekly / monthly |
| `AllocCategory` | 143 | stock / etf / fund / deposit / wealth / cash |
| `AlertType` / `AlertStatus` | 1040 / 1049 | take_profit / stop_loss / target_price；active / triggered / disabled |
| `AdviceAction` | 669 | 建议动作（buy / sell / hold 等） |

### 4.2 通用请求/响应

```go
// 分页响应（泛型）
type PaginatedResp[T any] struct {
    Items   []T   `json:"items"`
    HasMore bool  `json:"has_more"`
    Total   int   `json:"total"`
}

// 列表请求均含 page / page_size，另按域带筛选参数
// （如 ListTransactionsReq 含 account_id/asset_id/trade_type/date range）
```

主要请求 DTO 行号：`CreateAccountReq`(376)、`CreateAssetReq`(409)、`CreateCashFlowReq`(459)、`CreateTransactionReq`(486)、`CreateFeeReq`(520)、`CreateDCAPlanReq`(553)、`UpsertFxRateReq`(591)、`CreateAllocationReq`(608)、`SetPositionTargetReq`(661)、`AnalyticsReq`(822)、`DCASimulateReq`(961)、`CreateAlertReq`(1075)、`ImportTxReq`(1125)、`DynamicPotentialReq`(1147)、`QualityStockReq`(1180)、`IndexAnalysisReq`(1214)

主要响应 DTO：`PositionView`(652)、`TradeAdviceResp`(701)、`OverviewResp`(728)、`AllocationReviewResp`(744)、`EquityCurveResp`(844)、`ReturnsResp`(852)、`TradesReviewResp`(925)、`RiskMetricsResp`(942)、`DCASimulateResp`(971)、`FeesReportResp`(988)、`DividendReportResp`(1025)、`AlertCheckResult`(1113)、`ImportResult`(1133)
