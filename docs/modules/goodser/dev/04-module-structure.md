# Goodser Go 模块结构

## 1. 模块文件组织

```
zzz/goodser/
├── module.go            # Module 接口实现（Name, Init, Close, RegisterRoutes）
├── handler.go           # HTTP 处理器（请求解析 + 响应输出）
├── service.go           # 业务逻辑层（面向接口编程）
├── repository.go        # 数据访问层（MySQL 查询）
├── model.go             # 核心数据模型 + DTO + 枚举
├── errors.go            # 模块级错误码
├── routes.go            # 路由注册（可选，可合并到 module.go）
│
├── model/               # 可选：细分模型文件
│   ├── inventory.go
│   ├── product.go
│   ├── order.go
│   ├── inbound_log.go
│   ├── tag.go
│   ├── status_code.go
│   └── whitelist.go
│
└── handler/             # 可选：拆分多个 handler 文件
    ├── inventory.go
    ├── product.go
    ├── inbound.go
    ├── outbound.go
    ├── image.go
    ├── tag.go
    ├── status_code.go
    └── whitelist.go
```

---

## 2. module.go — 模块入口

```go
package goodser

import (
    "github.com/Allinost/go-backend-core/internal/config"
    "github.com/Allinost/go-backend-core/zzz/goodser/handler"
    "github.com/Allinost/go-backend-core/zzz/goodser/repository"
    "github.com/Allinost/go-backend-core/zzz/goodser/service"
    "github.com/gin-gonic/gin"
)

type Module struct {
    cfg  *config.Config
    h    *handler.Handler
    svc  *service.Service
    repo *repository.Repository
}

func (m *Module) Name() string {
    return "goodser"
}

func (m *Module) Init(cfg *config.Config) error {
    m.cfg = cfg

    // 获取 MySQL 连接池
    pool := database.DB.MySQL["main"]
    if pool == nil {
        return fmt.Errorf("goodser: MySQL main pool not available")
    }

    // 初始化分层
    m.repo = repository.New(pool.DB)
    m.svc = service.New(m.repo)
    m.h = handler.New(m.svc)

    return nil
}

func (m *Module) Close() error {
    return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    // Legacy 兼容端点
    legacy := r.Group("/legacy")
    legacy.POST("/loadInventories", m.h.LoadInventories)
    legacy.POST("/loadProducts", m.h.LoadProducts)
    legacy.POST("/queryProducts", m.h.QueryProducts)
    // ... 约 35 个 legacy 端点

    // RESTful 端点
    // Inventories
    r.GET("/inventories", m.h.ListInventories)
    r.POST("/inventories", m.h.CreateInventory)
    r.PUT("/inventories/:id", m.h.UpdateInventory)
    r.DELETE("/inventories/:id", m.h.DeleteInventory)
    r.GET("/inventories/:id/stats", m.h.InventoryStats)

    // Products
    inv := r.Group("/inventories/:id")
    inv.GET("/products", m.h.ListProducts)
    inv.POST("/products", m.h.CreateProduct)
    inv.GET("/products/:pid", m.h.GetProduct)
    inv.PUT("/products/:pid", m.h.UpdateProduct)
    inv.DELETE("/products/:pid", m.h.DeleteProduct)
    inv.POST("/products/search", m.h.SearchProducts)
    inv.POST("/products/allocate-seq", m.h.AllocateSeq)

    // Inbound
    inv.POST("/inbound/single", m.h.InboundSingle)
    inv.POST("/inbound/batch", m.h.InboundBatch)
    inv.POST("/inbound/search-import", m.h.InboundSearchImport)
    inv.GET("/inbound/logs", m.h.ListInboundLogs)
    inv.GET("/inbound/logs/:logId", m.h.GetInboundLog)
    inv.PUT("/inbound/logs/:logId", m.h.UpdateInboundLog)
    inv.DELETE("/inbound/logs/:logId", m.h.DeleteInboundLog)

    // Outbound
    inv.GET("/outbound/orders", m.h.ListOutboundOrders)
    inv.POST("/outbound/orders", m.h.CreateOutboundOrder)
    inv.GET("/outbound/orders/:oid", m.h.GetOutboundOrder)
    inv.POST("/outbound/orders/:oid/confirm", m.h.ConfirmOutbound)
    inv.POST("/outbound/orders/:oid/cancel", m.h.CancelOutbound)
    inv.POST("/outbound/reserves", m.h.CreateReserveOrder)
    inv.POST("/outbound/reserves/:rid/cancel", m.h.CancelReserve)
    inv.POST("/outbound/reserves/:rid/to-outbound", m.h.ReserveToOutbound)

    // Images
    r.POST("/images/presign", m.h.PresignUpload)
    r.POST("/images/confirm", m.h.ConfirmUpload)
    r.GET("/images/:key/url", m.h.GetImageURL)

    // Settings
    s := r.Group("/settings")
    s.GET("/whitelist", m.h.ListWhitelist)
    s.POST("/whitelist", m.h.AddWhitelist)
    s.DELETE("/whitelist/:id", m.h.RemoveWhitelist)
    s.POST("/whitelist/check", m.h.CheckWhitelist)
    s.GET("/status-codes", m.h.ListStatusCodes)
    s.POST("/status-codes", m.h.AddStatusCode)
    s.PUT("/status-codes/:id", m.h.UpdateStatusCode)
    s.DELETE("/status-codes/:id", m.h.RemoveStatusCode)
    s.GET("/tags", m.h.ListTags)
    s.POST("/tags", m.h.CreateTag)
    s.PUT("/tags/:id", m.h.UpdateTag)
    s.DELETE("/tags/:id", m.h.DeleteTag)
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

// LoadInventories 兼容 legacy 端点
func (h *Handler) LoadInventories(c *gin.Context) {
    inventories, err := h.svc.ListInventories(c.Request.Context())
    if err != nil {
        response.Fail(c, err)
        return
    }
    response.Success(c, inventories)
}
```

