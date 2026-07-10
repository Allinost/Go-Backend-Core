# Goodser Phase 3 开发架构概览

## 1. 项目背景

Goodser 是一个**个人/小团队物品库存管理系统**（微信小程序前端），管理物品的入库、出库、预留、标签、状态码等库存全生命周期。

### 架构定位

Goodser 作为 Go-Backend-Core 框架下的第一个实际业务模块（Phase 3），复用 Phase 1 和 Phase 2 产出的全部公共基础设施（数据库连接池、缓存、认证、日志、事件总线等）。

### 命名空间隔离策略

为区分**业务模块代码**与**框架公共代码**，采用以下隔离方案：

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | 存储至项目根 `zzz/` 目录下 | `zzz/goodser/` |
| 表名 | `zzz_{模块名}_{表名}` | `zzz_goodser_inventories` |
| Redis Key | 添加 `zzz:` 前缀 | `zzz:goodser:cache:...` |
| config 配置段 | 添加 `zzz_` 前缀 | `zzz_goodser.xxx` |

> 使用**当前开发库**（`go_backend_core`），不新建数据库。通过表名前缀 `zzz_{模块名}_` 实现逻辑隔离。
> 当后续业务模块（知识管理 s2、音乐管理 s3 等）也通过 `zzz/` 目录存储时，各自使用自身的 `zzz_{模块名}_` 表名前缀。

---

## 2. 代码目录结构

```
go-backend-core/
├── zzz/                              # 实际业务代码根目录（与 internal/、pkg/ 平级）
│   └── goodser/                      # Goodser 业务模块
│       ├── module.go                 # Module 接口实现（Name="goodser"）
│       ├── handler.go                # HTTP 处理器
│       ├── service.go                # 业务逻辑层
│       ├── repository.go             # 数据访问层
│       ├── model.go                  # 数据模型 / DTO
│       ├── errors.go                 # 模块级错误码
│       ├── routes.go                 # 路由注册（可选，可在 module.go 中完成）
│       └── model/                    # 细分模型（可选）
│           ├── inventory.go
│           ├── product.go
│           ├── order.go
│           ├── inbound_log.go
│           ├── tag.go
│           ├── status_code.go
│           └── whitelist.go
├── docs/modules/goodser/dev/         # Goodser 开发文档
├── migrations/mysql/                 # 数据库迁移文件
│   ├── 002_zzz_goodser_init.up.sql
│   └── 002_zzz_goodser_init.down.sql
└── config.yaml                       # 配置中新增 zzz_goodser 配置段
```

### 模块注册

在 `cmd/server/main.go` 中注册：

```go
import "github.com/Allinost/go-backend-core/zzz/goodser"

modules.Register(&goodser.Module{})
```

路由自动挂载至 `/api/v1/goodser/...`。

---

## 3. 依赖关系

```
Goodser Module
  ├── internal/database           # DBManager 获取 MySQL Pool
  │   └── mysql:main
  ├── internal/services/cache     # 多级缓存（可选）
  ├── internal/services/eventbus  # 事件总线（库存变更通知）
  ├── internal/services/monitor   # 健康检查集成
  ├── internal/services/scheduler # 定时任务（库存预警等）
  ├── internal/services/search    # 全文搜索（商品搜索）
  ├── internal/pkg/response       # 统一响应格式
  └── internal/pkg/errors         # 错误码体系
```

---

## 4. 核心业务流程

### 4.1 入库流程

```
客户端 POST /api/v1/goodser/inbound/single
  → 解析入库请求（商品信息 + 数量）
  → 查询商品是否存在
    ├─ 存在 → 增加库存数量（quantity + inbound_qty）
    └─ 不存在 → 分配序号 → 创建商品
  → 创建入库日志（inbound_logs）
  → 返回结果
```

### 4.2 出库/预留流程

```
客户端 POST /api/v1/goodser/outbound/create
  → 创建出库单（status=pending）
  → 锁定库存（reserved_quantity += outbound_qty）
  → 返回出库单

客户端 POST /api/v1/goodser/outbound/confirm
  → 确认出库
  → quantity -= outbound_qty, reserved_quantity -= outbound_qty
  → status=confirmed

客户端 POST /api/v1/goodser/outbound/cancel
  → 取消出库
  → reserved_quantity -= outbound_qty
  → status=cancelled
```

### 4.3 序号分配

```
分配序号 allocateSeq(inventory_id, main_zone, sub_zone):
  → 查询 recycled_seq_numbers 是否有回收序号
    ├─ 有 → 取最小回收序号，删除回收记录
    └─ 无 → seq_counters 的 current_max + 1，更新计数器
  → 返回 seq_number
```

---

## 5. 认证策略

当前阶段**不接入认证**，所有接口可直接访问。后续接入框架 AUTH 模块时，在路由组添加中间件即可：

```go
// 后续启用认证时：
protected := r.Group("")
protected.Use(middleware.AuthRequired(...))
protected.POST("/inventories", m.h.CreateInventory)
```

---

## 6. 数据库设计

- **数据库**: 使用当前开发库 `go_backend_core`（不新建库）
- **表名格式**: `zzz_{模块名}_{表名}`，例如 `zzz_goodser_inventories`
- 详见 `02-data-model.md`
