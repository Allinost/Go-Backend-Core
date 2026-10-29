# 系统架构设计

> 本文描述 Go-Backend-Core 的整体架构、启动流程、模块体系与目录结构。
> 模块细节见 [module-design.md](module-design.md)，数据表结构见 [data-model.md](data-model.md)。

## 整体架构

```
┌──────────────────────────────────────────────────────────────────┐
│                          客户端层                                 │
│        Web / 小程序 / App / 第三方系统 / 运维脚本                  │
└────────────────────────────┬─────────────────────────────────────┘
                             │ HTTPS / WSS
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                      接入层（双 Gin 引擎）                         │
│                                                                  │
│  主引擎 :29090（业务 API）          调试引擎 :29092（仅内网）        │
│  ┌──────────────────────────┐      ┌──────────────────────────┐  │
│  │ Logger → CORS → Recovery │      │ Logger → CORS → Recovery │  │
│  │ → RateLimit(100/min)     │      │                          │  │
│  │ → /api/v1/{module}/*     │      │ Swagger UI（环境预设切换） │  │
│  │                          │      │ /api/v1/s0/* 调试与健康检查│  │
│  └──────────────────────────┘      └──────────────────────────┘  │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                    业务模块层（15 个已注册模块）                    │
│                                                                  │
│  基础模块:  s0  s1  s2  auth  migrate  ops  webhook  servicenav   │
│  服务模块:  scheduler  net  crypto                                │
│  业务模块:  goodser  shopchecker  login3rd  ia                    │
│            （handler → service → repository 三层）                │
│            模块间通过 Service Interface 解耦，经 EventBus 异步通信  │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                       公共服务层（internal/services）              │
│  ai  audit  auth  cache  config  crypto  discovery  eventbus     │
│  feature  fileref  migrate  monitor  net  notification           │
│  scheduler  search  storage  template  tracing  ws               │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                     数据访问层（internal/database）                │
│        统一 Manager + 熔断保护（breaker.go）                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │   MySQL  │ │PostgreSQL│ │  Redis   │ │ MinIO /  │            │
│  │ (主数据库)│ │(NAS 场景)│ │(缓存/队列)│ │ RustFS   │            │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘            │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                    可观测 / 基础设施（独立 Compose）                │
│         Prometheus   Grafana   告警    日志轮转(zerolog)           │
└──────────────────────────────────────────────────────────────────┘
```

## 启动流程

`cmd/server/main.go` 按以下顺序执行：

1. **加载配置** — `config.Load("config.yaml")`（Viper，优先级：Docker Secret > 环境变量 `APP_*` > 配置文件 > 默认值）
2. **初始化日志** — zerolog 全局日志 + 独立审计日志 `logs/audit.log`
3. **创建事件总线** — `eventbus.NewLocal()`，并注册 `config.OnChange` 回调（配置热更新时发布 `config.changed` 事件）
4. **初始化数据库** — `database.InitAll(cfg)` 建立全部连接，随后 `config.RegisterReloader(database.DB)` 注册热加载
5. **注册模块** — `modules.Register(...)` 依次注册 15 个模块（注册顺序 = 初始化顺序）
6. **初始化模块** — `modules.InitAll(cfg)`，任一模块初始化失败即 panic（快速失败）
7. **启动主引擎** — `:29090`，挂载中间件后 `modules.RegisterAllRoutes` 将所有模块挂到 `/api/v1/{name}`
8. **启动调试引擎** — `:29092`（goroutine），提供自定义 Swagger UI 与 `s0` 模块
9. **优雅退出** — 依次 `database.CloseAll()` → `modules.CloseAll()`（逆序）→ 关闭日志

## 架构原则

### 1. 模块化（Modular Monolith → Microservices）

- **初期**：单体仓库 + 模块化分层，按业务领域拆分为独立 Go package
- **后期**：按需将高负载模块抽取为独立微服务，通过 gRPC 通信
- **模块边界清晰**：每个业务模块拥有自己的 handler / service / repository 三层

### 2. 依赖方向

```
Handler (路由/入参校验) → Service (业务逻辑) → Repository (数据访问)
                                                      ↓
                                              MySQL / Redis / RustFS
                                         (PostgreSQL 仅 NAS 场景)
```

- 依赖只能从外层指向内层，禁止循环依赖
- 模块间通过 `Service Interface` 解耦

### 3. 配置中心化

- 所有模块共享一个配置文件 `config.yaml`（Viper 管理）
- 环境变量覆盖配置项（12-Factor App）
- 支持 fsnotify 热更新，变更通过 EventBus 广播

### 4. 统一中间件链

```
Request → Logger → CORS → Recovery → RateLimit → /api/v1/{module} 路由
                                                            ↓
                                        模块内按需启用 JWT Auth / RBAC
                                                            ↓
                                                          Response
```

认证与鉴权按路由粒度启用，不在全局链路上强制拦截。

## 模块体系

模块是本项目的核心抽象，接口定义在 `internal/modules/registry.go`：

```go
type Module interface {
    Name() string                      // 模块唯一标识（路由前缀 /api/v1/{name}）
    RegisterRoutes(r *gin.RouterGroup) // 注册模块路由
    Init(cfg *config.Config) error     // 模块初始化
    Close() error                      // 模块关闭（资源清理）
}
```

**注册表机制**：

