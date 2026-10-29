# 投资分析 API 设计

## 1. 统一规范

- 路由前缀：`/api/v1/zzz-ia`
- 统一响应格式同项目规范（`response.Response`：`code` / `message` / `data`）
- 分页响应结构：`{ "items": [...], "has_more": bool, "total": int }`
- 错误码段：`42xxx`（见 [04-module-structure.md](04-module-structure.md)）
- 错误处理统一在 `handler.go:1431-1449` `handleError`：`IAError` → 业务码；`sql.ErrNoRows` → 404；`pkgNet.ErrCircuitOpen` → 42303；其余 → 500

---

## 2. 账户与标的

### 2.1 账户

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/accounts` | 列表（分页/类型/状态/关键词） |
| POST | `/accounts` | 创建（name、type、currency、cost_method） |
| GET | `/accounts/:id` | 详情 |
| PUT | `/accounts/:id` | 更新 |
| DELETE | `/accounts/:id` | 删除（有持仓/流水时拒绝，`ErrAccountHasPositions`） |

### 2.2 标的

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/assets` | 列表 |
| GET | `/assets/resolve?code=` | 证券代码解析：调东方财富补全名称/行情/PE/PB |
| POST | `/assets` | 创建 |
| GET/PUT/DELETE | `/assets/:id` | 详情 / 更新 / 删除 |

---

## 3. 资金、交易与持仓

### 3.1 资金流水

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/cash-flows` | 入金/出金/划转（划转要求双方账户不同） |
| GET | `/cash-flows` | 查询 |
| DELETE | `/cash-flows/:id` | 删除 |

### 3.2 交易

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/transactions` | 记录买入/卖出/分红/送转等，落库后触发重放 |
| GET | `/transactions` | 查询 |
| GET | `/transactions/:id` | 详情 |
| POST | `/transactions/:id/confirm` | 确认在途（场外 T+1）交易 |
| DELETE | `/transactions/:id` | 删除并**重放修正持仓** |

### 3.3 持仓与总览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/positions` | 实时持仓（含市值/浮盈，`PositionView` 动态字段） |
| PUT | `/positions/valuation` | 手工估值（存款/理财） |
| POST | `/positions/target` | 设置追踪目标价/止损参考价（重放不覆盖） |
| GET | `/overview` | 全局与分账户资产总览 |
| GET | `/advice` | 买卖建议（规则引擎 + MA20/60） |
| GET | `/price-history` | 价格/净值历史 |

---

## 4. 分析报表

全部为 GET，均接受时间区间等 query 参数（`AnalyticsReq`）：

| 路径 | 说明 |
|------|------|
| `/analytics/equity-curve` | 收益曲线（可叠加基准指数归一化对比） |
| `/analytics/returns` | 简单收益 + Modified Dietz + XIRR |
| `/analytics/trades-review` | 加仓减持复盘 |
| `/analytics/risk` | 回撤 / 年化波动率 / 集中度 / HHI |
| `/analytics/dca-simulate` | 定投 vs 一次性模拟 |
| `/analytics/fees` | 费用分析与侵蚀度 |
| `/analytics/dividends` | 分红统计与 YOC |
| `/analytics/index-analysis` | 指数均线/分位/年化分析 |

---

## 5. 计划、汇率、配比与费用

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/fees` | 独立费用查询 / 新增 |
| DELETE | `/fees/:id` | 删除费用 |
| GET/POST | `/dca-plans` | 定投计划列表 / 创建 |
| GET/PUT/DELETE | `/dca-plans/:id` | 定投计划详情 / 更新 / 删除 |
| GET/PUT | `/fx-rates` | 汇率查询 / 录入 |
| GET/POST | `/allocations` | 资产配比目标列表 / 创建 |
| PUT/DELETE | `/allocations/:id` | 配比更新 / 删除 |
| GET | `/allocations/review` | 实际 vs 目标对比（偏差预警） |

---

## 6. 价格告警

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/alerts` | 告警规则列表 / 创建 |
| GET/PUT/DELETE | `/alerts/:id` | 告警详情 / 更新 / 删除 |

---

## 7. 定时任务手动触发

均为 POST，**幂等**（重复触发不产生重复数据）：

| 路径 | 说明 |
|------|------|
| `/quotes/refresh` | 手动刷新行情 |
| `/tasks/accrue` | 手动计息 |
| `/tasks/snapshot` | 手动快照（幂等覆盖） |
| `/tasks/maturity-check` | 到期检测 |
| `/tasks/check-alerts` | 手动告警检查 |
| `/tasks/check-risk` | 手动风险巡检 |
| `/tasks/refresh-fx` | 手动刷新汇率 |

---

## 8. 导入与筛选

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/import/transactions` | 券商对账单 CSV 导入（含 `TxTupleExists` 查重） |
| GET | `/screen/dynamic-potential` | 动态潜力股筛选（全市场快照，TTL 10min） |
| GET | `/screen/quality-stocks` | 绩优股筛选 |

---

## 9. 业务错误码（`42xxx`）

| 段 | 码位 | 示例 |
|----|------|------|
| 资源不存在 | 42001–42099 | 42001 账户不存在、42002 标的不存在、42003 交易不存在、42009 汇率不存在 |
| 参数校验 | 42100–42199 | 42101 账户类型无效、42102 成本法无效、42108 比例无效、42113 通用参数错误 |
| 业务冲突 | 42200–42299 | 42201 标的重复、42202 账户尚有持仓、42203 份额不足、42206 配比总和超限 |
| 任务/外部依赖 | 42300–42399 | 42301 行情不可用、42302 任务失败、42303 熔断打开、42304 数据不足 |
| 告警/导入 | 42400–42499 | 42401 告警不存在、42403 导入格式错误、42404 导入未指定账户 |
