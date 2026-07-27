# Webhook 公共模块架构概览

## 1. 模块定位

Webhook 是一个**公共服务模块**，负责统一管理出站 Webhook 的配置、调用、安全认证、频率控制和投递追踪。所有需要发送 Webhook 通知的业务模块（如 ShopChecker 价格监控、系统告警、CI/CD 通知等）都通过本模块发送，不自行管理 Webhook 配置。

### 1.1 为什么设计为公共模块

| 问题 | 方案 |
|------|------|
| 每个业务模块各自管理 Webhook 配置 → 数据冗余、管理分散 | 统一 Webhook 配置中心，一个地方管理所有 Webhook |
| 飞书、钉钉、企微、GitHub 各自不同的鉴权和消息格式 | 模块内部适配，业务模块只需调用统一接口 |
| 多个业务模块可能发送到同一个 Webhook → 需要频率控制 | 模块级别限流 + 去重 |
| 投递失败需要重试和告警 | 统一重试策略 + 投递日志 |

### 1.2 与业务模块的关系

```
                     ┌──────────────────────┐
                     │   业务模块 (Consumer)  │
                     │  ShopChecker / 告警 / …│
                     └─────────┬────────────┘
                               │ 调用 Send()
                               ▼
                     ┌──────────────────────┐
                     │   zzz/webhook 模块     │
                     │  ┌────────────────┐   │
                     │  │ Webhook 配置管理  │   │
                     │  │ 鉴权 / 限流 / 重试 │   │
                     │  │ 投递日志 / 统计   │   │
                     │  └────────────────┘   │
                     └─────────┬────────────┘
                               │ HTTP POST
                               ▼
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
          飞书 Webhook    GitHub Webhook    通用 Webhook
```

### 1.3 命名空间

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | `zzz/webhook/` | `zzz/webhook/service.go` |
| 表名 | `zzz_{模块名}_{表名}` | `zzz_webhook_configs` |
| Redis Key | `zzz:` 前缀 | `zzz:webhook:ratelimit:xxx` |
| 路由前缀 | `/api/v1/webhook` | `/api/v1/webhook/configs` |
| 模块名 | `webhook` | `modules.Register(&webhook.Module{})` |

---

## 2. 支持的消息类型 (Type)

| 类型 | 标识 | 说明 |
|------|------|------|
| **飞书** | `feishu` | 飞书机器人 Webhook，支持文本/富文本/消息卡片 |
| **钉钉** | `dingtalk` | 钉钉机器人 Webhook |
| **企业微信** | `wecom` | 企微机器人 Webhook |
| **GitHub** | `github` | GitHub Webhook (Push/Release/Issue 等事件) |
| **通用** | `generic` | 通用 HTTP POST Webhook，自定义 Headers + Body |

每个类型在发送时自动处理：
- **签名/鉴权**: 飞书签名、钉钉加签、GitHub HMAC 验证
- **消息格式**: 自动构建对应平台的消息体结构
- **内容约束**: 各平台的消息长度、频率限制

---

## 3. 核心能力

### 3.1 配置管理

- Webhook 配置的 CRUD（名称、URL、类型、鉴权信息、白名单、限流等）
- 启用/禁用、标签分组
- 测试发送（向 Webhook 发送一条测试消息）

### 3.2 投递服务

业务模块通过 Go API 直接调用：

```go
// 统一发送接口
func (s *Service) Send(ctx context.Context, req *SendReq) (*SendResult, error)

type SendReq struct {
    WebhookIDs []string      // 指定 Webhook ID 列表（优先于标签）
    Tags       []string      // 按标签选择 Webhook
    Title      string        // 消息标题
    Content    string        // 消息正文（Markdown）
    Data       map[string]any // 额外数据（用于模板）
    Type       string        // 消息类型：text / markdown / card / interactive
}

type SendResult struct {
    Success int            // 成功投递数
    Failed  int            // 失败投递数
    Results []WebhookResult // 每个 Webhook 的投递结果
}
```

### 3.3 鉴权与安全

| 类型 | 鉴权方式 | 配置字段 |
|------|----------|----------|
| 飞书 | 无 (URL 自带 secret) | — |
| 飞书签名 | HMAC-SHA256 签名校验 | `signing_key` |
| 钉钉 | AccessToken + 加签 | `access_token`, `signing_key` |
| 企业微信 | Webhook URL 自带 Key | — |
| GitHub | Secret Token (HMAC-SHA256) | `secret_token` |
| 通用 | 自定义 Header / Basic Auth | `headers` (JSON map), `auth_type`, `auth_value` |

