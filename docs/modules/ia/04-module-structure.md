# 投资分析模块结构

## 1. 文件组织

代码目录：`zzz/ia/`（19 个主包文件 + 7 个 quote 子包文件）

### 1.1 主包 `zzz/ia/`

| 文件 | 行数 | 职责 | 测试 |
|------|------|------|------|
| `module.go` | 580 | 模块入口：Name/Init/Close/RegisterRoutes、**自动建表 DDL**、HTTP/断路器配置 | ❌ |
| `handler.go` | 1449 | HTTP 处理器：参数绑定、调 Service、统一 `handleError` | ❌ |
| `service.go` | 1340 | 业务核心：校验、交易构建、**持仓重放引擎**（`replay`，677 行起） | ✅ |
| `repository.go` | 2100 | 数据访问（原生 SQL），13 张表常量与全部查询 | ❌ |
| `model.go` | 1256 | 实体、枚举、请求/响应 DTO（`db:`/`json:` tag） | ❌ |
| `errors.go` | 130 | `42xxx` 错误码、`IAError` 类型 | ❌ |
| `tasks.go` | 328 | 定时任务实现：行情刷新、计息、快照、到期检测 | ❌ |
| `scheduler.go` | 175 | 内部 JobScheduler（ticker + 每日一次标记） | ✅ |
| `analytics.go` | 725 | 分析报表编排 | ✅（math 层） |
| `analytics_math.go` | 370 | 纯函数金融数学：XIRR、Dietz、回撤、波动率、HHI、DCA | ✅ |
| `advice.go` | 280 | 买卖建议引擎：`evaluateAdvice` 规则 + `TradeAdvice` 编排 | ✅ |
| `screening.go` | 588 | 筛选与多币种层：潜力股/绩优股/指数分析/汇率/快照缓存 | ✅ |
| `alerts_import.go` | 560 | 价格告警 CRUD/检查/通知 + 券商对账单 CSV 导入 | ✅ |

### 1.2 子包 `zzz/ia/quote/`

| 文件 | 职责 | 测试 |
|------|------|------|
| `quote.go` | `Provider` 接口 + `Registry`（**主/筛选双 HTTP 客户端、独立断路器**） | ✅ |
| `eastmoney.go` | 东方财富 push2 实时行情（A股/ETF/港/美）与单股详情 | ✅ |
| `eastmoney_screen.go` | clist 全市场 A 股快照分页拉取（含单股 fallback） | ✅ |
| `fund.go` | 天天基金 fundgz：场外基金官方净值 + 实时估值（JSONP） | ✅（在 quote_test） |
| `kline.go` | 东方财富日 K 线、基准指数 secid 映射、序列归一化 | ✅ |
| `fx.go` | 免费汇率 open.er-api.com：CNY 基准拉取 USD/HKD | ✅ |

---

## 2. 分层实现

### 2.1 Handler → Service → Repository

```
handler.go                    service.go                   repository.go
  ListAccounts      ────────►  ListAccounts      ─────────►  ListAccounts
  CreateTransaction ─────────► CreateTransaction ──────────► CreateTransaction
                              buildTx (校验/构建)
                              replay (持仓重放)   ─────────► UpsertPosition
                                                          ReplaceRealizedPnL
  AnalyticsReturns  ─────────► ReturnsAnalytics   ─────────► 聚合查询（Snapshot/Income/Realized）
  TriggerSnapshot   ─────────► GenerateSnapshots  ─────────► SnapshotUpsert
```

核心方法索引：

- **service.go**：账户 CRUD(48-127)、标的与代码解析(130-331)、流水(353-406)、交易与重放（`CreateTransaction`:424、`buildTx`:463、`ConfirmTransaction`:579、`DeleteTransaction`:597、`ReplayAccountAll`:649、**`replay`:677**）、持仓总览(843-933)、定投(1008-1128)、汇率(1156)、配比(1188-1300)
- **tasks.go**：`RefreshQuotes`:34、`AccrueInterest`:107、`GenerateSnapshots`:176、`CheckMaturities`:299
- **analytics.go**：`EquityCurve`:19、`ReturnsAnalytics`:67、`TradesReview`:220、`RiskMetrics`:385、`DCASimulate`:468、`FeesReport`:532、`DividendReport`:618
- **alerts_import.go**：告警 CRUD(23-101)、`CheckPriceAlerts`:103、`CheckRiskAlerts`:165、`ImportTransactions`:416
- **screening.go**：`DynamicPotential`:167、`QualityStockScreen`:301、`IndexAnalysis`:398、`RefreshFxRates`:529、`warmScreenCache`:589

### 2.2 注入点（可选依赖解耦）

| 方法 | 注入方 | 作用 |
|------|--------|------|
| `AttachQuoteRegistry` | module.go:57 | 注入行情源注册表（tasks/筛选使用） |
| `AttachWebhook` | module.go:62 | 注入 webhook 服务（到期/告警通知） |
| `SetNotifier` | module.go:232 `SetWSHub` | WebSocket 房间 `zzz-ia` 实时推送 |
| `GetService()` | module.go:125 | 暴露 Service 给其他模块 |

