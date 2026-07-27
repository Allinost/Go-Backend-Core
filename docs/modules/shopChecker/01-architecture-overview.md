# ShopChecker 模块架构概览

## 1. 项目背景

ShopChecker 是一个**电商商品价格监控系统**，定期抓取京东、淘宝、拼多多等平台指定商品的价格，当价格低于预设阈值时，通过飞书 Webhook 发送降价通知。

### 1.1 关于"主动监控"的说明

京东、淘宝、拼多多等电商平台**不对外提供实时价格变更的 Webhook/Push 接口**，因此无法实现真正的"主动推送"式监控。唯一可行的方案是**定时轮询（Polling）**：按配置的检查周期，定时抓取商品页面解析最新价格，与历史价格对比后决定是否发送通知。

### 1.2 命名空间隔离策略

沿用项目统一规范：

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | 存储至项目根 `zzz/` 目录下 | `zzz/shopchecker/` |
| 表名 | `zzz_{模块名}_{表名}` | `zzz_shopchecker_products` |
| Redis Key | 添加 `zzz:` 前缀 | `zzz:shopchecker:cache:...` |
| config 配置段 | 添加 `zzz_` 前缀 | `zzz_shopchecker.xxx` |

使用当前开发库 `go_backend_core`，不新建数据库。

---

## 2. 代码目录结构

```
go-backend-core/
├── zzz/
│   └── shopchecker/                  # ShopChecker 业务模块
│       ├── module.go                 # Module 接口实现（Name="shopchecker"）
│       ├── handler.go                # HTTP 处理器
│       ├── service.go                # 业务逻辑层
│       ├── repository.go             # 数据访问层
│       ├── model.go                  # 数据模型 / DTO
│       ├── errors.go                 # 模块级错误码
│       ├── collector/                # 价格采集器
│       │   ├── collector.go          # 采集器接口定义
│       │   ├── jd.go                 # 京东价格采集
│       │   ├── taobao.go             # 淘宝价格采集
│       │   └── pdd.go               # 拼多多价格采集
│       └── scheduler/                # 调度器
│           └── scheduler.go          # 定时任务调度 + 结果处理
├── docs/modules/shopChecker/         # 设计文档
├── migrations/mysql/
│   └── 003_zzz_shopchecker_init.up.sql
└── config.yaml                       # 新增 zzz_shopchecker 配置段
```

---

## 3. 依赖关系

```
ShopChecker Module
  ├── internal/database           # DBManager 获取 MySQL Pool
  ├── internal/services/scheduler # 框架定时任务调度器 (robfig/cron)
  ├── internal/pkg/response       # 统一响应格式
  ├── internal/pkg/errors         # 错误码体系
  ├── zzz/webhook                 # Webhook 公共模块（通知发送）
  ├── github.com/robfig/cron/v3   # 定时表达式解析
  └── net/http                    # HTTP 抓取
```

---

## 4. 核心业务流程

### 4.1 价格检查流程

```
定时触发 (按商品配置的 cron 表达式)
  → scheduler.NextCheck()
  → 遍历所有 active 商品
    → collector.FetchPrice(ctx, product)  // 调用对应平台采集器
    → 解析返回当前价格
    → repository.SavePriceRecord(ctx, productID, price)
    → 与上一次价格对比
      ├─ 价格下降 >= 阈值 → 调用 webhook 模块发送通知
      └─ 价格未达阈值 → 跳过，记录日志
  → 更新商品 next_check_at
```

### 4.2 Webhook 优先级规则

降价通知通过 **zzz/webhook 公共模块** 发送，不自行管理 Webhook 配置。

```
发送通知时判断使用哪个 Webhook:
  1. 商品是否配置了 custom_webhook_ids (非空)？
     ├─ 是 → 调用 webhook 模块，传入商品自定义的 webhook_id 列表
     └─ 否 → 调用 webhook 模块，按标签 "shopchecker" 匹配渠道级 Webhook
```

Webhook 的创建、编辑、测试、频率控制、投递日志等均在 `zzz/webhook` 模块中管理，见 `docs/modules/webhook/`。

### 4.3 采集器设计

每个平台实现 `Collector` 接口：

```go
type Collector interface {
    Platform() string           // "jd" | "taobao" | "pdd"
    FetchPrice(ctx context.Context, url string) (*PriceResult, error)
}

type PriceResult struct {
    CurrentPrice float64
    OriginalPrice float64       // 原价/划线价
    Title        string         // 商品标题
    Platform     string
    FetchedAt    time.Time
}
```

采集方式：
- **京东**: 解析商品页面 HTML / 调用开放 API
- **淘宝**: 解析商品页面 HTML / 调用开放 API
- **拼多多**: 解析商品页面 HTML / 调用开放 API

> 实际采集可能需要处理反爬、Cookie 登录等问题，初期可先用简单 HTML 解析 + User-Agent 伪装。

---

## 5. 调度策略

- 使用 `robfig/cron/v3` 表达式定义每个商品的检查周期（精确到分钟级别）
- 商品粒度调度：每个商品独立记录 `next_check_at`，调度器每秒轮询一次，到期即执行
- 并发控制：通过 `worker_concurrency` 限制同时抓取的商品数量
- 失败重试：单次抓取失败时，按指数退避策略最多重试 3 次，仍失败则跳过本轮