### 3.4 白名单过滤

支持按关键词白名单过滤，只有消息内容包含指定关键词时才投递：

```json
{
  "whitelist_keywords": ["降价", "缺货", "异常"],
  "whitelist_mode": "any"   // any=命中任一即投递, all=全部命中才投递
}
```

### 3.5 频率限制

| 层级 | 限流粒度 | 配置 |
|------|----------|------|
| 全局 | 所有 Webhook 共享 | `global_rps` (每秒请求数) |
| 每 Webhook | 单个 Webhook 限制 | `max_per_minute`, `max_per_hour` |
| 每内容 | 相同内容去重 | `dedup_window` (相同内容在窗口内不重复投递) |

超出限制的消息进入死信队列或在当前窗口静默丢弃。

### 3.6 重试策略

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `max_retries` | 3 | 最大重试次数 |
| `retry_interval` | 30s | 重试间隔（固定或指数退避） |
| `retry_backoff` | `exponential` | `fixed` / `exponential` |
| `timeout` | 10s | 单次 HTTP 请求超时 |

---

## 4. 代码目录结构

```
zzz/webhook/
├── module.go                    # Module 接口实现
├── handler.go                   # HTTP 处理器（Webhook 配置管理 API）
├── service.go                   # 核心业务逻辑（配置管理 + 投递服务）
├── repository.go                # 数据访问层
├── model.go                     # 数据模型 / DTO
├── errors.go                    # 模块级错误码
│
├── sender/                      # 消息发送器（按类型适配）
│   ├── sender.go                # Sender 接口 + 工厂
│   ├── feishu.go                # 飞书消息构建 + 发送
│   ├── dingtalk.go              # 钉钉消息构建 + 发送
│   ├── wecom.go                 # 企业微信消息构建 + 发送
│   ├── github.go                # GitHub Webhook 事件发送
│   └── generic.go               # 通用 HTTP Webhook
│
├── limiter/                     # 频率限制
│   ├── limiter.go               # 限流接口 + 工厂
│   └── redis_limiter.go         # 基于 Redis 的滑动窗口限流
│
└── dedup/                       # 去重
    └── dedup.go                 # 基于内容哈希的去重
```

---

## 5. 业务模块集成方式

### 5.1 Go 接口调用（推荐）

```go
// 在 ShopChecker 的 Init() 中获取 webhook 模块
import "github.com/Allinost/go-backend-core/internal/modules"

func (m *ShopCheckerModule) Init(cfg *config.Config) error {
    // 通过模块注册表获取 webhook 模块
    whModule := modules.Get("webhook").(*webhook.Module)
    whSvc := whModule.GetService()

    // 构建通知器时传入 whSvc
    m.notifier = notifier.New(whSvc)
    // ...
}
```

### 5.2 通过 EventBus 异步触发（松耦合）

```go
// ShopChecker 发布事件
bus.Publish(ctx, "price.dropped", eventbus.Event{
    Source: "shopchecker",
    Payload: map[string]any{
        "webhook_ids": []string{"wh-xxx", "wh-yyy"},
        "title": "降价提醒",
        "content": "...",
    },
})

// Webhook 模块订阅事件
bus.Subscribe("price.dropped", func(ctx context.Context, e eventbus.Event) {
    whSvc.Send(ctx, parseSendReq(e.Payload))
})
```

### 5.3 HTTP API 调用

```go
// 通过内部 HTTP 调用（适用于跨进程/跨服务场景）
POST /api/v1/webhook/send
{
    "webhook_ids": ["wh-xxx"],
    "title": "降价提醒",
    "content": "商品A 从 ¥100 降至 ¥80"
}
```

---

## 6. 依赖关系

```
Webhook Module
  ├── internal/database           # MySQL Pool
  ├── internal/services/cache     # Redis（限流/去重）
  ├── internal/pkg/response       # 统一响应格式
  ├── internal/pkg/errors         # 错误码体系
  ├── github.com/robfig/cron/v3   # 定时重试任务
  └── net/http                    # HTTP 调用
```