---

## 3. 模块生命周期（`module.go`）

```go
func (m *Module) Name() string { return "zzz-ia" }   // module.go:40

Init(cfg):                                            // module.go:45-83
  1. 取 MySQL main 连接池（nil 则报错）→ NewRepository（原生 SQL）
  2. NewService(repo)
  3. 构建 quote.Registry（pkg/net 客户端 + 东财 UA/Referer + 断路器配置）
  4. 可选: modules.Get("webhook") → AttachWebhook（未注册仅告警）
  5. initTables(pool.DB)          // 13 张 zzz_ia_* 表，幂等
     migratePositionTargetCols    // 存量列兼容（忽略 Duplicate column）
  6. NewJobScheduler(svc).Start() // 内部 ticker 调度器
  7. registerSchedulerTask()      // 集中调度占位 zzz_ia_daily_maintenance
  8. NewHandler(svc)
  9. go warmScreenCache(...)      // 异步预热全市场快照缓存

Close():  m.sch.Stop()            // module.go:86-91，停 ticker 并 close stopCh
```

**注册位置**：`cmd/server/main.go:106` `modules.Register(&ia.Module{})`。

---

## 4. 定时任务实现（`scheduler.go`）

- 常量定义 `scheduler.go:14-23`，主循环 `tick()`（78-122）
- **at-most-once**：内存 `dailyDone` map 标记当日已执行；失败次日重试或手动 `/tasks/*` 触发
- 每任务 **5 分钟超时** + panic recover（`scheduler.go:152-170`）
- 任务清单见 [01-architecture-overview.md 第 3 节](01-architecture-overview.md#3-定时任务)
- 集中调度注册 `registerSchedulerTask()`（`module.go:94-122`）：任务 `zzz_ia_daily_maintenance`，interval 30m，**仅管理面可见，实际不驱动执行**

---

## 5. quote 子包设计

```go
// quote.go:42
type Provider interface {
    Name() string
    Support(a AssetInfo) bool
    Fetch(ctx context.Context, a AssetInfo) (*Quote, error)
}
```

- **Registry 双客户端**（`quote.go:54-94`）：
  - 主客户端：行情/K线/净值/汇率
  - 筛选客户端：clist 全市场快照，**独立断路器**（失败阈值 10、恢复阈值 2、60s 超时，可经 `Net.HTTP.ScreenBreaker` 配置）
- `Fetch` 按注册顺序匹配 Provider，**失败不跨源兜底**
- K 线可经 `KlineWithClient` 走筛选客户端，绕过主断路器
- 基准指数 secid 映射（`kline.go:20`）：CSI300/CSI500/CSI1000/SSE/SZSE/GEM/STAR50/NASDAQ100/NASDAQ/SP500

**外部上游**：

| 上游 | 端点 | 用途 |
|------|------|------|
| 东方财富 push2 | `push2.eastmoney.com/api/qt/stock/get` | 实时行情与详情 |
| 东方财富 push2 clist | `.../api/qt/clist/get` | 全市场快照（筛选） |
| 东方财富 push2his | `push2his.eastmoney.com/api/qt/stock/kline/get` | 日 K 线 |
| 天天基金 fundgz | `fundgz.1234567.com.cn/js/{code}.js` | 净值 + 估值（JSONP） |
| open.er-api.com | `open.er-api.com/v6/latest/CNY` | 汇率 |

---

## 6. 错误码定义（`errors.go`，`42xxx` 段）

- 类型：`IAError{Code, Message}`（`errors.go:104`）、`NewIAError(code, args...)`（:119，支持格式串覆盖默认文案）
- 分段：42001-42099 资源不存在 / 42100-42199 参数校验 / 42200-42299 业务冲突 / 42300-42399 任务与外部依赖 / 42400-42499 告警与导入
- 映射：`errMsgs`（:68-101）默认文案表 + `extraMsgs`（:56-65）动态文案
- Handler 统一转换：`handler.go:1431-1449`（`IAError` → 业务码；`sql.ErrNoRows` → 404；`pkgNet.ErrCircuitOpen` → 42303；其余 → 500）

---

## 7. 已知限制

1. `module.go`、`handler.go`、`repository.go`、`model.go`、`errors.go`、`tasks.go` 无单元测试
2. `fees` 的增删查在 handler 直调 `svc.repo`，未经过 service 层（与其它域不一致）
3. 建表 DDL 内联在 `module.go`（约 245 行），未纳入 `migrations/` 版本化管理
4. 每日任务标记依赖内存 `dailyDone`，多实例部署会重复执行（当前单实例可接受）
5. `getHost()` 类硬编码外部地址（webhook 回调链接依赖部署环境）
