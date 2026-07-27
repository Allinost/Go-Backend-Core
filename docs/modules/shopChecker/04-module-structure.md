# ShopChecker Go 模块结构

## 1. 模块文件组织

```
zzz/shopchecker/
├── module.go                 # Module 接口实现（Name, Init, Close, RegisterRoutes）
├── handler.go                # HTTP 处理器
├── service.go                # 业务逻辑层
├── repository.go             # 数据访问层（MySQL 查询）
├── model.go                  # 核心数据模型 + DTO + 枚举
├── errors.go                 # 模块级错误码
│
├── collector/                # 价格采集器
│   ├── collector.go          # Collector 接口 + 工厂函数
│   ├── jd.go                 # 京东采集实现
│   ├── taobao.go             # 淘宝采集实现
│   └── pdd.go                # 拼多多采集实现
│
└── scheduler/                # 调度器
    └── scheduler.go          # 定时任务调度 + 结果处理
```

---

### 通知器（notifier/）

ShopChecker 不自建通知模块，**降价通知通过 `zzz/webhook` 公共模块发送**。在 `module.go` 中通过模块注册表获取 webhook 模块的 Service 实例：

```go
package shopchecker

import (
    "github.com/Allinost/go-backend-core/internal/config"
    "github.com/Allinost/go-backend-core/internal/database"
    "github.com/Allinost/go-backend-core/internal/modules"
    "github.com/Allinost/go-backend-core/zzz/webhook"
    "github.com/gin-gonic/gin"
)

type Module struct {
    cfg     *config.Config
    h       *handler.Handler
    svc     *service.Service
    repo    *repository.Repository
    sched   *scheduler.Scheduler
    whSvc   *webhook.Service  // webhook 公共模块服务
}

func (m *Module) Name() string {
    return "shopchecker"
}

func (m *Module) Init(cfg *config.Config) error {
    pool := database.DB.MySQL["main"]
    m.repo = repository.New(pool.DB)
    m.svc = service.New(m.repo)
    m.h = handler.New(m.svc)

    // 通过模块注册表获取 webhook 公共模块
    whModule := modules.Get("webhook").(*webhook.Module)
    m.whSvc = whModule.GetService()

    // 初始化采集器
    collectors := collector.NewRegistry()
    collectors.Register("jd", &collector.JDCollector{})
    collectors.Register("taobao", &collector.TaobaoCollector{})
    collectors.Register("pdd", &collector.PDDCollector{})

    // 启动调度器，传入 whSvc
    m.sched = scheduler.New(m.repo, collectors, m.whSvc)
    m.sched.Start()
    return nil
}

func (m *Module) Close() error {
    m.sched.Stop()
    return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    // 商品管理
    r.GET("/products", m.h.ListProducts)
    r.POST("/products", m.h.CreateProduct)
    r.GET("/products/:id", m.h.GetProduct)
    r.PUT("/products/:id", m.h.UpdateProduct)
    r.DELETE("/products/:id", m.h.DeleteProduct)
    r.POST("/products/:id/check", m.h.ManualCheck)
    r.GET("/products/:id/price-records", m.h.ListPriceRecords)

    // 降价通知记录
    r.GET("/products/:id/notification-logs", m.h.ListNotificationLogs)

    // 仪表盘
    r.GET("/dashboard", m.h.Dashboard)
}
```

---

## 3. 三层架构

### Handler 层（handler.go）

职责：HTTP 请求解析、参数校验、调用 Service、输出统一响应。

```go
type Handler struct {
    svc *service.Service
}

func New(svc *service.Service) *Handler {
    return &Handler{svc: svc}
}

func (h *Handler) CreateProduct(c *gin.Context) {
    var req CreateProductReq
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, appErr.New(appErr.CodeBadRequest, "参数错误: "+err.Error()))
        return
    }
    product, err := h.svc.CreateProduct(c.Request.Context(), &req)
    if err != nil {
        response.Fail(c, err)
        return
    }
    response.Success(c, product)
}
```

### Service 层（service.go）

职责：纯业务逻辑、事务管理、跨模型操作。