### Service 层（service.go）

职责：纯业务逻辑、事务管理、跨模型操作。

```go
type Service struct {
    repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) CreateOutboundOrder(ctx context.Context, req *CreateOutboundReq) (*OutboundOrder, error) {
    // 1. 生成订单号
    // 2. 验证库存是否充足
    // 3. 锁定预留库存
    // 4. 创建出库单
    // 5. 返回结果
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

func (r *Repository) FindProductsByInventory(ctx context.Context, inventoryID string) ([]Product, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT * FROM zzz_products WHERE inventory_id = ? ORDER BY seq_number", inventoryID)
    // ...
}
```

---

## 4. 关键业务逻辑要点

### 4.1 序号分配（Service 层事务）

```go
func (s *Service) AllocateSeq(ctx context.Context, req *AllocateSeqReq) (int, error) {
    tx, err := s.repo.BeginTx(ctx)
    defer tx.Rollback()

    // 1. 先查回收池是否有可复用的序号
    seq, err := s.repo.PopRecycledSeq(tx, req.InventoryID, req.MainZone, req.SubZone)
    if err == nil {
        tx.Commit()
        return seq, nil
    }

    // 2. 无回收序号，从计数器获取新序号
    seq, err = s.repo.IncrementSeqCounter(tx, req.InventoryID, req.MainZone, req.SubZone)
    if err != nil {
        return 0, err
    }

    tx.Commit()
    return seq, nil
}
```

### 4.2 出库库存锁定

```go
func (s *Service) ConfirmOutbound(ctx context.Context, req *ConfirmOutboundReq) (*OutboundOrder, error) {
    tx, err := s.repo.BeginTx(ctx)
    defer tx.Rollback()

    order, err := s.repo.GetOutboundOrderForUpdate(tx, req.ID)
    // 验证状态是否为 pending/reserved

    for _, item := range order.Items {
        // quantity -= item.quantity
        // reserved_quantity -= item.quantity
        s.repo.UpdateProductQuantity(tx, item.ProductID, -item.Quantity, -item.Quantity)
    }

    order.Status = "confirmed"
    s.repo.UpdateOutboundOrderStatus(tx, order.ID, "confirmed")

    tx.Commit()
    return order, nil
}
```

### 4.3 入库增加库存

```go
func (s *Service) InboundSingle(ctx context.Context, req *InboundSingleReq) (*Product, error) {
    tx, err := s.repo.BeginTx(ctx)

    // 查找商品是否存在
    product, err := s.repo.FindProductByCode(tx, req.InventoryID, req.Code)
    if err == sql.ErrNoRows {
        // 不存在 → 分配序号 → 创建新商品
        seq, _ := s.AllocateSeq(ctx, &AllocateSeqReq{
            InventoryID: req.InventoryID,
            MainZone:    req.MainZone,
            SubZone:     req.SubZone,
        })
        req.SeqNumber = seq
        product, err = s.repo.CreateProduct(tx, req)
    } else {
        // 存在 → 增加库存
        err = s.repo.AddProductQuantity(tx, product.ID, req.Quantity)
    }

    // 创建入库日志
    s.repo.CreateInboundLog(tx, &InboundLogInput{
        InventoryID: req.InventoryID,
        Type:        "single",
        Items:       buildInboundLogItems(product, req.Quantity),
    })

    tx.Commit()
    return product, nil
}
```

---

## 5. 错误码定义

在 `errors.go` 中定义：

```go
package goodser

const (
    // Goodser 模块错误码范围：20000-29999
    ErrInventoryNotFound  = 20001
    ErrProductNotFound    = 20002
    ErrOrderNotFound      = 20003
    ErrInboundLogNotFound = 20004
    ErrTagNotFound        = 20005
    ErrStatusCodeNotFound = 20006
    ErrWhitelistNotFound  = 20007

    ErrInvalidZone        = 20100
    ErrInvalidStatusCode  = 20101
    ErrInvalidQuantity    = 20102
    ErrDuplicateTagName   = 20103
    ErrDuplicateCode      = 20104
    ErrDuplicateOpenid    = 20105

    ErrInsufficientStock  = 20200
    ErrInvalidOrderStatus = 20201
    ErrReserveConvertFail = 20202

    ErrImageUpload        = 20300
    ErrImageNotFound      = 20301
)
```

在 handler 中统一转换为框架 `AppError`:

```go
import appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"

func (h *Handler) handleError(c *gin.Context, err error) {
    switch e := err.(type) {
    case *GoodserError:
        response.Fail(c, appErr.New(e.Code, e.Message))
    default:
        response.Fail(c, appErr.New(appErr.CodeInternal, "服务内部错误"))
    }
}
```
