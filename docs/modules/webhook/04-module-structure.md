# Webhook Go 模块结构

## 1. 模块文件组织

```
zzz/webhook/
├── module.go              # Module 接口实现 + Service 暴露
├── handler.go             # HTTP 处理器（配置管理 API）
├── service.go             # 核心业务逻辑（投递 + 管理）
├── repository.go          # 数据访问层
├── model.go               # 数据模型 + DTO
├── errors.go              # 模块级错误码
│
├── sender/                # 消息发送器
│   ├── sender.go          # Sender 接口 + Factory
│   ├── feishu.go          # 飞书消息格式化 + 发送
│   ├── dingtalk.go        # 钉钉消息格式化 + 发送
│   ├── wecom.go           # 企业微信消息格式化 + 发送
│   ├── github.go          # GitHub Webhook 事件发送
│   └── generic.go         # 通用 HTTP Webhook
│
├── limiter/               # 频率限制
│   ├── limiter.go         # Limiter 接口
│   └── redis_limiter.go   # Redis 滑动窗口实现
│
└── dedup/                 # 去重
    └── dedup.go           # 内容哈希去重
```

---

## 2. module.go — 模块入口

```go
package webhook

import (
    "github.com/Allinost/go-backend-core/internal/config"
    "github.com/Allinost/go-backend-core/internal/database"
    "github.com/Allinost/go-backend-core/internal/modules"
    "github.com/Allinost/go-backend-core/zzz/webhook/sender"
    "github.com/Allinost/go-backend-core/zzz/webhook/limiter"
    "github.com/Allinost/go-backend-core/zzz/webhook/dedup"
    "github.com/gin-gonic/gin"
)

type Module struct {
    cfg  *config.Config
    h    *handler.Handler
    svc  *Service
    repo *repository.Repository
}

func (m *Module) Name() string { return "webhook" }

func (m *Module) Init(cfg *config.Config) error {
    pool := database.DB.MySQL["main"]
    m.repo = repository.New(pool.DB)

    // 初始化发送器工厂
    senderFactory := sender.NewFactory()

    // 初始化限流器
    rateLimiter := limiter.NewRedisLimiter(database.DB.Redis["main"])

    // 初始化去重器
    deduper := dedup.New(database.DB.Redis["main"])

    m.svc = NewService(m.repo, senderFactory, rateLimiter, deduper)
    m.h = handler.New(m.svc)
    return nil
}

func (m *Module) Close() error { return nil }

// GetService 供其他模块获取 Service 实例
func (m *Module) GetService() *Service {
    return m.svc
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    // 配置管理
    r.GET("/configs", m.h.ListConfigs)
    r.POST("/configs", m.h.CreateConfig)
    r.GET("/configs/:id", m.h.GetConfig)
    r.PUT("/configs/:id", m.h.UpdateConfig)
    r.DELETE("/configs/:id", m.h.DeleteConfig)
    r.POST("/configs/:id/status", m.h.UpdateStatus)
    r.POST("/configs/:id/test", m.h.TestSend)

    // 投递服务
    r.POST("/send", m.h.Send)

    // 投递日志
    r.GET("/delivery-logs", m.h.ListDeliveryLogs)

    // 统计
    r.GET("/stats", m.h.Stats)
    r.GET("/configs/:id/stats", m.h.ConfigStats)
}
```

---

## 3. Service 层 — 核心接口

Service 是对外暴露的核心 API，其他业务模块通过此接口发送消息。

### 3.1 Service 结构

```go
type Service struct {
    repo          *Repository
    senderFactory *sender.Factory
    rateLimiter   limiter.Limiter
    deduper       *dedup.Deduper
}

func NewService(
    repo *Repository,
    senderFactory *sender.Factory,
    rateLimiter limiter.Limiter,
    deduper *dedup.Deduper,
) *Service {
    return &Service{
        repo:          repo,
        senderFactory: senderFactory,
        rateLimiter:   rateLimiter,
        deduper:       deduper,
    }
}
```