```go
type Service struct {
    repo  *repository.Repository
    whSvc *webhook.Service // 从 webhook 公共模块注入
}

func New(repo *repository.Repository, whSvc *webhook.Service) *Service {
    return &Service{repo: repo, whSvc: whSvc}
}

func (s *Service) CreateProduct(ctx context.Context, req *CreateProductReq) (*Product, error) {
    if err := validateURL(req.URL, req.Platform); err != nil {
        return nil, ErrInvalidURL
    }
    now := time.Now()
    nextCheck := computeNextCheck(now, req.CheckInterval, req.CronExpr)
    product := &Product{
        ID:              uuid.NewString(),
        Name:            req.Name,
        Platform:        req.Platform,
        URL:             req.URL,
        TargetPrice:     req.TargetPrice,
        PriceDropRatio:  req.PriceDropRatio,
        CheckInterval:   req.CheckInterval,
        CronExpr:        req.CronExpr,
        CustomWebhookIDs: req.CustomWebhookIDs,
        Enabled:         true,
        NextCheckAt:     &nextCheck,
        CreatedAt:       now,
        UpdatedAt:       now,
    }
    return s.repo.CreateProduct(ctx, product)
}

// 调度器触发的一次完整检查流程
func (s *Service) ExecuteCheck(ctx context.Context, product *Product, collector collector.Collector) error {
    result, err := collector.FetchPrice(ctx, product.URL)
    if err != nil {
        s.repo.SavePriceRecord(ctx, product.ID, 0, "", err.Error())
        return err
    }

    oldPrice := product.LastPrice
    record, _ := s.repo.SavePriceRecord(ctx, product.ID, result.CurrentPrice, result.Title, "")

    // 判断是否降价
    if s.shouldNotify(oldPrice, result.CurrentPrice, product.TargetPrice, product.PriceDropRatio) {
        // 通过 webhook 公共模块发送通知
        content := fmt.Sprintf(
            "**商品**: [%s](%s)\n**平台**: %s\n**原价**: ¥%.2f\n**现价**: ¥%.2f\n**降幅**: %.1f%%",
            product.Name, product.URL, product.Platform,
            oldPrice, result.CurrentPrice,
            (oldPrice-result.CurrentPrice)/oldPrice*100,
        )

        sendReq := &webhook.SendReq{
            Source:  "shopchecker",
            Title:   "💰 " + product.Name + " 降价了!",
            Content: content,
            MsgType: "markdown",
        }

        // 商品自定义 webhook > 渠道级
        if len(product.CustomWebhookIDs) > 0 {
            sendReq.WebhookIDs = product.CustomWebhookIDs
        } else {
            sendReq.Tags = []string{"shopchecker", "price"}
        }

        sendResult, err := s.whSvc.Send(ctx, sendReq)

        // 记录降价通知业务日志
        summary := "all_success"
        if err != nil || sendResult.Failed > 0 {
            if sendResult.Success > 0 {
                summary = "partial"
            } else {
                summary = "all_failed"
            }
        }
        s.repo.SaveNotificationLog(ctx, product.ID, record.ID, product.CustomWebhookIDs, oldPrice, result.CurrentPrice, summary)
        s.repo.MarkPriceRecordNotified(ctx, record.ID)
    }

    now := time.Now()
    nextCheck := computeNextCheck(now, product.CheckInterval, product.CronExpr)
    s.repo.UpdateCheckResult(ctx, product.ID, result.CurrentPrice, now, nextCheck)
    return nil
}

func (s *Service) shouldNotify(oldPrice, newPrice, targetPrice, dropRatio float64) bool {
    if oldPrice <= 0 {
        return false // 首次抓取，不通知
    }
    if newPrice <= 0 {
        return false // 抓取失败或价格无效
    }
    if newPrice >= oldPrice {
        return false // 未降价
    }
    if targetPrice > 0 && newPrice > targetPrice {
        return false // 未达到目标价
    }
    if dropRatio > 0 {
        dropPercent := (oldPrice - newPrice) / oldPrice * 100
        if dropPercent < dropRatio {
            return false // 降价幅度不足
        }
    }
    return true
}
```

### Repository 层（repository.go）

职责：纯数据访问，SQL 查询，无业务逻辑。

```go
type Repository struct {
    db *sql.DB
}

func New(db *sql.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) FindDueProducts(ctx context.Context, limit int) ([]Product, error) {
    rows, err := r.db.QueryContext(ctx,
        `SELECT id, name, platform, url, target_price, price_drop_ratio,
                last_price, check_interval, cron_expr, custom_webhook_ids
         FROM zzz_shopchecker_products
         WHERE enabled = 1 AND (next_check_at IS NULL OR next_check_at <= NOW())
         ORDER BY next_check_at ASC
         LIMIT ?`, limit)
    // ...
}

func (r *Repository) SavePriceRecord(ctx context.Context, productID string, price float64, title, errMsg string) (*PriceRecord, error) {
    result, err := r.db.ExecContext(ctx,
        `INSERT INTO zzz_shopchecker_price_records (product_id, price, title, error_msg)
         VALUES (?, ?, ?, ?)`, productID, price, title, errMsg)
    // ...
}
```

---

## 4. 采集器（collector/）

```go
// collector.go — 接口定义
type PriceResult struct {
    CurrentPrice  float64
    OriginalPrice float64
    Title         string
    FetchedAt     time.Time
}

type Collector interface {
    Platform() string
    FetchPrice(ctx context.Context, url string) (*PriceResult, error)
}

type Registry struct {
    collectors map[string]Collector
}

func NewRegistry() *Registry {
    return &Registry{collectors: make(map[string]Collector)}
}

func (r *Registry) Register(platform string, c Collector) {
    r.collectors[platform] = c
}

func (r *Registry) Get(platform string) (Collector, bool) {
    c, ok := r.collectors[platform]
    return c, ok
}
```

