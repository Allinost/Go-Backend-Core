# Webhook 数据模型

## 1. 数据库

- **数据库**: 使用当前开发库 `go_backend_core`
- **表名格式**: `zzz_{模块名}_{表名}`
- **字符集**: `utf8mb4` / `utf8mb4_unicode_ci`
- **引擎**: InnoDB

---

## 2. 表结构

### 2.1 `zzz_webhook_configs` — Webhook 配置

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(100) | NOT NULL | Webhook 名称（如"主力群通知"） |
| type | VARCHAR(20) | NOT NULL | 类型: `feishu` / `dingtalk` / `wecom` / `github` / `generic` |
| url | VARCHAR(2048) | NOT NULL | Webhook 地址 |
| status | VARCHAR(10) | NOT NULL DEFAULT 'active' | `active` / `paused` / `disabled` |
| tags | JSON | NULL | 标签数组（用于分组选择，如 `["shopchecker", "price"]`） |
| description | VARCHAR(500) | DEFAULT '' | 备注说明 |
| source | VARCHAR(50) | NOT NULL DEFAULT 'system' | 来源标识（如 `system` / `shopchecker` / `cicd`） |
| app | VARCHAR(50) | DEFAULT '' | 应用/场景标识（如 `price-monitor` / `deploy-notify`） |
| signing_key | VARCHAR(255) | DEFAULT '' | 签名密钥（飞书签名/钉钉加签/GitHub Secret） |
| access_token | VARCHAR(255) | DEFAULT '' | 访问令牌（钉钉 AccessToken 等） |
| auth_type | VARCHAR(20) | DEFAULT '' | 通用鉴权类型: `basic` / `bearer` / `header` |
| auth_value | TEXT | DEFAULT '' | 鉴权值（Basic base64 / Bearer token 等） |
| custom_headers | JSON | NULL | 自定义 HTTP Headers（`{"X-Custom": "value"}`） |
| whitelist_keywords | JSON | NULL | 白名单关键词数组 `["降价", "缺货"]` |
| whitelist_mode | VARCHAR(10) | DEFAULT 'any' | `any`=命中任一 / `all`=全部命中 |
| max_per_minute | INT | DEFAULT 60 | 每分钟最大发送次数 |
| max_per_hour | INT | DEFAULT 1000 | 每小时最大发送次数 |
| dedup_window | INT | DEFAULT 0 | 去重窗口秒数（0=不启用） |
| max_retries | INT | DEFAULT 3 | 最大重试次数 |
| retry_interval | INT | DEFAULT 30 | 重试间隔秒数 |
| timeout_ms | INT | DEFAULT 10000 | HTTP 请求超时毫秒数 |
| last_sent_at | DATETIME | NULL | 最近发送时间 |
| last_success_at | DATETIME | NULL | 最近成功时间 |
| last_error | VARCHAR(500) | DEFAULT '' | 最近一次错误信息 |
| total_sent | INT | DEFAULT 0 | 累计发送次数 |
| total_failed | INT | DEFAULT 0 | 累计失败次数 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

索引：
- `idx_status` (status)
- `idx_type` (type)
- `idx_source_app` (source, app)
- `idx_tags` (tags) — 使用虚拟列或 JSON_MEMBER

### 2.2 `zzz_webhook_delivery_logs` — 投递日志

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | BIGINT | PK AUTO_INCREMENT | 自增 ID |
| webhook_id | VARCHAR(36) | NOT NULL, INDEX | 关联 Webhook 配置 ID |
| source | VARCHAR(50) | NOT NULL | 来源模块标识（如 `shopchecker`） |
| title | VARCHAR(255) | DEFAULT '' | 消息标题 |
| content_hash | VARCHAR(64) | NOT NULL | 消息内容 SHA256（用于去重查询） |
| status | VARCHAR(20) | NOT NULL DEFAULT 'pending' | `pending` / `success` / `failed` / `rate_limited` / `deduped` |
| request_body | TEXT | NULL | 发送的请求体 |
| response_code | INT | DEFAULT 0 | HTTP 响应状态码 |
| response_body | TEXT | NULL | 响应体（截断） |
| error_msg | VARCHAR(500) | DEFAULT '' | 错误信息 |
| retry_count | INT | DEFAULT 0 | 已重试次数 |
| duration_ms | INT | DEFAULT 0 | 请求耗时 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

索引：
- `idx_webhook_status` (webhook_id, status)
- `idx_content_hash` (content_hash, created_at) — 去重查询
- `idx_created` (created_at) — 日志清理

### 2.3 `zzz_webhook_rate_limits` — 频率限制状态（可选持久化）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | BIGINT | PK AUTO_INCREMENT | 自增 ID |
| webhook_id | VARCHAR(36) | NOT NULL | 关联 Webhook |
| window_start | DATETIME | NOT NULL | 窗口起始时间 |
| window_type | VARCHAR(10) | NOT NULL | `minute` / `hour` |
| count | INT | NOT NULL DEFAULT 0 | 当前窗口计数 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | |

唯一索引：`idx_webhook_window` (webhook_id, window_type, window_start)

> 频率限制主要基于 Redis 实现，此表作为持久化兜底和统计查询使用。

---

## 3. Go 数据模型

