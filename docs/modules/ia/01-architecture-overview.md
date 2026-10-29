# 投资分析模块架构概览

## 1. 模块定位

投资分析模块（`zzz-ia`，代码目录 `zzz/ia`）是一个**个人投资组合管理与分析模块**，覆盖存款、银行理财、基金（含 QDII）、个股、指数 ETF 等多资产类型，提供从记账、持仓核算到行情接入、分析报表、买卖建议、告警与筛选的完整链路。

### 1.1 解决的问题

| 问题 | 方案 |
|------|------|
| 多类资产分散在各处，无法统一查看 | 账户 + 标的 + 流水 + 交易统一建模，`/overview` 一屏总览 |
| 持仓成本与收益口径不一致 | 交易全量重放引擎，WA/FIFO 双法核算，已实现/未实现收益分账记录 |
| 手工记账易错、易漏 | 券商对账单 CSV 导入（含查重），定时自动计息/快照/到期检测 |
| 缺少收益归因与风险视角 | 收益曲线、Dietz/XIRR、回撤/波动率/集中度、复盘、费用侵蚀度、分红 YOC |
| 买卖决策靠感觉 | MA20/60 技术标志 + 规则引擎建议、价格告警、资产配比偏差预警 |
| 行情源不稳拖垮业务 | Provider 注册表 + 重试/断路器，主行情与筛选独立熔断 |

### 1.2 分层架构

```
HTTP (/api/v1/zzz-ia/*)
        │
        ▼
handler.go (1449 行)  参数绑定 · 统一错误处理 handleError
        │
        ▼
service.go (1340 行)  业务校验 · 交易构建 · 持仓重放引擎
   ├─ tasks.go          定时任务实现（行情/计息/快照/到期/告警）
   ├─ analytics.go      分析报表编排 ── analytics_math.go 纯函数金融数学
   ├─ advice.go         买卖建议规则引擎
   ├─ alerts_import.go  价格告警 + CSV 导入
   └─ screening.go      筛选 / 指数分析 / 汇率刷新 / 快照缓存
        │
        ▼
repository.go (2100 行)  原生 SQL · 13 张 zzz_ia_* 表
        │
        ▼
MySQL main 连接池 (database.DB.MySQL["main"])

旁路依赖:
  quote/ 子包 ── 东方财富 / 天天基金 / open.er-api (经 pkg/net 熔断 HTTP)
  webhook 模块 ── 到期提醒、告警推送（可选）
  ws Hub ── 房间 "zzz-ia" 实时推送（可选）
  scheduler 模块 ── 集中调度占位注册（可选）
```

### 1.3 命名空间

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | `zzz/ia/`（业务域包） | `zzz/ia/service.go`、`zzz/ia/quote/` |
| 表名 | `zzz_ia_{表名}` | `zzz_ia_accounts`、`zzz_ia_positions` |
| 路由前缀 | `/api/v1/zzz-ia` | `/api/v1/zzz-ia/accounts` |
| 模块名 | `zzz-ia` | `modules.Register(&ia.Module{})`（`module.go:40`） |
| 错误码 | `42xxx` 段 | `42001` 账户不存在 |
| WebSocket 房间 | `zzz-ia` | 实时推送（`service.go:17`） |

---

## 2. 核心能力

### 2.1 账户与标的

- 账户 CRUD：类型（broker/bank/fund/other）、币种、成本法（wa/fifo）、状态
- 标的 CRUD：类型（deposit/wealth/fund/stock/etf）、子类型（qdii 等）、年化利率、起息/到期日、锁定期、风险等级 R1-R5、跟踪指数
- `GET /assets/resolve` 证券代码解析：调东方财富补全名称/行情/PE/PB

### 2.2 交易与持仓核算（核心）

- 流水：入金/出金/划转（`cash-flows`）
- 交易：买入/卖出/分红（现金/再投）/利息/送转/配股，支持**在途状态**（场外 T+1）与手动 confirm
- **持仓重放引擎**（`service.go:677` `replay`）：任意账户的持仓由其全部已确认交易重放推导，删除交易即重放修正；WA/FIFO 双法
- 已实现收益落表 `zzz_ia_realized_pnl`（FIFO 记录批次）
- 手工估值（存款/理财 `PUT /positions/valuation`）、追踪目标价/止损价（重放不覆盖）

### 2.3 行情接入（quote 子包）