### 3.2 发送流程

```go
func (s *Service) Send(ctx context.Context, req *SendReq) (*SendResult, error) {
    // 1. 解析目标 Webhook 列表
    webhooks, err := s.resolveWebhooks(ctx, req.WebhookIDs, req.Tags)
    if err != nil {
        return nil, err
    }

    result := &SendResult{}
    sem := make(chan struct{}, 5) // 并发控制，最多同时发 5 个

    for _, wh := range webhooks {
        // 2. 白名单过滤
        if !s.matchWhitelist(wh, req.Content) {
            result.Results = append(result.Results, WebhookResult{
                WebhookID: wh.ID, Status: "filtered",
            })
            continue
        }

        sem <- struct{}{}
        go func(wh WebhookConfig) {
            defer func() { <-sem }()
            whResult := s.sendToWebhook(ctx, wh, req)
            result.Results = append(result.Results, whResult)
            if whResult.Status == "success" {
                result.Success++
            } else {
                result.Failed++
            }
        }(wh)
    }

    // 等待所有 goroutine 完成
    for i := 0; i < cap(sem); i++ {
        sem <- struct{}{}
    }

    return result, nil
}

func (s *Service) sendToWebhook(ctx context.Context, wh WebhookConfig, req *SendReq) WebhookResult {
    // 1. 频率限制检查
    allowed, err := s.rateLimiter.Allow(ctx, wh.ID, wh.MaxPerMinute, wh.MaxPerHour)
    if err != nil || !allowed {
        s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
            WebhookID: wh.ID, Source: req.Source, Title: req.Title,
            Status: string(DeliveryRateLimited),
        })
        return WebhookResult{WebhookID: wh.ID, Status: "rate_limited"}
    }

    // 2. 去重检查
    contentHash := sha256Hash(req.Content)
    if wh.DedupWindow > 0 {
        isDup, _ := s.deduper.IsDuplicate(ctx, wh.ID, contentHash, wh.DedupWindow)
        if isDup {
            s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
                WebhookID: wh.ID, Source: req.Source, Title: req.Title,
                ContentHash: contentHash, Status: string(DeliveryDeduped),
            })
            return WebhookResult{WebhookID: wh.ID, Status: "deduped"}
        }
    }

    // 3. 获取对应类型的 Sender 并发送
    snd := s.senderFactory.Get(WebhookType(wh.Type))
    start := time.Now()
    respCode, respBody, err := snd.Send(ctx, wh, req)
    duration := time.Since(start)

    status := "success"
    errMsg := ""
    if err != nil {
        status = "failed"
        errMsg = err.Error()
    }

    // 4. 记录投递日志
    s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
        WebhookID:    wh.ID,
        Source:       req.Source,
        Title:        req.Title,
        ContentHash:  contentHash,
        Status:       status,
        ResponseCode: respCode,
        ErrorMsg:     errMsg,
        DurationMS:   int(duration.Milliseconds()),
    })

    // 5. 更新 Webhook 统计
    s.repo.UpdateStats(ctx, wh.ID, status == "success")

    return WebhookResult{
        WebhookID:  wh.ID,
        Status:     status,
        Error:      errMsg,
        DurationMS: int(duration.Milliseconds()),
    }
}

func (s *Service) resolveWebhooks(ctx context.Context, ids, tags []string) ([]WebhookConfig, error) {
    if len(ids) > 0 {
        return s.repo.FindByIDs(ctx, ids)
    }
    if len(tags) > 0 {
        return s.repo.FindByTags(ctx, tags)
    }
    return s.repo.FindActive(ctx)
}

func (s *Service) matchWhitelist(wh WebhookConfig, content string) bool {
    if len(wh.WhitelistKeywords) == 0 {
        return true
    }
    for _, kw := range wh.WhitelistKeywords {
        matched := strings.Contains(content, kw)
        if wh.WhitelistMode == "any" && matched {
            return true
        }
        if wh.WhitelistMode == "all" && !matched {
            return false
        }
    }
    return wh.WhitelistMode == "all"
}
```