```go
package webhook

import (
    "encoding/json"
    "time"
)

// WebhookType 支持的 Webhook 类型
type WebhookType string
const (
    WebhookTypeFeishu   WebhookType = "feishu"
    WebhookTypeDingtalk WebhookType = "dingtalk"
    WebhookTypeWecom    WebhookType = "wecom"
    WebhookTypeGitHub   WebhookType = "github"
    WebhookTypeGeneric  WebhookType = "generic"
)

// WhitelistMode 白名单匹配模式
type WhitelistMode string
const (
    WhitelistModeAny WhitelistMode = "any" // 命中任一关键词即投递
    WhitelistModeAll WhitelistMode = "all" // 全部关键词命中才投递
)

// ConfigStatus Webhook 状态
type ConfigStatus string
const (
    ConfigStatusActive   ConfigStatus = "active"
    ConfigStatusPaused   ConfigStatus = "paused"
    ConfigStatusDisabled ConfigStatus = "disabled"
)

// DeliveryStatus 投递状态
type DeliveryStatus string
const (
    DeliveryPending     DeliveryStatus = "pending"
    DeliverySuccess     DeliveryStatus = "success"
    DeliveryFailed      DeliveryStatus = "failed"
    DeliveryRateLimited DeliveryStatus = "rate_limited"
    DeliveryDeduped     DeliveryStatus = "deduped"
)

// WebhookConfig Webhook 配置
type WebhookConfig struct {
    ID               string            `json:"_id" db:"id"`
    Name             string            `json:"name" db:"name"`
    Type             string            `json:"type" db:"type"`
    URL              string            `json:"url" db:"url"`
    Status           string            `json:"status" db:"status"`
    Tags             []string          `json:"tags,omitempty" db:"tags"`
    Description      string            `json:"description,omitempty" db:"description"`
    Source           string            `json:"source" db:"source"`
    App              string            `json:"app,omitempty" db:"app"`

    // 鉴权信息（敏感字段，查询时默认脱敏）
    SigningKey       string            `json:"signing_key,omitempty" db:"signing_key"`
    AccessToken      string            `json:"access_token,omitempty" db:"access_token"`
    AuthType         string            `json:"auth_type,omitempty" db:"auth_type"`
    AuthValue        string            `json:"auth_value,omitempty" db:"auth_value"`
    CustomHeaders    map[string]string `json:"custom_headers,omitempty" db:"custom_headers"`

    // 白名单
    WhitelistKeywords []string         `json:"whitelist_keywords,omitempty" db:"whitelist_keywords"`
    WhitelistMode    string            `json:"whitelist_mode,omitempty" db:"whitelist_mode"`

    // 频率控制
    MaxPerMinute     int               `json:"max_per_minute" db:"max_per_minute"`
    MaxPerHour       int               `json:"max_per_hour" db:"max_per_hour"`
    DedupWindow      int               `json:"dedup_window" db:"dedup_window"`

    // 重试
    MaxRetries       int               `json:"max_retries" db:"max_retries"`
    RetryInterval    int               `json:"retry_interval" db:"retry_interval"`
    TimeoutMS        int               `json:"timeout_ms" db:"timeout_ms"`

    // 统计
    LastSentAt       *time.Time        `json:"last_sent_at,omitempty" db:"last_sent_at"`
    LastSuccessAt    *time.Time        `json:"last_success_at,omitempty" db:"last_success_at"`
    LastError        string            `json:"last_error,omitempty" db:"last_error"`
    TotalSent        int               `json:"total_sent" db:"total_sent"`
    TotalFailed      int               `json:"total_failed" db:"total_failed"`

    CreatedAt        time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}

// WebhookConfigMasked 对外展示的脱敏配置
type WebhookConfigMasked struct {
    WebhookConfig
    SigningKey  string `json:"signing_key"`  // "sk-***abc"
    AccessToken string `json:"access_token"` // "at-***xyz"
    AuthValue   string `json:"auth_value"`   // "***"
}

// DeliveryLog 投递日志
type DeliveryLog struct {
    ID            int64     `json:"id" db:"id"`
    WebhookID     string    `json:"webhook_id" db:"webhook_id"`
    Source        string    `json:"source" db:"source"`
    Title         string    `json:"title,omitempty" db:"title"`
    ContentHash   string    `json:"content_hash" db:"content_hash"`
    Status        string    `json:"status" db:"status"`
    RequestBody   *string   `json:"request_body,omitempty" db:"request_body"`
    ResponseCode  int       `json:"response_code" db:"response_code"`
    ResponseBody  *string   `json:"response_body,omitempty" db:"response_body"`
    ErrorMsg      string    `json:"error_msg,omitempty" db:"error_msg"`
    RetryCount    int       `json:"retry_count" db:"retry_count"`
    DurationMS    int       `json:"duration_ms" db:"duration_ms"`
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// --- DTO ---

// SendReq 发送请求（业务模块调用时传入）
type SendReq struct {
    WebhookIDs []string       `json:"webhook_ids"`            // 指定 Webhook（二选一）
    Tags       []string       `json:"tags"`                   // 按标签选择（二选一）
    Source     string         `json:"source"`                 // 来源模块标识
    Title      string         `json:"title"`                  // 消息标题
    Content    string         `json:"content"`                // Markdown 正文
    Data       map[string]any `json:"data,omitempty"`         // 模板变量
    MsgType    string         `json:"msg_type"`               // text / markdown / card
}

// SendResult 发送结果
type SendResult struct {
    Success int             `json:"success"`
    Failed  int             `json:"failed"`
    Results []WebhookResult `json:"results"`
}

type WebhookResult struct {
    WebhookID  string `json:"webhook_id"`
    WebhookName string `json:"webhook_name"`
    Status     string `json:"status"`    // success / failed / rate_limited / deduped
    Error      string `json:"error,omitempty"`
    DurationMS int    `json:"duration_ms"`
}

// TestReq 测试发送
type TestReq struct {
    WebhookID string `json:"webhook_id"`
    TestMsg   string `json:"test_msg,omitempty"`
}
```