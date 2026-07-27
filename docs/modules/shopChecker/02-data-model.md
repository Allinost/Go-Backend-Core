# ShopChecker 数据模型

## 1. 数据库

- **数据库**: 使用当前开发库 `go_backend_core`，不新建库
- **表名格式**: `zzz_{模块名}_{表名}`
- **字符集**: `utf8mb4` / `utf8mb4_unicode_ci`
- **引擎**: InnoDB

---

## 2. 表结构

### 2.1 `zzz_shopchecker_products` — 监控商品

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(255) | NOT NULL | 商品名称（自定义） |
| platform | VARCHAR(20) | NOT NULL | 平台：`jd` / `taobao` / `pdd` |
| url | VARCHAR(2048) | NOT NULL | 商品页 URL |
| image_url | VARCHAR(1024) | DEFAULT '' | 商品图片 URL |
| target_price | DECIMAL(12,2) | DEFAULT 0 | 目标价（低于此价通知），0 表示任何降价都通知 |
| price_drop_ratio | DECIMAL(5,2) | DEFAULT 0 | 降价比例阈值（百分比），0 表示任何降价都通知 |
| last_price | DECIMAL(12,2) | DEFAULT 0 | 最近一次抓取的价格 |
| last_checked_at | DATETIME | NULL | 最近一次检查时间 |
| next_check_at | DATETIME | NULL | 下次检查时间 |
| check_interval | VARCHAR(20) | NOT NULL DEFAULT '1h' | 检查周期（Go duration 格式：30m/1h/6h/1d） |
| cron_expr | VARCHAR(100) | DEFAULT '' | 自定义 cron 表达式（优先级高于 check_interval） |
| custom_webhook_ids | JSON | NULL | 商品专属 Webhook ID 数组，覆盖渠道级；为空时使用渠道级 |
| enabled | TINYINT(1) | NOT NULL DEFAULT 1 | 是否启用监控 |
| enabled | TINYINT(1) | NOT NULL DEFAULT 1 | 是否启用监控 |
| remark | TEXT | NULL | 备注 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

索引：
- `idx_enabled_next_check` (enabled, next_check_at) — 调度器查询待检查商品
- `idx_platform` (platform)

### 2.2 `zzz_shopchecker_price_records` — 价格历史记录

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | BIGINT | PK AUTO_INCREMENT | 自增 ID |
| product_id | VARCHAR(36) | NOT NULL, INDEX | 关联商品 ID |
| price | DECIMAL(12,2) | NOT NULL | 本次抓取价格 |
| original_price | DECIMAL(12,2) | DEFAULT 0 | 划线价/原价 |
| title | VARCHAR(500) | DEFAULT '' | 抓取时的商品标题 |
| is_notified | TINYINT(1) | DEFAULT 0 | 是否已发送降价通知 |
| raw_data | JSON | NULL | 原始抓取数据（备用） |
| error_msg | VARCHAR(500) | DEFAULT '' | 抓取错误信息（失败时记录） |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 记录时间 |

索引：
- `idx_product_created` (product_id, created_at) — 历史价格查询

### 2.3 `zzz_shopchecker_notification_logs` — 降价通知记录

ShopChecker 仅记录**降价事件的业务上下文**，实际的 Webhook 投递日志由 `zzz/webhook` 模块管理。

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | BIGINT | PK AUTO_INCREMENT | 自增 ID |
| product_id | VARCHAR(36) | NOT NULL, INDEX | 关联商品 ID |
| price_record_id | BIGINT | NOT NULL | 关联价格记录 ID |
| webhook_ids | JSON | NOT NULL | 发送的目标 Webhook ID 列表 |
| old_price | DECIMAL(12,2) | DEFAULT 0 | 原始价格 |
| new_price | DECIMAL(12,2) | DEFAULT 0 | 新价格 |
| result_summary | VARCHAR(50) | DEFAULT '' | 投递结果概要：`all_success` / `partial` / `all_failed` |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 发送时间 |

---

## 3. Go 数据模型

```go
// Platform 电商平台
type Platform string
const (
    PlatformJD     Platform = "jd"
    PlatformTaobao Platform = "taobao"
    PlatformPDD    Platform = "pdd"
)

// NotificationStatus 通知状态
type NotificationStatus string
const (
    NotifPending NotificationStatus = "pending"
    NotifSuccess NotificationStatus = "success"
    NotifFailed  NotificationStatus = "failed"
)

// WebhookItem Webhook 条目
type WebhookItem struct {
    URL  string `json:"url"`
    Name string `json:"name"`
}

// Product 监控商品
type Product struct {
    ID               string         `json:"_id" db:"id"`
    Name             string         `json:"name" db:"name"`
    Platform         string         `json:"platform" db:"platform"`
    URL              string         `json:"url" db:"url"`
    ImageURL         string         `json:"image_url,omitempty" db:"image_url"`
    TargetPrice      float64        `json:"target_price,omitempty" db:"target_price"`
    PriceDropRatio   float64        `json:"price_drop_ratio,omitempty" db:"price_drop_ratio"`
    LastPrice        float64        `json:"last_price" db:"last_price"`
    LastCheckedAt    *time.Time     `json:"last_checked_at,omitempty" db:"last_checked_at"`
    NextCheckAt      *time.Time     `json:"next_check_at,omitempty" db:"next_check_at"`
    CheckInterval    string         `json:"check_interval" db:"check_interval"`
    CronExpr         string         `json:"cron_expr,omitempty" db:"cron_expr"`
    CustomWebhookIDs []string       `json:"custom_webhook_ids,omitempty" db:"custom_webhook_ids"`
    Enabled          bool           `json:"enabled" db:"enabled"`
    Remark           *string        `json:"remark,omitempty" db:"remark"`
    CreatedAt        time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

// PriceRecord 价格历史记录
type PriceRecord struct {
    ID            int64           `json:"id" db:"id"`
    ProductID     string          `json:"product_id" db:"product_id"`
    Price         float64         `json:"price" db:"price"`
    OriginalPrice float64         `json:"original_price,omitempty" db:"original_price"`
    Title         string          `json:"title,omitempty" db:"title"`
    IsNotified    bool            `json:"is_notified" db:"is_notified"`
    RawData       json.RawMessage `json:"raw_data,omitempty" db:"raw_data"`
    ErrorMsg      string          `json:"error_msg,omitempty" db:"error_msg"`
    CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// NotificationLog 降价通知记录（仅记录业务上下文，投递详情见 webhook 模块）
type NotificationLog struct {
    ID            int64     `json:"id" db:"id"`
    ProductID     string    `json:"product_id" db:"product_id"`
    PriceRecordID int64     `json:"price_record_id" db:"price_record_id"`
    WebhookIDs    []string  `json:"webhook_ids" db:"webhook_ids"`
    OldPrice      float64   `json:"old_price" db:"old_price"`
    NewPrice      float64   `json:"new_price" db:"new_price"`
    ResultSummary string    `json:"result_summary" db:"result_summary"` // all_success / partial / all_failed
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
```