---

## 4. 发送器（sender/）

```go
// sender.go — 接口定义
type Sender interface {
    Type() WebhookType
    Send(ctx context.Context, wh WebhookConfig, req *SendReq) (respCode int, respBody string, err error)
}

type Factory struct {
    senders map[WebhookType]Sender
}

func NewFactory() *Factory {
    return &Factory{
        senders: map[WebhookType]Sender{
            WebhookTypeFeishu:   &FeishuSender{},
            WebhookTypeDingtalk: &DingtalkSender{},
            WebhookTypeWecom:    &WecomSender{},
            WebhookTypeGitHub:   &GitHubSender{},
            WebhookTypeGeneric:  &GenericSender{},
        },
    }
}

func (f *Factory) Get(t WebhookType) Sender {
    return f.senders[t]
}
```

### 飞书发送器示例

```go
// feishu.go
type FeishuSender struct{}

func (s *FeishuSender) Type() WebhookType { return WebhookTypeFeishu }

func (s *FeishuSender) Send(ctx context.Context, wh WebhookConfig, req *SendReq) (int, string, error) {
    body := s.buildMessage(req)

    // 如果有 signing_key，计算签名
    headers := map[string]string{"Content-Type": "application/json"}
    if wh.SigningKey != "" {
        timestamp, sign := signFeishu(wh.SigningKey)
        headers["X-Timestamp"] = timestamp
        headers["X-Sign"] = sign
    }

    jsonBody, _ := json.Marshal(body)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(jsonBody))
    for k, v := range headers {
        httpReq.Header.Set(k, v)
    }

    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        return 0, "", err
    }
    defer resp.Body.Close()
    respBody, _ := io.ReadAll(resp.Body)
    return resp.StatusCode, string(respBody), nil
}

func (s *FeishuSender) buildMessage(req *SendReq) map[string]any {
    if req.MsgType == "card" {
        return buildFeishuCard(req)
    }
    // 默认使用 interactive 消息卡片模板
    return map[string]any{
        "msg_type": "interactive",
        "card": map[string]any{
            "header": map[string]any{
                "title":    map[string]any{"tag": "plain_text", "content": req.Title},
                "template": "red",
            },
            "elements": []map[string]any{
                {"tag": "div", "text": map[string]any{"tag": "lark_md", "content": req.Content}},
            },
        },
    }
}
```

### GitHub 发送器示例

```go
// github.go
type GitHubSender struct{}

func (s *GitHubSender) Type() WebhookType { return WebhookTypeGitHub }

func (s *GitHubSender) Send(ctx context.Context, wh WebhookConfig, req *SendReq) (int, string, error) {
    // GitHub Webhook 需要构建事件 payload + HMAC 签名
    payload := map[string]any{
        "ref": "refs/heads/main",
        "commits": []map[string]any{
            {
                "id":      sha256Hash(req.Content)[:12],
                "message": req.Title + "\n\n" + req.Content,
            },
        },
    }
    jsonBody, _ := json.Marshal(payload)

    httpReq, _ := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(jsonBody))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-GitHub-Event", "push")
    httpReq.Header.Set("X-GitHub-Delivery", uuid.NewString())

    // HMAC-SHA256 签名
    if wh.SigningKey != "" {
        mac := hmac.New(sha256.New, []byte(wh.SigningKey))
        mac.Write(jsonBody)
        sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
        httpReq.Header.Set("X-Hub-Signature-256", sig)
    }

    resp, err := http.DefaultClient.Do(httpReq)
    // ...
}
```

---

## 5. 限流器（limiter/）