- 全局 `registry`（map）+ `order`（slice）保存注册顺序
- `InitAll` 按注册顺序初始化，保证依赖模块（如 webhook）先于消费方就绪
- `CloseAll` 逆序关闭，确保依赖方先释放资源
- 重复注册 panic；初始化失败 panic（快速失败）
- `RegisterAllRoutes(r)` 挂载全部模块；`RegisterRoutesTo(r, "s0")` 可按名挂到指定引擎

**已注册模块（15 个）**：

| 模块 | 位置 | 说明 |
| --- | --- | --- |
| `s0` | `internal/modules/s0` | 调试与健康检查（ping/health/echo/metrics/info/modules），同时挂载到调试引擎 |
| `s1` | `internal/modules/s1` | 自建服务占位 |
| `s2` | `internal/modules/s2` | 自建服务占位 |
| `auth` | `internal/modules/auth` | JWT 认证与用户体系 |
| `migrate` | `internal/modules/migrate` | 数据库迁移 |
| `ops` | `internal/modules/ops` | 运维接口 |
| `webhook` | `internal/modules/webhook` | Webhook 订阅与投递 |
| `servicenav` | `internal/modules/servicenav` | 服务导航 |
| `scheduler` | `internal/services/scheduler` | 定时/周期任务调度 |
| `net` | `internal/services/net` | 网络通信服务（HTTP/gRPC/熔断/重试） |
| `crypto` | `internal/services/crypto` | 加解密服务 |
| `goodser` | `zzz/goodser` | 商品服务 |
| `shopchecker` | `zzz/shopchecker` | 店铺数据采集（淘宝/京东/拼多多） |
| `login3rd` | `zzz/login3rd` | 第三方登录（含 chromedp 浏览器自动化） |
| `ia` | `zzz/ia` | 投资分析（行情/筛选/告警/建议） |

> 公共服务本身不一定是模块：只有实现 `Module` 接口并调用 `modules.Register` 的才拥有路由。
> `internal/services` 下的其余服务（cache、eventbus、tracing 等）以库形式被模块调用。

## 目录结构

```
go-backend-core/
├── cmd/                        # 各程序入口
│   ├── server/                 # 主服务入口（main.go + swagger docs.go）
│   ├── migrate/                # 数据库迁移入口
│   └── backup/                 # 备份命令行入口
├── internal/                   # 私有代码（不对外暴露）
│   ├── config/                 # 配置加载、热更新、OnChange 回调
│   ├── database/               # 统一数据访问层（manager + 熔断）
│   │   ├── mysql/ postgres/ redis/ minio/ rustfs/
│   │   └── breaker.go
│   ├── middleware/             # 通用中间件：logger/CORS/ratelimit/rbac/metrics
│   ├── modules/                # 业务模块 + 模块注册表 registry.go
│   │   ├── s0/ s1/ s2/ auth/ migrate/ ops/ servicenav/ webhook/
│   │   ├── registry.go         # Module 接口与全局注册表
│   │   └── registry_test.go
│   ├── services/               # 公共服务（20 个）
│   │   ├── ai/ audit/ auth/ cache/ config/ crypto/ discovery/
│   │   ├── eventbus/ feature/ fileref/ migrate/ monitor/ net/
│   │   ├── notification/ scheduler/ search/ storage/ template/
│   │   └── tracing/ ws/
│   ├── pkg/                    # 内部工具库
│   │   ├── logger/             # 结构化日志 + 审计日志 + 轮转
│   │   ├── response/           # 统一响应格式
│   │   ├── errors/             # 错误码定义
│   │   └── crypto/             # 加解密与压缩工具
│   ├── nas/                    # NAS 连接层（预留，当前为空）
│   └── cloud/                  # 云服务连接层（预留，当前为空）
├── zzz/                        # 业务域模块（与 internal 解耦的业务包）
│   ├── goodser/                # 商品服务
│   ├── ia/                     # 投资分析（含 quote/ 行情子包）
│   ├── login3rd/               # 第三方登录（含 provider/ browser/ 子包）
│   └── shopchecker/            # 店铺采集（含 collector/ scheduler/ 子包）
├── pkg/                        # 公开库（可被外部引用，均含测试与 fuzz）
│   ├── apierror/               # API 错误码与错误封装
│   ├── crypto/                 # 加解密、哈希、压缩
│   ├── net/                    # HTTP/gRPC/TCP/UDP/DNS/熔断/重试/指标
│   └── pagination/             # 分页与查询构建
├── api/
│   ├── proto/                  # gRPC proto 定义
│   └── swagger/                # swaggo 生成的 API 文档
├── migrations/                 # 数据库版本化迁移脚本
│   ├── mysql/  postgres/
├── docs/                       # 项目文档（架构/模块/API/发布记录）
├── deploy/                     # 部署配置
│   ├── docker/  compose/  grafana/  prometheus/
├── scripts/                    # 运维脚本（run/test/lint/coverage 等）
├── aiPrompt/                   # AI 提示词与规范
├── .github/workflows/ci.yml    # CI 流水线
├── config.yaml                 # 配置文件
├── docker-compose.yml          # 应用编排
├── docker-compose.monitoring.yml  # 监控编排
└── Makefile                    # swagger/build/test/lint/docker-build
```

### 目录约定

| 目录 | 可见性 | 用途 |
| --- | --- | --- |
| `internal/` | 仅本模块 | 框架核心：配置、中间件、模块注册表、公共服务 |
| `zzz/` | 仅本模块 | 业务域模块，按垂直切片组织（handler/service/repository/model/errors） |
| `pkg/` | 公开 | 与业务无关、可被外部项目引用的通用库 |
| `cmd/` | 入口 | 每个子目录一个可执行程序 |