| Provider | 上游 | 覆盖 |
|----------|------|------|
| `EastmoneyProvider` | push2.eastmoney.com | A股/ETF/港股/美股实时行情与详情 |
| `EastmoneyScreen` | push2 clist | 全市场 A 股快照分页（筛选用，独立断路器） |
| `FundProvider` | fundgz.1234567.com.cn | 场外基金/QDII 官方净值 + 实时估值（JSONP） |
| `KLine` | push2his | 日 K 线、基准指数（CSI300/NASDAQ100/SP500 等） |
| `Fx` | open.er-api.com | 免费汇率（CNY→USD/HKD），每日 08:10 自动刷新 |

### 2.4 分析报表

- 收益曲线（可叠加基准指数归一化对比）
- 收益指标：简单收益 + Modified Dietz + **XIRR（牛顿法）**
- 交易复盘（加仓/减持）、风险指标（最大回撤/年化波动率/HHI 集中度）
- 定投模拟（DCA vs 一次性）、费用分析与侵蚀度、分红统计与 YOC
- 指数分析：均线/分位/年化（`/analytics/index-analysis`）

### 2.5 筛选与建议

- 动态潜力股、绩优股筛选（全市场快照缓存，TTL 10min，启动预热）
- 买卖建议引擎：纯函数规则 `evaluateAdvice` + MA20/60 技术标志
- 价格告警（止损/止盈/目标价）+ 风险巡检，经 webhook 推送
- 资产配比目标 vs 实际偏差预警（`/allocations/review`）
- 定投计划（daily/weekly/biweekly/monthly）

---

## 3. 定时任务

模块内置 JobScheduler（`scheduler.go`，ticker 驱动，at-most-once）：

| 任务 | 触发时机 | Service 方法 |
|------|----------|--------------|
| 行情刷新 | 周一至五 9:00–16:00 每 30 分钟 | `RefreshQuotes` |
| 自动计息 | 每日 ≥07:30（ACT/365） | `AccrueInterest` |
| 每日快照 | 每日 ≥16:35 | `GenerateSnapshots` |
| 到期检测 | 每日 ≥09:05（webhook 提醒） | `CheckMaturities` |
| 风险巡检 | 每日 ≥16:45 | `CheckRiskAlerts` |
| 汇率刷新 | 每日 08:10 | `RefreshFxRates` |

同时向 centralized scheduler 注册占位任务 `zzz_ia_daily_maintenance`（30m 间隔，仅管理可见性）。每个任务 5 分钟超时 + panic recover，均可通过 `/tasks/*` 手动触发（幂等）。

---

## 4. 依赖关系

```
zzz-ia Module
  ├── internal/database                  # MySQL main 连接池（原生 *sql.DB）
  ├── internal/config                    # 外部化 HTTP/断路器配置（Net.HTTP.*）
  ├── internal/modules                   # 模块注册表（取 webhook 可选依赖）
  ├── internal/modules/webhook           # 到期/告警通知（可选，未注册仅告警）
  ├── internal/services/scheduler        # 集中调度占位注册（可选）
  ├── internal/services/ws               # WebSocket 实时推送（可选，SetWSHub 注入）
  ├── internal/pkg/{logger,response,errors}
  ├── pkg/net                            # HTTPClient（重试 + 熔断）· ErrCircuitOpen
  ├── pkg/pagination                     # 分页
  └── github.com/google/uuid             # 主键生成
```

**仅依赖 MySQL `main` 一个数据库实例**，不依赖 Redis/缓存/事件总线，可独立运行。

---

## 5. 测试覆盖

| 测试文件 | 覆盖 |
|----------|------|
| `analytics_test.go` | XIRR/Dietz/最大回撤/年化波动率等纯函数 |
| `advice_test.go` | `evaluateAdvice` 全规则分支 + 目标价校验 |
| `alerts_import_test.go` | CSV 解析、市场/类型推断、告警命中与文案 |
| `screening_test.go` | 筛选与汇率逻辑 |
| `scheduler_test.go` | 调度器每日一次标记逻辑 |
| `service_test.go` | 服务层核心逻辑 |
| `quote/*_test.go` | 行情解析（ParseEastmoney/ParseFundGz/ParseKline/ParseSnapshot/ParseFx） |

> `module.go`、`handler.go`、`repository.go`、`model.go`、`errors.go`、`tasks.go` 暂无测试。