```go
type Limiter interface {
    Allow(ctx context.Context, key string, maxPerMinute, maxPerHour int) (bool, error)
}

type RedisLimiter struct {
    client *redis.Client
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, maxPerMinute, maxPerHour int) (bool, error) {
    now := time.Now()
    minuteKey := fmt.Sprintf("zzz:webhook:ratelimit:%s:minute:%d", key, now.Unix()/60)
    hourKey   := fmt.Sprintf("zzz:webhook:ratelimit:%s:hour:%d",   key, now.Unix()/3600)

    pipe := l.client.Pipeline()
    minuteCount := pipe.Incr(ctx, minuteKey)
    pipe.Expire(ctx, minuteKey, 70*time.Second)
    hourCount := pipe.Incr(ctx, hourKey)
    pipe.Expire(ctx, hourKey, 3700*time.Second)
    _, err := pipe.Exec(ctx)

    if err != nil {
        return true, err // 限流出错时放行，避免业务阻塞
    }
    if minuteCount.Val() > int64(maxPerMinute) {
        return false, nil
    }
    if hourCount.Val() > int64(maxPerHour) {
        return false, nil
    }
    return true, nil
}
```

---

## 6. 去重器（dedup/）

```go
type Deduper struct {
    client *redis.Client
}

func (d *Deduper) IsDuplicate(ctx context.Context, webhookID, contentHash string, windowSec int) (bool, error) {
    key := fmt.Sprintf("zzz:webhook:dedup:%s:%s", webhookID, contentHash)
    existed, err := d.client.SetNX(ctx, key, 1, time.Duration(windowSec)*time.Second).Result()
    if err != nil {
        return false, err
    }
    return !existed, nil // 如果 key 已存在，说明是重复内容
}
```

---

## 7. 业务模块集成示例

### ShopChecker 集成

```go
// zzz/shopchecker/notifier/shopchecker_notifier.go
type ShopCheckerNotifier struct {
    whSvc *webhook.Service
}

func New(whSvc *webhook.Service) *ShopCheckerNotifier {
    return &ShopCheckerNotifier{whSvc: whSvc}
}

func (n *ShopCheckerNotifier) NotifyPriceDrop(ctx context.Context, product *model.Product, oldPrice, newPrice float64) error {
    dropRatio := (oldPrice - newPrice) / oldPrice * 100
    content := fmt.Sprintf(
        "**商品**: [%s](%s)\n**平台**: %s\n**原价**: ¥%.2f\n**现价**: ¥%.2f\n**降幅**: %.1f%%",
        product.Name, product.URL, product.Platform, oldPrice, newPrice, dropRatio,
    )

    // 商品有自定义 Webhook 就用商品的，否则用标签匹配渠道级 Webhook
    var req *webhook.SendReq
    if len(product.CustomWebhooks) > 0 {
        req = &webhook.SendReq{
            WebhookIDs: product.CustomWebhooks,
            Source:     "shopchecker",
            Title:      "💰 " + product.Name + " 降价了!",
            Content:    content,
            MsgType:    "markdown",
        }
    } else {
        req = &webhook.SendReq{
            Tags:    []string{"shopchecker", "price"},
            Source:  "shopchecker",
            Title:   "💰 " + product.Name + " 降价了!",
            Content: content,
            MsgType: "markdown",
        }
    }

    _, err := n.whSvc.Send(ctx, req)
    return err
}
```

```go
// zzz/shopchecker/module.go
func (m *ShopCheckerModule) Init(cfg *config.Config) error {
    whModule := modules.Get("webhook").(*webhook.Module)
    whSvc := whModule.GetService()

    m.notifier = notifier.New(whSvc)
    // ...
}
```

---

## 8. 错误码定义

```go
package webhook

const (
    // Webhook 模块错误码范围：40000-40999
    ErrWebhookNotFound     = 40001
    ErrDeliveryLogNotFound = 40002

    ErrInvalidType         = 40100
    ErrInvalidURL          = 40101
    ErrInvalidStatus       = 40102
    ErrInvalidWhitelist    = 40103
    ErrInvalidRateLimit    = 40104

    ErrSendFailed          = 40200
    ErrAllWebhooksFailed   = 40201
    ErrNoWebhookTarget     = 40202
)
```