```go
// jd.go — 京东采集
type JDCollector struct{}

func (c *JDCollector) Platform() string { return "jd" }

func (c *JDCollector) FetchPrice(ctx context.Context, url string) (*PriceResult, error) {
    // 1. HTTP GET 商品页面
    // 2. 解析 HTML / JSON 提取价格
    // 3. 返回 PriceResult
}
```

---

## 5. 通知消息模板

ShopChecker 不自建通知器，降价通知通过 `zzz/webhook` 模块发送。ShopChecker 仅负责构建消息内容并调用 `webhook.Service.Send()`，消息格式和投递由 webhook 模块统一处理。

飞书消息卡片示例（由 webhook 模块的 sender 构建）：

```json
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": {"tag": "plain_text", "content": "💰 某商品 降价了!"},
      "template": "red"
    },
    "elements": [
      {"tag": "div", "text": {"tag": "lark_md", "content": "**商品**: [某商品](https://...)\n**平台**: jd\n**原价**: ¥259.00\n**现价**: ¥189.00\n**降幅**: 27.0%"}}
    ]
  }
}
```

---

## 6. 调度器（scheduler/）

```go
type Scheduler struct {
    repo       *repository.Repository
    collectors *collector.Registry
    whSvc      *webhook.Service
    ticker     *time.Ticker
    stopCh     chan struct{}
    sem        chan struct{} // 并发控制
}

func New(repo *repository.Repository, collectors *collector.Registry, whSvc *webhook.Service) *Scheduler {
    return &Scheduler{
        repo:       repo,
        collectors: collectors,
        whSvc:      whSvc,
        ticker:     time.NewTicker(1 * time.Second), // 每秒扫描一次待检查商品
        stopCh:     make(chan struct{}),
        sem:        make(chan struct{}, 5), // 最多同时检查 5 个商品
    }
}

func (s *Scheduler) Start() {
    go func() {
        for {
            select {
            case <-s.ticker.C:
                s.processDueProducts()
            case <-s.stopCh:
                return
            }
        }
    }()
}

func (s *Scheduler) processDueProducts() {
    products, err := s.repo.FindDueProducts(context.Background(), 20)
    if err != nil || len(products) == 0 {
        return
    }
    for _, p := range products {
        s.sem <- struct{}{}
        go func(product Product) {
            defer func() { <-s.sem }()
            collector, ok := s.collectors.Get(product.Platform)
            if !ok {
                return
            }
            svc := service.New(s.repo, s.whSvc)
            svc.ExecuteCheck(context.Background(), &product, collector)
        }(p)
    }
}

func (s *Scheduler) Stop() {
    close(s.stopCh)
    s.ticker.Stop()
}
```

---

## 7. 错误码定义

在 `errors.go` 中定义：

```go
package shopchecker

const (
    // ShopChecker 模块错误码范围：30000-39999
    ErrProductNotFound     = 30001
    ErrPriceRecordNotFound = 30002
    ErrNotifLogNotFound    = 30003

    ErrInvalidURL          = 30100
    ErrInvalidPlatform     = 30101
    ErrInvalidInterval     = 30102
    ErrInvalidCronExpr     = 30103

    ErrFetchFailed         = 30200
)
```

---

## 8. 迁移 SQL 示例

`migrations/mysql/003_zzz_shopchecker_init.up.sql`:

```sql
CREATE TABLE zzz_shopchecker_products (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    url VARCHAR(2048) NOT NULL,
    image_url VARCHAR(1024) DEFAULT '',
    target_price DECIMAL(12,2) DEFAULT 0,
    price_drop_ratio DECIMAL(5,2) DEFAULT 0,
    last_price DECIMAL(12,2) DEFAULT 0,
    last_checked_at DATETIME NULL,
    next_check_at DATETIME NULL,
    check_interval VARCHAR(20) NOT NULL DEFAULT '1h',
    cron_expr VARCHAR(100) DEFAULT '',
    custom_webhook_ids JSON NULL COMMENT '商品专属 Webhook ID 数组，为空时使用渠道级',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    remark TEXT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_enabled_next_check (enabled, next_check_at),
    INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE zzz_shopchecker_price_records (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    product_id VARCHAR(36) NOT NULL,
    price DECIMAL(12,2) NOT NULL,
    original_price DECIMAL(12,2) DEFAULT 0,
    title VARCHAR(500) DEFAULT '',
    is_notified TINYINT(1) DEFAULT 0,
    raw_data JSON NULL,
    error_msg VARCHAR(500) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_product_created (product_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE zzz_shopchecker_notification_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    product_id VARCHAR(36) NOT NULL,
    price_record_id BIGINT NOT NULL,
    webhook_ids JSON NOT NULL COMMENT '目标 Webhook ID 列表',
    old_price DECIMAL(12,2) DEFAULT 0,
    new_price DECIMAL(12,2) DEFAULT 0,
    result_summary VARCHAR(20) DEFAULT '' COMMENT 'all_success / partial / all_failed',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_product_id (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Webhook 配置表见 zzz/webhook 模块的迁移文件
```