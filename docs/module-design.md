# 模块化设计

## 模块划分总览

```
┌──────────────────────────────────────────────────────────────┐
│                    模块全景                                    │
├─────────────┬───────────────────────┬────────────────────────┤
│   类别       │  模块名               │  职责                   │
├─────────────┼───────────────────────┼────────────────────────┤
│ 基础设施模块 │  nas                  │  NAS 连接管理            │
│             │  cloud                │  云服务连接管理           │
│             │  database             │  统一数据访问层（PG/MySQL/Redis/MinIO/RustFS）│
├─────────────┼───────────────────────┼────────────────────────┤
│ 公共服务模块 │  auth                 │  认证授权                │
│             │  user                 │  用户管理                │
│             │  notify               │  消息通知/多渠道推送       │
│             │  file                 │  文件存储                │
│             │  ups                  │  UPS 状态监控              │
│             │  lanctl               │  局域网设备管理            │
│             │  scheduler            │  定时/周期任务调度          │
│             │  backup               │  数据备份与恢复            │
│             │  migration            │  数据库迁移管理            │
│             │  discovery            │  节点发现与P2P连接          │
│             │  crypto               │  加密解密与压缩解压缩        │
│             │  ai                   │  AI 推理服务（Ollama/GPU/API）│
│             │  logger               │  日志管理与审计              │
│             │  monitor              │  监控与告警                  │
│             │  config               │  配置信息管理                │
│             │  webhook              │  出站/入站 Webhook           │
│             │  net                  │  网络通信（HTTP/TCP/UDP/gRPC）│
│             │  eventbus             │  事件总线/消息队列            │
│             │  cache                │  缓存管理                  │
│             │  search               │  全文搜索服务              │
│             │  apikey               │  API 密钥管理              │
│             │  dataio               │  数据导入导出              │
│             │  feature              │  特性开关/灰度发布          │
│             │  template             │  模板引擎                  │
│             │  rules                │  规则引擎                  │
│             │  ratelimit            │  分布式限流                │
│             │  mqtt                 │  MQTT 协议客户端           │
│             │  filesystem           │  文件系统监听与索引         │
│             │  tagging              │  通用标签/分类系统          │
│             │  jobs                 │  后台任务管理（进度/取消）  │
│             │  media                │  媒体库管理（音频元数据）   │
│             │  analytics            │  数据聚合与报表引擎         │
│             │  plugin               │  插件注册与扩展机制         │
├─────────────┼───────────────────────┼────────────────────────┤
│ 业务模块    │  s0 (debug)           │  调测/调试服务           │
│             │  s1 (business-a)      │  自建业务A               │
│             │  s2 (business-b)      │  自建业务B               │
├─────────────┼───────────────────────┼────────────────────────┤
│ 公共层      │  pkg                  │  工具库/通用组件          │
│             │  middleware           │  中间件                  │
│             │  config               │  配置管理                │
└─────────────┴───────────────────────┴────────────────────────┘
```

## 模块接口规范

每个模块遵循统一的三层架构：

```
module/
├── handler.go       # HTTP 处理器（入参解析、响应输出）
├── service.go       # 业务逻辑（接口定义 + 实现）
├── repository.go    # 数据访问层
├── model.go         # 数据模型 / DTO
├── routes.go        # 路由注册
└── errors.go        # 模块级错误码
```

### 模块注册机制

```go
// Module 接口 - 所有模块需实现
type Module interface {
    Name() string
    RegisterRoutes(r *gin.RouterGroup)
    Init(cfg *config.Config) error
    Close() error
}
```

```go
// 全局模块注册器
var registry = make(map[string]Module)

func Register(m Module) {
    registry[m.Name()] = m
}

func InitAll(cfg *config.Config) {
    for _, m := range registry {
        m.Init(cfg)
        // 自动挂载路由: /api/v1/{module_name}/*
        m.RegisterRoutes(router.Group("/api/v1/" + m.Name()))
    }
}
```

## 1. NAS 连接模块 (`internal/nas/`)

管理后端对 NAS 设备的数据库/缓存服务连接。

### 子模块

| 子模块 | 功能 | 说明 |
|--------|------|------|
| `nas/mysql` | MySQL 连接池管理 | 用于连接 NAS 上运行的 MySQL |
| `nas/postgres` | PostgreSQL 连接池 | 连接 NAS 上的 PostgreSQL |
| `nas/redis` | Redis 客户端管理 | 连接 NAS 上的 Redis |

### 设计要点

- 各子模块提供 `GetConn()` / `Close()` 方法
- 支持多实例配置（可连接多个 NAS 设备）
- 健康检查 + 断线自动重连
- 连接配置从 `config.yaml` 读取

```go
type NASConfig struct {
    MySQL    []DBConfig    `yaml:"mysql"`
    Postgres []DBConfig    `yaml:"postgres"`
    Redis    []RedisConfig `yaml:"redis"`
}
```

## 2. 云服务连接模块 (`internal/cloud/`)

管理公有云/私有云服务的连接。

### 子模块

| 子模块 | 功能 |
|--------|------|
| `cloud/rustfs` | RustFS 对象存储客户端封装（S3 兼容 API） |

### 设计要点

- 统一的云服务配置段
- RustFS 封装 Put/Get/Delete/List 操作
- 支持多 bucket 管理

## 3. 调试服务 S0 (`internal/modules/s0/`)

用于系统调测、接口测试、健康检查。

| 功能 | 说明 |
|------|------|
| `/ping` | 存活检查 |
| `/health` | 各依赖服务健康状态聚合 |
| `/echo` | 回显请求（调试用） |
| `/config` | 查看当前配置（需鉴权） |
| `/metrics` | Prometheus 指标暴露 |

## 4. 自建服务 S1 / S2 (`internal/modules/s1/`, `internal/modules/s2/`)

业务占位模块，为后续业务开发预留完整结构：

- 独立的 handler / service / repository
- 独立的数据库表（各自 schema 前缀，如 `s1_*`, `s2_*`）
- 支持独立启停配置

## 5. 公共服务

| 服务 | 说明 |
|------|------|
| `auth` | JWT 签发/验证、OAuth2 社交登录、Casbin RBAC |
| `user` | 用户注册/登录/资料管理、多端设备绑定 |
| `notify` | 站内信、推送通知（各平台 Push） |
| `file` | 文件上传下载、RustFS 代理、图片处理 |

### auth 服务 — OAuth2 社交登录架构

支持六种第三方 OAuth 登录：微信、飞书、QQ、Apple ID、华为账号、荣耀账号。

#### 统一 Provider 接口

```go
type SocialProvider interface {
    Name() string                    // 唯一标识: wechat/feishu/qq/apple/huawei/honor
    AuthURL(state string) string     // 构造授权跳转 URL
    Exchange(ctx, code string) (*TokenInfo, error)  // code 换 token
    GetUserInfo(ctx, token) (*SocialUser, error)     // 获取用户信息
}

type SocialUser struct {
    OpenID      string // 平台用户唯一 ID
    UnionID     string // 平台统一 ID（微信）
    Nickname    string
    AvatarURL   string
    Gender      int
    Email       string
    Phone       string
}
```

#### 登录流程

```
客户端                                     服务端
  │                                         │
  │  1. 请求登录 → GET /api/v1/auth/wechat/url  │
  │  ◄──── 返回授权 URL + state ──────────── │
  │                                         │
  │  2. 跳转微信授权页，用户确认               │
  │                                         │
  │  3. 回调携带 code ─── POST /api/v1/auth/wechat/callback → │
  │                              │                              │
  │                         4. Exchange(code) → access_token    │
  │                         5. GetUserInfo(token) → 用户信息    │
  │                         6. 查询/创建本地用户                 │
  │                         7. 签发 JWT                         │
  │  ◄──── 返回 JWT + 用户信息 ─────────────────────────────── │
```

#### 路由注册

```go
func (a *AuthModule) RegisterRoutes(r *gin.RouterGroup) {
    auth := r.Group("/auth")
    for _, p := range a.providers {
        name := p.Name()
        auth.GET("/" + name + "/url", a.GetAuthURL(name))     // 获取授权 URL
        auth.POST("/" + name + "/callback", a.Callback(name))  // 回调处理
        auth.POST("/" + name + "/bind", a.Bind(name))          // 绑定已有账号
        auth.POST("/" + name + "/unbind", a.Unbind(name))      // 解绑
    }
    // 微信小程序特殊路由（code 直换登录态）
    auth.POST("/miniapp/login", a.MiniAppLogin)
}
```

#### 多账号绑定

支持一个本地用户绑定多个第三方账号（含同平台多个账号），详见 `docs/social-login.md#多账号绑定设计`。

#### 平台差异处理

| 特性 | 微信 | 飞书 | QQ | Apple ID | 华为 | 荣耀 |
|------|------|------|----|----------|------|------|
| 协议 | OAuth2 | OAuth2 | OAuth2 | OIDC | OAuth2 | OAuth2 |
| 小程序支持 | ✅ code 直换 | ❌ | ❌ | ❌ | ❌ | ❌ |
| UnionID | ✅ 同主体账号打通 | ✅ 同企业打通 | ✅ 同应用打通 | ❌ | ❌ | ❌ |
| 移动端 SDK 调用 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| 邮箱获取 | 部分 | ✅ | ❌ | ✅（可匿名） | ❌ | ❌ |
| 手机号获取 | ✅ 小程序 | ✅ | ❌ | ❌ | ✅ | ✅ |
| 实名认证 | ❌ | ✅ 企业认证 | ✅ | ❌ | ✅ 实名校验 | ❌ |

## 6. UPS 管理模块 (`internal/services/ups/`)

通过 USB 连接管理 UPS 设备，实时监控电力状态，在断电时触发关机流程。

### 技术方案

| 方案 | 说明 | 选型 |
|------|------|------|
| **NUT (Network UPS Tools)** | 成熟的开源 UPS 管理套件，通过 upsd 暴露状态 | ✅ 首选 |
| **USB HID direct** | 直接通过 libusb 读取 HID 报告 | 备选，当 NUT 不可用时 |

NUT 模式：本模块作为 NUT 客户端连接本地或远程 `upsd` 获取状态；Docker 部署时通过挂载 host 的 NUT socket 或网络访问。

### 数据模型

```go
type UPSStatus struct {
    Name             string  `json:"name"`              // UPS 名称
    Model            string  `json:"model"`             // 设备型号
    Status           string  `json:"status"`            // OL(在线) / OB(断电) / LB(低电量) / HB(旁路)
    BatteryCharge    int     `json:"battery_charge"`    // 电量百分比 0-100
    BatteryRuntime   int     `json:"battery_runtime"`   // 剩余时间（秒）
    BatteryVoltage   float64 `json:"battery_voltage"`
    InputVoltage     float64 `json:"input_voltage"`     // 输入电压，0 表示断电
    OutputVoltage    float64 `json:"output_voltage"`
    LoadPercent      float64 `json:"load_percent"`       // 负载百分比
    TimeLeft         string  `json:"time_left"`          // 剩余时间可读格式
}
```

### 事件与自动动作

| UPS 事件 | 触发条件 | 自动动作 |
|----------|----------|----------|
| OnBattery | 输入电压 < 阈值持续 5s | 通知 → 启动关机倒计时 |
| BatteryLow | 电量 ≤ 15% | 通知 → 触发 lanctl 关机 |
| Online | 输入电压恢复持续 30s | 通知 → 取消关机倒计时 |
| Shutdown | 倒计时归零 | 触发本机 + lanctl 关机 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/ups/status` | 获取 UPS 实时状态 |
| `GET` | `/api/v1/ups/history` | 历史事件记录 |
| `POST` | `/api/v1/ups/calibrate` | 手动校准电量 |

### 配置

```yaml
ups:
  driver: "nut"                    # nut | hid
  nut_host: "localhost"
  nut_port: 3493
  nut_username: "monuser"
  nut_password: "secret"
  ups_name: "myups"
  poll_interval: 5s                # 轮询间隔
  events:
    on_battery_delay: 5s           # 断电确认延迟
    battery_low_threshold: 15      # 低电量百分比
    online_stable_delay: 30s       # 恢复确认延迟
    shutdown_timeout: 120s         # 断电后自动关机倒计时
```

## 7. 局域网设备管理模块 (`internal/services/lanctl/`)

管理局域网内设备，在 UPS 断电时按照预定顺序执行优雅关机。

### 设备注册

```go
type LANDevice struct {
    ID          int       `json:"id"`
    Name        string    `json:"name"`         // 设备名（如 nas-01）
    Host        string    `json:"host"`         // IP 或主机名
    Port        int       `json:"port"`         // SSH 端口
    AuthMethod  string    `json:"auth_method"`  // password | key
    Username    string    `json:"username"`
    Password    string    `json:"password,omitempty"` // 加密存储
    KeyPath     string    `json:"key_path,omitempty"`
    ShutdownCmd string    `json:"shutdown_cmd"` // 自定义关机命令
    Timeout     int       `json:"timeout"`      // 单机等待秒数
    Order       int       `json:"order"`        // 关机顺序
    Enabled     bool      `json:"enabled"`
}
```

### 关机策略

```
断电 → UPS OnBattery 事件
  ↓
等待 shutdown_timeout（默认 120s）
  ↓（若未恢复）
查询所有 LANDevice，按 order 排序
  ↓
逐个执行 shutdown_cmd（SSH 或 HTTP）
  ┌──────────────┬──────────────────────┐
  │ SSH 模式      │ HTTP 模式             │
  │ ssh user@host │ POST /api/shutdown   │
  │ shutdown -h now │ {"key": "xxx"}     │
  └──────────────┴──────────────────────┘
  ↓
每个设备等待 Timeout 确认已关机
  ↓
所有设备关机后 → 触发本机关机
```

### 连接方式

| 方式 | 适用场景 | Go 实现 |
|------|----------|--------|
| **SSH** | Linux/Unix 设备（NAS、服务器） | `golang.org/x/crypto/ssh` |
| **HTTP API** | 支持 REST 的智能设备 | `net/http` |
| **WOL** | 唤醒待机设备（仅开机） | `github.com/sabhiram/go-wol` |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/lanctl/devices` | 设备列表 |
| `POST` | `/api/v1/lanctl/devices` | 注册新设备 |
| `PUT` | `/api/v1/lanctl/devices/:id` | 编辑设备 |
| `DELETE` | `/api/v1/lanctl/devices/:id` | 删除设备 |
| `GET` | `/api/v1/lanctl/devices/:id/status` | 设备在线状态 |
| `POST` | `/api/v1/lanctl/shutdown` | 手动触发关机流程 |
| `POST` | `/api/v1/lanctl/shutdown/cancel` | 取消关机流程 |
| `GET` | `/api/v1/lanctl/shutdown/status` | 当前关机流程状态 |
| `POST` | `/api/v1/lanctl/wake/:id` | WOL 唤醒设备 |

### 配置

```yaml
lanctl:
  shutdown_timeout_per_device: 30   # 每台设备等待确认秒数
  concurrent: false                 # 是否并发关机（默认顺序）
  ssh_connect_timeout: 10s
  auto_discovery: false             # 是否启用 mDNS 自动发现
```

### 与 UPS 模块联动

```
ups (OnBattery) ──事件通知──→ lanctl (启动关机倒计时)
                                   │
ups (Online)  ──事件通知──→ lanctl (取消关机倒计时)
                                   │
ups (BatteryLow) ────→ lanctl (立即执行关机)
```

实现方式：`ups` 模块通过 Event Bus 发送事件，`lanctl` 模块订阅后执行。同进程内通过接口直接调用，拆分后通过 asynq / Redis Pub/Sub。

## 8. 定时/周期任务调度模块 (`internal/services/scheduler/`)

管理所有定时执行、周期执行的后台任务，提供统一的创建、取消、执行记录查询接口。

### 技术选型

| 组件 | 用途 |
|------|------|
| **robfig/cron/v3** | 解析 Cron 表达式，管理周期调度器 |
| **asynq** | 延迟任务、分布式任务队列、任务去重 |
| **PostgreSQL** | 任务定义 + 执行历史持久化 |

### 任务模型

```go
type Task struct {
    ID          int64           `json:"id" gorm:"primaryKey"`
    Name        string          `json:"name"`                   // 任务名称，唯一标识
    Type        string          `json:"type"`                   // cron | once | interval
    Expression  string          `json:"expression"`             // cron 表达式 / 间隔
    Handler     string          `json:"handler"`                // 处理器名称
    Payload     json.RawMessage `json:"payload,omitempty"`      // 自定义参数
    Status      string          `json:"status"`                 // active | paused | finished
    MaxRetries  int             `json:"max_retries"`            // 失败重试次数
    Timeout     int             `json:"timeout"`                // 超时秒数
    LastRunAt   *time.Time      `json:"last_run_at"`
    NextRunAt   *time.Time      `json:"next_run_at"`
    CreatedBy   int64           `json:"created_by"`             // 创建者 user_id
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

type TaskLog struct {
    ID        int64          `json:"id" gorm:"primaryKey"`
    TaskID    int64          `json:"task_id"`
    Status    string         `json:"status"`       // running | success | failed | timeout
    StartedAt time.Time      `json:"started_at"`
    EndedAt   *time.Time     `json:"ended_at"`
    Result    json.RawMessage `json:"result,omitempty"`
    Error     string         `json:"error,omitempty"`
    CreatedAt time.Time      `json:"created_at"`
}
```

### 三种任务类型

| 类型 | 说明 | Expression 示例 |
|------|------|----------------|
| `cron` | 按 Cron 表达式周期执行 | `*/5 * * * *`（每5分钟） |
| `once` | 一次性延迟执行（指定时刻） | `2026-07-08T10:00:00+08:00` |
| `interval` | 固定间隔重复执行 | `30m`（每30分钟） / `2h`（每2小时） |

Cron 表达式支持标准 5 位格式，由 `robfig/cron/v3` 解析。

### 处理器注册机制

```go
// 任务处理器接口
type TaskHandler interface {
    Name() string                                          // 处理器唯一标识
    Execute(ctx context.Context, payload json.RawMessage) error  // 执行逻辑
}

// 全局处理器注册
var handlers = make(map[string]TaskHandler)

func RegisterHandler(h TaskHandler) {
    handlers[h.Name()] = h
}
```

#### 内置处理器

| Handler | 说明 | 用途场景 |
|---------|------|----------|
| `health_check` | 各服务健康自检 | 定时检查依赖状态 |
| `cleanup_log` | 清理过期日志 | 每日凌晨清理 30 天前日志 |
| `sync_cert` | 证书续期 | 定期检查 TLS 证书有效期 |
| `ups_log_snapshot` | UPS 状态定时快照 | 每 5 分钟记录 UPS 数据到历史表 |
| `lanctl_wake_schedule` | 定时唤醒设备 | 按计划启动局域网设备 |
| `notify_cleanup` | 清理过期通知 | 清理 90 天前的通知记录 |

业务模块可自行注册 Handler：

```go
// 在 s1 模块的 Init() 中注册
func (m *S1Module) Init(cfg *config.Config) error {
    scheduler.RegisterHandler(&S1ReportGenHandler{})
    return nil
}
```

### 调度引擎工作流

```
服务启动
  │
  ├─ 加载 DB 中 status=active 的任务
  │
  ├─ cron / interval 类型 → 注册到 robfig/cron 调度器
  │     │
  │     └─ 到达触发时间 → 写入 asynq 队列 → Worker 消费执行
  │
  ├─ once 类型 → 计算剩余时间 → 写入 asynq 延迟队列
  │     │
  │     └─ 到期 → Worker 消费执行
  │
  └─ 执行完成 → 记录 TaskLog → 更新 LastRunAt / NextRunAt
```

### 分布式保障

| 问题 | 方案 |
|------|------|
| 重复执行（多副本） | asynq 任务去重 + DB 唯一约束 |
| 任务丢失 | 执行前写 TaskLog(status=running)，原子更新 |
| 超时控制 | context.WithTimeout + asynq 超时配置 |
| 重试机制 | asynq 内置 RetryDelay + MaxRetry |
| 调度器宕机 | 服务重启时重新加载所有 active 任务 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/scheduler/tasks` | 任务列表（支持分页/状态筛选） |
| `POST` | `/api/v1/scheduler/tasks` | 创建任务 |
| `GET` | `/api/v1/scheduler/tasks/:id` | 任务详情 |
| `PUT` | `/api/v1/scheduler/tasks/:id` | 更新任务 |
| `DELETE` | `/api/v1/scheduler/tasks/:id` | 删除任务 |
| `POST` | `/api/v1/scheduler/tasks/:id/pause` | 暂停任务 |
| `POST` | `/api/v1/scheduler/tasks/:id/resume` | 恢复任务 |
| `POST` | `/api/v1/scheduler/tasks/:id/run` | 手动立即执行一次 |
| `GET` | `/api/v1/scheduler/tasks/:id/logs` | 任务执行历史 |
| `GET` | `/api/v1/scheduler/handlers` | 查看已注册的处理器列表 |

### 配置

```yaml
scheduler:
  enabled: true                     # 是否启用调度器
  timezone: "Asia/Shanghai"         # 默认时区
  worker_concurrency: 5             # 同时执行任务数
  default_timeout: 300              # 默认超时秒数
  default_max_retries: 3            # 默认重试次数
  log_retention_days: 90            # 执行日志保留天数
  health_check_cron: "@every 5m"    # 健康检查周期
  cleanup_cron: "0 3 * * *"         # 清理任务执行时间（每天凌晨3点）
```

### 常见内置任务（预置）

| 任务名 | 表达式 | Handler | 说明 |
|--------|--------|---------|------|
| `sys_health_check` | `@every 5m` | `health_check` | 服务健康自检 |
| `sys_log_cleanup` | `0 3 * * *` | `cleanup_log` | 每日凌晨清理过期日志 |
| `ups_status_snapshot` | `@every 5m` | `ups_log_snapshot` | UPS 状态定时快照（依赖 ups 模块） |

## 9. 数据备份模块 (`internal/services/backup/`)

全自动备份数据库、Redis、文件、配置到本地或远端存储。

### 备份类型

| 类型 | 源 | 方式 | 说明 |
|------|----|------|------|
| `postgres` | PostgreSQL | `pg_dump` 管道压缩 | stream + gzip，支持 S3/本地 |
| `mysql` | NAS MySQL | `mysqldump` 管道压缩 | 通过 NAS 连接模块执行 |
| `redis` | Redis | `SAVE` + copy RDB | 先触发持久化再复制文件 |
| `file` | 应用文件 | tar + gzip | 配置目录、数据目录 |
| `config` | 配置 | 复制 `config.yaml` | 含敏感信息加密选项 |

### 工作原理

```
scheduler 触发 → backup.Execute(task)
  │
  ├─ 1. 创建 backup_records 记录 (status=running)
  │
  ├─ 2. 根据 backup_type 选择执行器
  │    ├─ postgres → pg_dump ... | gzip → file
  │    ├─ mysql    → mysqldump ... | gzip → file
  │    ├─ redis    → SAVE → cp dump.rdb → gzip
  │    └─ file     → tar -czf → file
  │
  ├─ 3. 计算 SHA256 checksum
  │
  ├─ 4. 上传到远端存储（可选）
  │    └─ RustFS / NAS 共享目录 / S3 兼容存储
  │
  ├─ 5. 更新备份记录 (status=success)
  │
  └─ 6. 清理过期备份
       └─ 根据保留策略删除旧文件
```

### 数据模型

```go
type BackupPolicy struct {
    ID           int64            `json:"id" gorm:"primaryKey"`
    Name         string           `json:"name"`                     // 策略名称
    BackupType   string           `json:"backup_type"`              // postgres | mysql | redis | file | config
    Schedule     string           `json:"schedule"`                 // cron 表达式，如 "0 2 * * *"
    TargetPath   string           `json:"target_path"`              // 本地存储路径
    RemoteTarget string           `json:"remote_target,omitempty"`  // 远端目标（RustFS bucket / NAS 路径）
    Retention    int              `json:"retention"`                // 保留最近 N 份
    Compression  string           `json:"compression"`              // gzip | zstd | none
    Encrypt      bool             `json:"encrypt"`                  // 是否加密
    Enabled      bool             `json:"enabled"`
    LastBackupAt *time.Time       `json:"last_backup_at"`
    CreatedAt    time.Time        `json:"created_at"`
}
```

### 按需恢复 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/backup/policies` | 备份策略列表 |
| `POST` | `/api/v1/backup/policies` | 创建备份策略 |
| `PUT` | `/api/v1/backup/policies/:id` | 编辑策略 |
| `DELETE` | `/api/v1/backup/policies/:id` | 删除策略 |
| `POST` | `/api/v1/backup/policies/:id/run` | 手动执行一次备份 |
| `GET` | `/api/v1/backup/records` | 备份记录列表（分页） |
| `GET` | `/api/v1/backup/records/:id` | 备份详情 |
| `DELETE` | `/api/v1/backup/records/:id` | 删除备份文件 |
| `POST` | `/api/v1/backup/restore` | 从备份记录恢复 |

### 恢复流程

```
POST /api/v1/backup/restore  { record_id, target_type }
  │
  ├─ postgres → pg_restore 或 psql < 文件
  ├─ mysql    → mysql < 文件
  ├─ redis    → 停止 → 替换 RDB → 重启
  └─ file     → tar -xzf 到指定目录
```

恢复操作为危险操作，要求双重确认：
1. 请求必须来自管理员 JWT
2. 需额外传入 `confirm_token`（通过确认接口预生成）

### 配置

```yaml
backup:
  storage_path: "/data/backups"       # 本地备份存储根目录
  temp_path: "/tmp/backups"           # 临时工作目录
  remote:
    rustfs:
      bucket: "backups"
      prefix: "db/"
    nas:
      path: "//nas/backup/share"
  encryption_key_path: "/run/secrets/backup_key"  # 加密密钥文件
  default_retention: 30               # 默认保留 30 份
  restore_confirm_ttl: 5m             # 恢复确认码有效期
  executor:
    pg_dump: "pg_dump"                # pg_dump 命令路径
    mysqldump: "mysqldump"            # mysqldump 命令路径
    redis_cli: "redis-cli"            # redis-cli 命令路径
```

备份策略的调度自动注册到 `scheduler` 模块：

```go
func (b *BackupModule) Init(cfg *config.Config) error {
    policies := loadPolicies(cfg)
    for _, p := range policies {
        if p.Enabled {
            scheduler.RegisterTask(scheduler.Task{
                Name:       "backup_" + p.Name,
                Expression: p.Schedule,
                Handler:    "backup_executor",
                Payload:    json.Marshal(p),
            })
        }
    }
    return nil
}
```

## 10. 数据库迁移管理模块 (`internal/services/migration/`)

基于 `golang-migrate/migrate` 的版本化迁移管理，提供 HTTP API 执行、查看、回滚迁移。

### 与 GORM AutoMigrate 的定位

| 方式 | 适用环境 | 用途 |
|------|----------|------|
| GORM AutoMigrate | 开发/测试 | 自动同步模型，快速原型 |
| golang-migrate | 生产/预发布 | 版本化 SQL，可回滚，可 Review |

### 功能

| 功能 | 说明 |
|------|------|
| 查看迁移版本 | 当前数据库所处迁移版本号 |
| 查看迁移列表 | 所有迁移文件及其状态（已执行/待执行） |
| 执行迁移 | 正向迁移到指定版本或最新版 |
| 回滚迁移 | 回退到指定版本 |
| 强制设置版本 | 应急修复 dirty 状态 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/migration/status` | 当前迁移版本 + dirty 状态 |
| `GET` | `/api/v1/migration/list` | 所有迁移文件及执行状态 |
| `POST` | `/api/v1/migration/up` | 执行到最新版本 |
| `POST` | `/api/v1/migration/up/:steps` | 执行 N 步 |
| `POST` | `/api/v1/migration/down` | 回滚 1 步 |
| `POST` | `/api/v1/migration/down/:steps` | 回滚 N 步 |
| `POST` | `/api/v1/migration/to/:version` | 迁移到指定版本 |
| `POST` | `/api/v1/migration/force/:version` | 强制设置版本（修复 dirty） |

### 配置

```yaml
migration:
  source: "file://migrations/postgres"  # 迁移文件路径
  database_url: "${DATABASE_URL}"       # 数据库连接
  table_name: "schema_migrations"       # 版本记录表名
```

### 迁移文件示例

```sql
-- migrations/postgres/000001_create_users_table.up.sql
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- migrations/postgres/000001_create_users_table.down.sql
DROP TABLE IF EXISTS users;
```

### 安全规则

- 迁移文件一旦发布（合入主分支），禁止修改内容
- 回滚必须编写对应的 `down.sql` 文件
- 生产环境迁移前自动执行 `dry-run` 模式预览影响
- `force` 操作用于修复 dirty 状态，需管理员权限

## 11. 节点发现与 P2P 连接模块 (`internal/services/discovery/`)

管理多节点间的自动发现、健康监测和点对点加密通信。支持多副本部署时节点自动组网。

### 技术方案

| 组件 | 技术 | 说明 |
|------|------|------|
| **节点发现** | **mDNS**（局域网）+ **Redis Registry**（广域网） | 局域网零配置发现，广域网通过注册中心 |
| **P2P 传输** | **libp2p** | 去中心化、NAT 穿透、加密传输、多路复用 |
| **健康监测** | 自定义心跳 + Gossip | 节点状态实时感知 |
| **数据同步** | **gRPC stream** + **PubSub** | 状态同步 / 事件广播 |

### 部署拓扑

```
┌───────────────────────────┐          ┌───────────────────────────┐
│   节点 A (192.168.1.10)    │          │   节点 B (192.168.1.11)    │
│                           │  mDNS    │                           │
│  discovery.Register()     │◄────────►│  discovery.Discover()     │
│  ┌─────────────────────┐  │ libp2p   │  ┌─────────────────────┐  │
│  │ Peer ID: QmX...      │  │◄────────►│  │ Peer ID: QmY...      │  │
│  │ addrs: /ip4/.../tcp  │  │ gRPC     │  │ addrs: /ip4/.../tcp  │  │
│  │ uptime: 3h           │  │ stream   │  │ uptime: 3h           │  │
│  │ load: 0.45           │  │          │  │ load: 0.23           │  │
│  │ roles: [scheduler]   │  │          │  │ roles: [backup,nas]  │  │
│  └─────────────────────┘  │          │  └─────────────────────┘  │
└───────────────────────────┘          └───────────────────────────┘
         │                                     │
         │         Redis Registry              │
         └──────────────────┬──────────────────┘
                            ▼
                  ┌──────────────────┐
                  │   Redis          │
                  │   discovery:nodes│
                  │   A: {peer,ttl}  │
                  │   B: {peer,ttl}  │
                  └──────────────────┘
```

### 发现方式

| 方式 | 网络 | 配置 | 说明 |
|------|------|------|------|
| **mDNS** | 局域网 | 零配置 | 自动发现同网段节点，适合 NAS 内网环境 |
| **Redis Registry** | 广域网 | 需 Redis 地址 | 节点注册到 Redis，定期心跳续期 |
| **Static** | 任何网络 | 手动配置节点地址列表 | 兜底方案，指定已知节点 |

### 节点模型

```go
type NodeInfo struct {
    ID          string            `json:"id"`           // 节点 UUID
    Name        string            `json:"name"`         // 自定义节点名
    PeerID      string            `json:"peer_id"`      // libp2p Peer ID
    Addrs       []string          `json:"addrs"`        // 监听地址
    Version     string            `json:"version"`      // 软件版本
    Uptime      int64             `json:"uptime"`       // 运行时长（秒）
    Load        float64           `json:"load"`         // 负载指标
    Roles       []string          `json:"roles"`        // 节点角色
    Status      string            `json:"status"`       // online | offline | unhealthy
    LastSeen    time.Time         `json:"last_seen"`    // 最后心跳时间
    Labels      map[string]string `json:"labels"`       // 自定义标签
    Metadata    json.RawMessage   `json:"metadata"`     // 自定义元数据
}
```

### 心跳与健康检测

```
节点启动
  │
  ├─ 1. 生成 PeerID，启动 libp2p host
  │
  ├─ 2. 注册到 Redis (TTL=30s)
  │    HSET discovery:nodes:{id} {info}
  │    EXPIRE discovery:nodes:{id} 30
  │
  ├─ 3. 每 10s 发送心跳
  │    └─ 刷新 Redis TTL
  │    └─ 广播 Gossip 消息到已连接节点
  │
  ├─ 4. 每 30s 检查节点列表
  │    └─ 标记超过 60s 未心跳的节点为 offline
  │
  └─ 5. 关闭时发送下线通知
       └─ 从 Redis 移除
       └─ 广播离开消息
```

### P2P 通信

P2P 通道用于节点间直接交换数据，不经过中心服务器。

| 场景 | 通信方式 | 协议 |
|------|----------|------|
| 节点状态同步 | Gossip PubSub | libp2p PubSub |
| 任务分发 | Direct Stream | libp2p Stream (protobuf) |
| 文件传输 | Direct Stream | libp2p Stream (chunked) |
| 事件广播 | Gossip PubSub | libp2p PubSub |

```go
// 注册 P2P 协议处理器
host.SetStreamHandler("/sync/v1", func(s libp2p.Stream) {
    // 处理节点间数据同步
})

host.SetStreamHandler("/task/v1", func(s libp2p.Stream) {
    // 处理分布式任务
})
```

### 使用场景

| 场景 | 说明 |
|------|------|
| **调度器高可用** | 多个调度器节点互为主备，任务不重复执行 |
| **UPS 状态共享** | 主节点 UPS 断电时，通知其他节点准备关机 |
| **备份协同** | 备份任务分发到不同节点并行执行 |
| **配置同步** | 主节点配置变更后自动同步到所有节点 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/discovery/nodes` | 节点列表（含状态） |
| `GET` | `/api/v1/discovery/nodes/:id` | 节点详情 |
| `GET` | `/api/v1/discovery/peers` | 已建立 P2P 连接的节点 |
| `POST` | `/api/v1/discovery/connect` | 手动连接指定节点 |
| `POST` | `/api/v1/discovery/disconnect/:id` | 断开与指定节点的 P2P 连接 |
| `GET` | `/api/v1/discovery/self` | 本节点信息 |

### 配置

```yaml
discovery:
  enabled: true
  node_name: "node-1"                   # 本节点名称
  listen_port: 29091                     # P2P 监听端口（HTTP 端口+1）
  enable_mdns: true                      # 局域网 mDNS 发现
  enable_redis_registry: true            # Redis 注册中心
  redis_registry_ttl: 30s                # 心跳 TTL
  heartbeat_interval: 10s                # 心跳间隔
  cleanup_interval: 30s                  # 离线节点清理间隔
  offline_threshold: 60s                 # 判定离线阈值
  roles:
    - scheduler
    - backup
    - ups
  static_peers:                          # 静态节点列表（兜底）
    - "/ip4/192.168.1.20/tcp/29091/p2p/QmX..."
  labels:
    region: "cn-shanghai"
    rack: "rack-01"
```

### 安全

- 所有 P2P 通信默认 TLS 加密（libp2p 内置）
- 节点加入需验证 `shared_secret` 或 TLS 证书
- 可选的节点白名单

```yaml
discovery:
  psk: "aes-256-shared-secret-key"       # 预共享密钥（可选）
  allowlist: ["QmX...", "QmY..."]         # 允许连接的 PeerID 白名单
```

## 12. 加密解密与压缩解压缩模块 (`internal/services/crypto/` + `pkg/crypto/`)

提供统一的加解密、哈希、压缩解压缩能力，供公共服务和业务模块内部调用，同时暴露 HTTP API 供客户端使用。

### 架构定位

```
client ──→ crypto HTTP API ──→ pkg/crypto（核心实现）
                                   │
                        ┌──────────┼──────────┐
                        ▼          ▼          ▼
                     backup     file       auth / 其他模块
                     (加密备份)  (文件加密)   (密码哈希)
```

核心库 `pkg/crypto/` 被所有模块直接引用，`internal/services/crypto/` 是对外 HTTP 接口层。

### 功能矩阵

| 类别 | 功能 | 算法/格式 | 调用方 |
|------|------|-----------|--------|
| **对称加密** | 数据加密/解密 | AES-256-GCM, ChaCha20-Poly1305 | backup, file, crypto API |
| **非对称加密** | 密钥加密/签名 | RSA-OAEP, ECDSA P-256 | auth, discovery |
| **密码哈希** | 密码存储验证 | Argon2id (慢哈希) | auth |
| **内容哈希** | 完整性校验 | SHA-256, SHA-512, BLAKE3 | backup, file, 通用 |
| **密钥派生** | 密钥派生 | HKDF, PBKDF2 | crypto API |
| **压缩** | 数据压缩/解压 | zstd, gzip, brotli, snappy | backup, file, crypto API |
| **文件加密** | 大文件流式加密 | AES-256-GCM + zstd 复合 | backup, file |

### 核心接口

```go
package crypto

// ——— 对称加密 ———

// Encrypt 加密数据，返回格式: nonce(12B) + ciphertext
func Encrypt(plaintext []byte, key *[32]byte) ([]byte, error)

// Decrypt 解密数据
func Decrypt(ciphertext []byte, key *[32]byte) ([]byte, error)

// EncryptFile 流式加密文件（适合大文件）
func EncryptFile(src, dst string, key *[32]byte) error

// DecryptFile 流式解密文件
func DecryptFile(src, dst string, key *[32]byte) error

// ——— 非对称加密 ———

// GenerateKeyPair 生成 RSA/ECDSA 密钥对
func GenerateKeyPair(bits int) (*PrivateKey, *PublicKey, error)

// EncryptRSA 使用公钥加密
func EncryptRSA(plaintext []byte, pub *PublicKey) ([]byte, error)

// DecryptRSA 使用私钥解密
func DecryptRSA(ciphertext []byte, priv *PrivateKey) ([]byte, error)

// ——— 密码哈希 ———

// HashPassword Argon2id 哈希密码
func HashPassword(password string) (string, error)

// VerifyPassword 验证密码
func VerifyPassword(password, hash string) bool

// ——— 压缩 ———

// Compress 压缩数据（自动选择最优算法）
func Compress(data []byte, algo Algorithm) ([]byte, error)

// Decompress 解压数据
func Decompress(data []byte) ([]byte, error)

// ——— 复合操作（加密 + 压缩） ———

// EncryptAndCompress 先压缩再加密
func EncryptAndCompress(data []byte, key *[32]byte) ([]byte, error)

// DecryptAndDecompress 先解密再解压
func DecryptAndDecompress(data []byte, key *[32]byte) ([]byte, error)
```

### Key 管理

```go
type KeyManager struct {
    // 支持的密钥来源
    // 1. 配置文件路径
    // 2. 环境变量
    // 3. Docker Secret (/run/secrets/*)
    // 4. Vault / KMS（预留）
}

// GetKey 按名称获取密钥
func (km *KeyManager) GetKey(name string) (*[32]byte, error)

// RotateKey 轮换密钥（保留旧密钥用于解密旧数据）
func (km *KeyManager) RotateKey(name string) error
```

### 密钥层级

```
主密钥 (Master Key)
  └── 加密密钥 (Encryption Keys)
       ├── backup.key       # 备份加密
       ├── file.key         # 文件加密
       └── config.key       # 配置文件敏感字段加密
```

密钥来源（按优先级）：
1. `Docker Secret`: `/run/secrets/{name}`
2. 环境变量: `CRYPTO_KEY_{NAME}`
3. 配置文件路径: 指向外部密钥文件

### 算法选择指南

| 场景 | 推荐算法 | 说明 |
|------|----------|------|
| 通用数据加密 | AES-256-GCM | 硬件加速，性能优异 |
| 移动端/低性能设备 | ChaCha20-Poly1305 | 无硬件加速时更快 |
| 文件加密 | AES-256-GCM + zstd | 先压缩后加密 |
| 密码存储 | Argon2id | 抗 GPU/ASIC 暴力破解 |
| 短期敏感数据 | XSalsa20-Poly1305 | 支持随机访问加密 |
| 数据完整性 | SHA-256 / BLAKE3 | BLAKE3 性能优于 SHA-256 |
| 日志压缩 | zstd / snappy | zstd 压缩率高，snappy 速度快 |
| Web 内容 | brotli / gzip | 浏览器广泛支持 |
| 密钥交换 | X25519 | 现代椭圆曲线 DH |

### 与 Backup 模块的集成

`crypto` 模块被 `backup` 模块依赖用于加密备份文件：

```go
// backup/executor.go
func (e *PGExecutor) Execute(ctx context.Context, policy BackupPolicy) error {
    dump := pgDump(ctx)
    compressed, _ := crypto.Compress(dump, crypto.Zstd)
    encrypted, _ := crypto.Encrypt(compressed, keyManager.GetKey("backup"))
    return saveToStorage(encrypted)
}
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/crypto/encrypt` | 加密数据 |
| `POST` | `/api/v1/crypto/decrypt` | 解密数据 |
| `POST` | `/api/v1/crypto/compress` | 压缩数据 |
| `POST` | `/api/v1/crypto/decompress` | 解压数据 |
| `POST` | `/api/v1/crypto/hash` | 计算哈希 (SHA-256/SHA-512/BLAKE3) |
| `POST` | `/api/v1/crypto/keys` | 列出可用密钥名称 |
| `POST` | `/api/v1/crypto/keygen` | 生成新密钥 |

所有 API 操作记录审计日志，密钥管理类操作要求管理员权限。

### 配置

```yaml
crypto:
  default_symmetric: "aes-256-gcm"       # 默认对称加密算法
  default_compression: "zstd"            # 默认压缩算法
  compression_level: 3                   # zstd 压缩级别 (1-22)
  keys:
    backup:
      source: "docker_secret"            # docker_secret | env | file
      path: "/run/secrets/crypto_backup_key"
    file:
      source: "env"
      path: "CRYPTO_KEY_FILE"
    config:
      source: "file"
      path: "/etc/secrets/config_key.bin"
```

## 13. AI 推理服务模块 (`internal/services/ai/`)

统一接入多种 AI 推理后端，为业务模块提供文本生成、向量嵌入、对话等 AI 能力。

### 支持的推理后端

| 后端 | 连接方式 | 适用场景 |
|------|----------|----------|
| **Ollama** | HTTP API (`ollama/api`) | 本地私有化 LLM，部署在 NAS 或 GPU 服务器 |
| **Direct GPU** | gRPC (Triton / vLLM) | 高性能自研模型，直连 GPU 推理服务 |
| **OpenAI API** | HTTP | GPT-4o 等云端模型 |
| **One API** | HTTP (OpenAI 兼容) | 聚合多供应商，统一管理 API Key 与配额 |

### 统一 Provider 接口

```go
type AIProvider interface {
    Name() string
    Models(ctx) ([]Model, error)
    Chat(ctx, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx, req ChatRequest) (<-chan Token, error)
    Embed(ctx, req EmbedRequest) (*EmbedResponse, error)
    Close() error
}
```

### 路由策略

| 策略 | 说明 |
|------|------|
| 直接指定 | 客户端指定 model，路由到对应 Provider |
| 模型映射 | client 请求 `gpt-4o` → 实际路由到 `qwen2:72b` |
| Fallback | 主模型失败 → 降级备选模型 |
| 加权轮询 | 多个同能力模型按权重分发 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/ai/models` | 可用模型列表 |
| `POST` | `/api/v1/ai/chat` | 对话补全（支持 stream SSE） |
| `POST` | `/api/v1/ai/embeddings` | 向量嵌入 |
| `GET` | `/api/v1/ai/providers` | Provider 状态 |

## 14. 统一数据访问模块 (`internal/database/`)

统一管理所有数据存储的连接池、健康检查和指标采集。各业务模块通过此模块访问数据库，不直接创建连接。

### 架构定位

```
业务模块 (services / modules)
       │
       ▼
┌──────────────────────────────────────────┐
│          database (统一数据访问层)           │
│  ┌────────┐ ┌────────┐ ┌──────┐ ┌─────┐  │
│  │Postgres│ │ MySQL  │ │ Redis│ │ S3  │  │
│  └────────┘ └────────┘ └──────┘ └─────┘  │
└──────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────┐
│  NAS / Cloud (环境配置上下文)               │
└──────────────────────────────────────────┘
```

`internal/database/` 提供核心连接管理；`internal/nas/` 和 `internal/cloud/` 引用 database 模块，传入特定环境的配置。

### 支持的存储类型

| 类型 | 库 | 用途 | 配置别名 |
|------|----|------|----------|
| **PostgreSQL** | `pgx` / GORM | 主业务数据库 + NAS 实例 | `postgres.main`, `postgres.nas` |
| **MySQL** | `go-sql-driver/mysql` | NAS 实例 | `mysql.nas` |
| **Redis** | `go-redis/v9` | 缓存/会话/队列/PubSub | `redis.cache`, `redis.nas` |
| **MinIO** | `minio-go/v7` | S3 兼容对象存储 | `minio.default` |
| **RustFS** | `minio-go/v7` (S3 兼容) | 自建对象存储 | `rustfs.default` |

### 管理器

```go
type DBManager struct {
    Postgres map[string]*pgxpool.Pool   // 多实例: main, nas1, nas2...
    MySQL    map[string]*sql.DB
    Redis    map[string]*redis.Client
    S3       map[string]*minio.Client     // MinIO 和 RustFS 统一用 S3 客户端
}

// 全局唯一实例
var DB *DBManager

// InitAll 根据配置初始化所有连接
func InitAll(cfg *config.Config) error

// CloseAll 优雅关闭所有连接
func CloseAll()

// Health 聚合所有连接的健康状态
func Health() map[string]HealthStatus
```

### 健康检查

```go
func Health() map[string]HealthStatus {
    result := make(map[string]HealthStatus)
    for name, pool := range DB.Postgres {
        result["postgres:"+name] = ping(pool)
    }
    for name, client := range DB.Redis {
        result["redis:"+name] = ping(client)
    }
    // ... MySQL, S3
    return result
}
```

暴露为 `GET /api/v1/s0/health` 的一部分，同时提供独立的 `/api/v1/database/health` 端点。

### 配置

```yaml
database:
  postgres:
    main:                              # 主数据库
      dsn: "postgres://user:pass@postgres:5432/app"
      max_open: 50
      max_idle: 10
      conn_max_lifetime: 30m
    nas:                               # NAS 上的 PostgreSQL
      dsn: "postgres://user:pass@nas.local:5432/db"
      max_open: 10

  mysql:
    nas:
      dsn: "user:pass@tcp(nas.local:3306)/db"
      max_open: 10

  redis:
    cache:                             # 主缓存
      addr: "redis:6379"
      db: 0
    nas:                               # NAS 上的 Redis
      addr: "nas.local:6379"
      db: 0

  s3:
    minio:
      endpoint: "minio:9000"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"
      use_ssl: false
      bucket: "app-data"
    rustfs:
      endpoint: "rustfs.example.com"
      access_key: "${RUSTFS_ACCESS_KEY}"
      secret_key: "${RUSTFS_SECRET_KEY}"
      use_ssl: true
      bucket: "app-files"
```

### 与 NAS / Cloud 的关系

```go
// internal/nas/  - 使用 database 模块 + NAS 特定配置
func NewNASClient(cfg *config.NASConfig) *NASClient {
    return &NASClient{
        Postgres: database.DB.Postgres["nas"],  // 引用 database 中的实例
        MySQL:    database.DB.MySQL["nas"],
        Redis:    database.DB.Redis["nas"],
    }
}
```

```go
// internal/cloud/ - 使用 database 模块 + 云服务特定配置
func NewCloudClient(cfg *config.CloudConfig) *CloudClient {
    return &CloudClient{
        RustFS: database.DB.S3["rustfs"],
        MinIO:  database.DB.S3["minio"],
    }
}
```

### 指标

每个连接池自动暴露 Prometheus 指标：

| 指标 | 类型 | 标签 |
|------|------|------|
| `db_connections_open` | Gauge | `type`, `name` |
| `db_connections_max` | Gauge | `type`, `name` |
| `db_connection_errors_total` | Counter | `type`, `name` |
| `db_query_duration_seconds` | Histogram | `type`, `name`, `op` |
| `db_health_status` | Gauge | `type`, `name` (1=健康, 0=异常) |

```
┌─────────┐     gRPC/Event     ┌─────────┐
│  Module  │ ◄──────────────► │  Module  │
│    S1    │                   │    S2    │
└─────────┘                   └─────────┘
       │                           │
       │        HTTP API           │
       ▼                           ▼
┌──────────────────────────────────────────────────────────────┐
│         公共服务层                                              │
│   Auth / User / Notify / File / UPS / LAN Ctl / Scheduler   │
│   Backup / Migration / Discovery / Crypto / AI / Logger     │
│   Monitor / Config / Webhook                                    │
├──────────────────────────────────────────────────────────────┤
│         基础设施层 (数据访问)                                   │
│   PostgreSQL / MySQL / Redis / MinIO / RustFS                │
└──────────────────────────────────────────────────────────────┘
```

## 15. 日志管理模块 (`internal/services/logger/`)

集中管理所有日志输出、存储、查询与审计。基于 zerolog（已选型）构建统一日志管道。

### 日志分层

```
┌─────────────┐
│  业务日志    │ ← 各模块通过 Logger 接口写入
├─────────────┤
│  访问日志    │ ← Gin middleware 自动记录
├─────────────┤
│  系统日志    │ ← 服务启动/关闭/错误
├─────────────┤
│  审计日志    │ ← 敏感操作（管理员、密钥、权限变更）
└──────┬──────┘
       │
       ▼
┌──────────────────────┐
│    Log Processor     │ ← 格式化、过滤、缓冲
├──────────────────────┤
│    Log Exporter      │ ← 输出到多个目标
└──────────────────────┘
       │
       ├── stdout (Docker 容器日志，默认)
       ├── 文件 (日志轮转)
       └── Loki (独立 Docker Compose，后续配置)
```

### Logger 接口

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    Fatal(msg string, err error, fields ...Field)
    Audit(action string, userID int64, fields ...Field) // 审计日志专用
    With(fields ...Field) Logger                         // 带上下文的派生 Logger
}
```

### 审计日志

记录所有敏感操作，方便追溯。

| 审计事件类型 | 示例 |
|-------------|------|
| `user.login` | 用户登录 |
| `user.password_change` | 修改密码 |
| `admin.*` | 管理员操作（创建用户、修改权限） |
| `config.*` | 配置变更 |
| `crypto.key_rotate` | 密钥轮换 |
| `backup.restore` | 数据恢复 |

审计日志同时写入 PostgreSQL `audit_logs` 表和日志流。

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/logger/level` | 获取当前日志级别 |
| `PUT` | `/api/v1/logger/level` | 动态调整日志级别（debug/info/warn/error） |
| `GET` | `/api/v1/logger/search` | 搜索日志（时间范围+级别+关键字） |
| `GET` | `/api/v1/logger/audit` | 审计日志查询（管理员） |
| `GET` | `/api/v1/logger/stats` | 日志量统计 |

### 配置

```yaml
logger:
  level: "info"                        # debug | info | warn | error
  format: "json"                       # json | text
  output:
    - type: "stdout"                   # 容器标准输出
    - type: "file"                     # 文件输出
      path: "/var/log/app"
      max_size: 100                    # MB
      max_age: 30                      # 天
      max_backups: 10
    - type: "loki"                     # Loki（可选，需部署 Loki）
      url: "http://loki:3100/loki/api/v1/push"
      labels:
        app: "go-backend-core"
        env: "production"
  audit:
    enabled: true
    retention_days: 365                # 审计日志保留 1 年
```

### 部署说明

- **stdout**: Docker 容器默认，始终启用
- **文件**: 挂载 volume `/var/log/app` 持久化
- **Loki**: 独立 Docker Compose，本模块仅推送；由 Grafana 统一查询

## 16. 监控模块 (`internal/services/monitor/`)

聚合所有健康检查、性能指标，暴露 Prometheus 端点，统一管理告警规则。

### 架构定位

```
┌──────────────────────────────┐
│       Go Backend             │
│  ┌────────────────────────┐  │
│  │  monitor.Service       │  │
│  │  ├─ Health aggregator  │  │
│  │  ├─ /metrics endpoint │  │  ← Prometheus 拉取
│  │  └─ Alert manager     │  │
│  └────────────────────────┘  │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│   独立 Docker Compose          │
│  Prometheus ←→ Grafana       │
│  (告警 → Webhook → 通知)     │
└──────────────────────────────┘
```

### 健康检查聚合

```go
type HealthChecker interface {
    Name() string
    Check(ctx) HealthResult
}

type HealthResult struct {
    Status   string            `json:"status"`   // ok | degraded | down
    Latency  int64             `json:"latency_ms"`
    Message  string            `json:"message,omitempty"`
    Details  map[string]any    `json:"details,omitempty"`
}
```

所有模块注册健康检查到 monitor：

```go
// 各模块 Init 中注册
monitor.RegisterHealthCheck(&database.HealthChecker{})
monitor.RegisterHealthCheck(&ups.HealthChecker{})
monitor.RegisterHealthCheck(&discovery.HealthChecker{})
```

聚合结果暴露为 `GET /api/v1/monitor/health`：

```json
{
  "status": "degraded",
  "uptime": "72h30m",
  "checks": {
    "postgres:main":   { "status": "ok",  "latency_ms": 2 },
    "postgres:nas":    { "status": "ok",  "latency_ms": 15 },
    "redis:cache":     { "status": "ok",  "latency_ms": 1 },
    "redis:nas":       { "status": "down","latency_ms": 0, "message": "connection refused" },
    "ups:main":        { "status": "ok",  "latency_ms": 50 }
  }
}
```

### Prometheus 指标

`/metrics` 端点（Prometheus 拉取方式），注册自定义指标：

| 指标 | 类型 | 说明 |
|------|------|------|
| `go_*` | 内置 | Go 运行时指标 |
| `http_requests_total` | Counter | 请求总数（按 method/path/status） |
| `http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `db_connections_open` | Gauge | 数据库连接数 |
| `db_query_duration_seconds` | Histogram | 查询延迟 |
| `ups_battery_charge` | Gauge | UPS 剩余电量 |
| `node_health_status` | Gauge | 各节点健康状态 |

### 告警定义（配置驱动）

```yaml
monitor:
  health_check_interval: 30s
  alert:
    webhook: "http://alertmanager:9093/api/v1/alerts"
    rules:
      - name: "database_down"
        condition: "health.postgres:main.status == down"
        duration: "30s"
        severity: "critical"
        summary: "主数据库不可达"
      - name: "ups_on_battery"
        condition: "health.ups:main.status == degraded && ups.battery_charge < 20"
        duration: "60s"
        severity: "warning"
        summary: "UPS 电量低于 20%"
      - name: "high_latency"
        condition: "http_request_duration_seconds.p99 > 5"
        duration: "5m"
        severity: "warning"
```

告警规则由 monitor 模块本地评估，满足条件时推送到 Prometheus Alertmanager Webhook。

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/monitor/health` | 全服务健康状态聚合 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/v1/monitor/status` | 模块运行状态（各模块启动/运行时间） |
| `GET` | `/api/v1/monitor/alerts` | 当前活跃告警列表 |
| `GET` | `/api/v1/monitor/dashboard` | 关键指标摘要（给前端展示） |

### 部署说明

- **本 Docker Compose**: 仅含 Go 后端（暴露 `/metrics`）
- **独立 Docker Compose**: Prometheus 配置 scrape 本服务 `/metrics`，Grafana 连接 Prometheus 做可视化
- 用户手动在 Prometheus 配置中添加本服务 target

## 17. 配置信息管理模块 (`internal/services/config/`)

集中管理所有配置的读取、热加载、版本查询和敏感字段加解密。

### 功能

| 功能 | 说明 |
|------|------|
| **配置读取** | Viper 加载 config.yaml + 环境变量覆盖 |
| **热加载** | 监听配置文件变更，自动重载 |
| **配置查询** | 通过 API 查看当前生效配置（管理员） |
| **配置校验** | 启动时校验配置完整性 |
| **敏感加密** | 配置中的密码/密钥字段自动加解密（调用 crypto 模块） |
| **版本追踪** | 记录配置变更历史 |

### 配置加载优先级

```
1. Docker Secret (/run/secrets/*)      ← 最高优先级
2. 环境变量 (APP_*)                     ← 次高
3. config.yaml                         ← 默认
4. 内置默认值                           ← 最低
```

### 配置热加载

```go
// Viper 监听文件变更
viper.WatchConfig()
viper.OnConfigChange(func(e fsnotify.Event) {
    // 1. 校验新配置
    // 2. 通知各模块 Reload(cfg)
    // 3. 记录配置变更日志
    eventBus.Publish("config:changed", newCfg)
})
```

各模块实现 `Reloader` 接口实现热加载：

```go
type Reloader interface {
    Reload(cfg *config.Config) error
}

// 模块注册到 config 服务
config.RegisterReloader("database", &db.Reloader{})
config.RegisterReloader("ups", &ups.Reloader{})
```

### 敏感字段加密

```yaml
# config.yaml 中敏感字段存储为加密值
database:
  postgres:
    main:
      dsn: "enc:aes-gcm:base64:xxxxx..."  # enc:算法:编码:密文
```

config 模块启动时用 crypto 模块解密，内存中保持明文，日志中自动脱敏。

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/config` | 当前配置（管理员，敏感字段脱敏） |
| `GET` | `/api/v1/config/:key` | 按路径查询特定配置 |
| `POST` | `/api/v1/config/reload` | 手动触发配置重载 |
| `GET` | `/api/v1/config/history` | 配置变更历史 |
| `GET` | `/api/v1/config/schema` | 配置结构定义文档 |
| `GET` | `/api/v1/config/validate` | 验证当前配置完整性 |

### 配置

```yaml
config:
  watch: true                            # 启用热加载
  watch_interval: 5s                     # 文件监听间隔
  sensitive_keys:                        # 日志中自动脱敏的字段路径
    - "database.postgres.main.dsn"
    - "auth.jwt.secret"
    - "ai.providers.openai.api_key"
  encrypt_fields:                        # 启动时自动加密这些字段
    - "database.postgres.main.dsn"
  history:
    enabled: true
    storage: "postgres"                  # 变更历史存储位置
    max_records: 1000
```

### 部署说明

- 配置文件 `config.yaml` 通过 volume 挂载到容器
- 敏感字段可在部署前手动加密，或在首次启动时由 config 模块自动加密
- 配置变更通过 `POST /api/v1/config/reload` 触发，无需重启容器

## 18. Webhook 模块 (`internal/services/webhook/`)

管理出站 Webhook（系统向外部发送事件通知）和入站 Webhook（接收外部系统回调），支持签名验证、重试、模板化。

### 功能全景

```
┌─────────────────────────────────────────────────────────┐
│                    Webhook 模块                          │
│                                                         │
│  ┌──────────────┐     ┌──────────────┐                  │
│  │  出站 Webhook │     │  入站 Webhook │                  │
│  │  (Outgoing)  │     │  (Incoming)  │                  │
│  └──────┬───────┘     └──────┬───────┘                  │
│         │                    │                          │
│         ▼                    ▼                          │
│  ┌────────────────────────────────────┐                 │
│  │        Webhook Engine              │                 │
│  │  调度  ── 签名  ── 发送  ── 重试   │                 │
│  └────────────────────────────────────┘                 │
└─────────────────────────────────────────────────────────┘
       │                               │
       ▼                               ▼
  外部服务 (Slack/企业微信/…)      GitHub/GitLab/Stripe/…
```

### 出站 Webhook

系统内部事件发生时，向外部服务发送 HTTP 请求。

#### 数据模型

```go
type OutgoingWebhook struct {
    ID          int64             `json:"id" gorm:"primaryKey"`
    Name        string            `json:"name"`          // 名称（如 "告警推送-企业微信"）
    URL         string            `json:"url"`           // 目标 URL
    Secret      string            `json:"secret,omitempty"` // 签名密钥（加密存储）
    Events      []string          `json:"events"`        // 订阅事件列表
    Format      string            `json:"format"`        // json | form
    Template    string            `json:"template"`      // Go template 字符串
    Headers     map[string]string `json:"headers"`       // 自定义请求头
    RetryPolicy RetryPolicy       `json:"retry_policy"`
    Timeout     int               `json:"timeout"`       // 超时秒数
    Status      string            `json:"status"`        // active | paused
    LastSentAt  *time.Time        `json:"last_sent_at"`
    CreatedAt   time.Time         `json:"created_at"`
}
```

#### 事件订阅与发送

```go
// 事件总线 → Webhook 模块自动订阅
eventBus.Subscribe("backup:completed", func(evt Event) {
    webhook.Dispatch("backup:completed", evt.Payload)
})

eventBus.Subscribe("ups:on_battery", func(evt Event) {
    webhook.Dispatch("ups:on_battery", evt.Payload)
})

eventBus.Subscribe("monitor:alert", func(evt Event) {
    webhook.Dispatch("monitor:alert", evt.Payload)
})
```

发送流程：

```
事件触发 → 匹配订阅事件的 Webhook
  │
  ├─ 1. 用 Go template 渲染消息体
  ├─ 2. 计算签名 (HMAC-SHA256)
  ├─ 3. HTTP POST 到目标 URL
  ├─ 4. 记录发送结果 (成功/失败/状态码)
  └─ 5. 失败 → 按重试策略重试 (asynq 延迟队列)
      └─ 指数退避: 10s → 30s → 1m → 5m → 30m
```

#### 内置模板

```yaml
webhook:
  outgoing:
    templates:
      wechat-robot: |                    # 企业微信机器人
        {
          "msgtype": "markdown",
          "markdown": {
            "content": "【{{.title}}】\n{{.message}}"
          }
        }
      slack:                             # Slack
        {
          "text": "{{.title}}\n{{.message}}"
        }
      custom:                            # 自定义
        "{{ toJSON . }}"
```

### 入站 Webhook

接收外部系统的回调请求，验证签名后分发到内部处理器。

#### 接收流程

```
外部系统 → POST /api/v1/webhook/in/{name}
  │
  ├─ 1. 查找 name 对应的 IncomingWebhook 配置
  ├─ 2. 验证签名（根据配置的验证方式）
  │    ├─ HMAC-SHA256 (请求体 + Secret)
  │    ├─ Bearer Token
  │    └─ IP 白名单
  ├─ 3. 解析请求体 → 标准化 Event
  ├─ 4. 调用注册的 Handler 处理
  ├─ 5. 记录处理结果
  └─ 6. 返回 200 OK（快速响应，异步处理）
```

#### 内置处理器

| 名称 | 来源 | 说明 |
|------|------|------|
| `github-push` | GitHub | 代码推送时触发 CI/CD |
| `gitlab-merge` | GitLab | MR 合并时触发部署 |
| `stripe-payment` | Stripe | 支付回调 |
| `custom` | 任意 | 自定义 JSON 回调 |

#### 签名验证

```go
func VerifySignature(signature, secret string, body []byte) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/webhook/out` | 出站 Webhook 列表 |
| `POST` | `/api/v1/webhook/out` | 创建出站 Webhook |
| `PUT` | `/api/v1/webhook/out/:id` | 编辑出站 Webhook |
| `DELETE` | `/api/v1/webhook/out/:id` | 删除出站 Webhook |
| `POST` | `/api/v1/webhook/out/:id/test` | 发送测试消息 |
| `GET` | `/api/v1/webhook/out/:id/logs` | 发送历史 |
| `GET` | `/api/v1/webhook/in` | 入站 Webhook 配置列表 |
| `POST` | `/api/v1/webhook/in` | 创建入站 Webhook |
| `PUT` | `/api/v1/webhook/in/:id` | 编辑入站 Webhook |
| `DELETE` | `/api/v1/webhook/in/:id` | 删除入站 Webhook |
| `POST` | `/api/v1/webhook/in/:name` | 外部系统回调入口 |
| `GET` | `/api/v1/webhook/events` | 查询支持的事件列表 |

### 与 Monitor 模块联动

monitor 告警触发时自动通过 webhook 推送：

```go
// monitor 发出告警 → webhook 发送到企业微信/Slack/钉钉
eventBus.Publish("monitor:alert", Alert{
    Name:     "database_down",
    Severity: "critical",
    Message:  "主数据库不可达",
})
// webhook 模块订阅后匹配出站配置，渲染模板，发送
```

### 配置

```yaml
webhook:
  outgoing:
    default_timeout: 10
    max_retries: 5
    retry_backoff: [10s, 30s, 1m, 5m, 30m]
    ipv4_only: true

  incoming:
    allowed_ips:                           # IP 白名单（可选）
      - "192.30.252.0/22"                  # GitHub
      - "140.82.112.0/20"                  # GitHub
    max_body_size: 1MB                     # 最大请求体
```

## 19. 网络通信模块 (`internal/services/net/` + `pkg/net/`)

提供统一的 HTTP/TCP/UDP/gRPC 客户端与服务器封装，包含连接池、重试、断路器、负载均衡等通用能力。

### 架构定位

核心库 `pkg/net/` 提供底层通信工具，被所有模块直接引用；`internal/services/net/` 提供 HTTP API 封装和管理界面。

```
业务模块 (services / modules)
       │
       ▼
┌─────────────────────────────────────────┐
│            pkg/net (核心库)              │
│  ┌────────┐ ┌────────┐ ┌────────────┐  │
│  │ HTTP   │ │ TCP/   │ │ gRPC       │  │
│  │ Client │ │ UDP    │ │ Client     │  │
│  └────────┘ └────────┘ └────────────┘  │
│  ┌────────┐ ┌────────┐ ┌────────────┐  │
│  │ Proxy  │ │ Tunnel │ │ LB/Retry   │  │
│  │ 支持    │ │ 穿透    │ │ 断路器     │  │
│  └────────┘ └────────┘ └────────────┘  │
└─────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│     internal/services/net (管理 API)     │
│     端口检测 / 代理配置 / 连接管理        │
└─────────────────────────────────────────┘
```

### pkg/net/ — 核心客户端

#### HTTP 客户端

```go
package net

type HTTPClient struct {
    client *http.Client
}

type HTTPConfig struct {
    Timeout           time.Duration  // 请求超时
    RetryMax          int            // 最大重试次数
    RetryWaitMin      time.Duration  // 重试最小间隔
    RetryWaitMax      time.Duration  // 重试最大间隔
    MaxIdleConns      int            // 最大空闲连接
    IdleConnTimeout   time.Duration  // 空闲连接超时
    DisableKeepAlives bool           // 禁用长连接
    ProxyURL          string         // 代理地址（可选）
    TLSConfig         *TLSConfig     // TLS 配置
}

// NewHTTPClient 创建带重试和连接池的 HTTP 客户端
func NewHTTPClient(cfg HTTPConfig) *HTTPClient

// Get/Post/Put/Delete 自动处理重试、超时、连接池
func (c *HTTPClient) Get(ctx, url string) (*Response, error)
func (c *HTTPClient) Post(ctx, url string, body []byte) (*Response, error)
```

| 特性 | 说明 |
|------|------|
| 自动重试 | 指数退避 + jitter，可配置最大次数 |
| 连接池 | 复用连接，减少 TCP 握手 |
| 代理支持 | HTTP/HTTPS/SOCKS5 代理 |
| TLS 配置 | 自定义证书、双向 TLS |
| 断路器 | 连续失败阈值熔断（可选） |
| 限速器 | 每秒请求数限制（可选） |
| 指标 | 请求数 / 延迟 / 错误率 → Prometheus |

#### TCP 客户端

```go
type TCPConn struct {
    conn    net.Conn
    timeout time.Duration
}

// 用于自定义协议设备通信（如 UPS NUT、局域网设备）
func DialTCP(ctx, addr string, cfg TCPConfig) (*TCPConn, error)
func (c *TCPConn) Send(data []byte) ([]byte, error)
func (c *TCPConn) SendWithTimeout(data []byte, timeout time.Duration) ([]byte, error)
```

#### UDP 客户端

```go
func DialUDP(ctx, addr string, cfg UDPConfig) (*UDPConn, error)
func (c *UDPConn) Send(data []byte) ([]byte, error)
```

#### gRPC 客户端

```go
type GRPCClient struct {
    conn *grpc.ClientConn
}

func NewGRPCClient(addr string, opts ...grpc.DialOption) (*GRPCClient, error)
func (c *GRPCClient) Conn() *grpc.ClientConn   // 返回原始连接供 protobuf 使用
func (c *GRPCClient) Close() error
```

### 内置代理支持

| 代理类型 | 用途 |
|----------|------|
| HTTP 正向代理 | 出站 HTTP 请求代理 |
| SOCKS5 | TCP/UDP 代理 |
| 反向代理 | 入站 HTTP 转发 |
| TCP 隧道 | 端口转发（如本地转发到内网服务） |

### 端口扫描与检测

```go
func CheckPort(host string, port int, timeout time.Duration) bool
func ScanPorts(host string, ports []int, timeout time.Duration) map[int]bool
func CheckTCP(host string, port int) bool
func CheckHTTP(url string) (int, error)  // 返回状态码
```

### Services/net — 管理 API

通过 HTTP API 暴露网络检测和管理能力：

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/net/http/request` | 发送 HTTP 请求（自定义 URL/方法/头/体） |
| `POST` | `/api/v1/net/http/check` | HTTP 健康检查（状态码 + 延迟） |
| `POST` | `/api/v1/net/tcp/check` | TCP 端口连通性检测 |
| `POST` | `/api/v1/net/udp/check` | UDP 端口检测 |
| `POST` | `/api/v1/net/dns/lookup` | DNS 解析查询 |
| `POST` | `/api/v1/net/traceroute` | 路由追踪 |
| `GET` | `/api/v1/net/proxy` | 代理配置查看 |
| `PUT` | `/api/v1/net/proxy` | 更新代理配置 |

### 与各模块集成

```go
// webhook 模块使用 HTTP 客户端发送
webhookHTTP := pkgnet.NewHTTPClient(pkgnet.HTTPConfig{
    Timeout:  10 * time.Second,
    RetryMax: 3,
})

// ups 模块使用 TCP 连接 NUT
nutConn, _ := pkgnet.DialTCP(ctx, "nas.local:3493", pkgnet.TCPConfig{
    Timeout: 5 * time.Second,
})

// discovery 模块使用 gRPC
grpcConn, _ := pkgnet.NewGRPCClient("peer:29091", grpc.WithInsecure())

// backup 模块使用 HTTP 客户端上传到 RustFS
rustFSClient := pkgnet.NewHTTPClient(pkgnet.HTTPConfig{
    Timeout: 300 * time.Second,
})
```

### 配置

```yaml
net:
  http:
    default_timeout: 30s
    max_retries: 3
    retry_wait_min: 500ms
    retry_wait_max: 5s
    max_idle_conns: 100
    idle_conn_timeout: 90s
    proxy:                              # 全局代理（可选）
      http:  "http://proxy:8080"
      https: "http://proxy:8080"
      socks5: "socks5://proxy:1080"
      no_proxy:                         # 不走代理的地址
        - "localhost"
        - "10.0.0.0/8"
        - "nas.local"

  tcp:
    connect_timeout: 5s
    read_timeout: 10s

  grpc:
    default_timeout: 30s
    max_retries: 2

  proxy:
    enabled: false
    http_port: 8080                     # 内置 HTTP 代理端口（可选启用）
```

## 20. 事件总线模块 (`internal/services/eventbus/`)

为模块间提供正式的发布/订阅消息通道，替代目前伪代码中的 `eventBus.Publish()`。

### 技术选型

| 组件 | 用途 |
|------|------|
| **Redis Stream** | 持久化消息队列，支持消费者组 |
| **asynq** | 延迟/定时消息 |
| **内存 Channel** | 同进程模块间同步通信（低延迟，不持久化） |

### 架构

```
                      ┌──────────────────┐
                      │    Event Bus      │
                      │  ┌────────────┐   │
  Publisher ──────────┼─►│  Topic 1   │───┼──► Consumer A
                      │  └────────────┘   │
  Publisher ──────────┼─►┌────────────┐   │
                      │  │  Topic 2   │───┼──► Consumer B, C
                      │  └────────────┘   │
                      │  ┌────────────┐   │
                      │  │  Dead Letter│   │  ← 重试失败的消息
                      │  └────────────┘   │
                      └──────────────────┘
```

### 接口

```go
type Event struct {
    ID        string         `json:"id"`
    Type      string         `json:"type"`     // "backup.completed", "ups.on_battery"
    Source    string         `json:"source"`   // 发布者模块名
    Timestamp time.Time      `json:"timestamp"`
    Payload   map[string]any `json:"payload"`
    Metadata  map[string]any `json:"metadata"`
}

type EventBus interface {
    Publish(topic string, event Event) error
    Subscribe(topic string, handler EventHandler) (Subscription, error)
    Unsubscribe(sub Subscription) error
    PublishAsync(topic string, event Event) (string, error)  // 异步，返回消息 ID
}
```

### 预置主题

| 主题 | 发布者 | 消费者 | 用途 |
|------|--------|--------|------|
| `ups.on_battery` | ups | lanctl, notify, webhook | 断电通知 |
| `ups.online` | ups | lanctl | 恢复供电 |
| `backup.completed` | backup | notify, webhook | 备份完成通知 |
| `backup.failed` | backup | monitor, webhook | 备份失败告警 |
| `monitor.alert` | monitor | notify, webhook | 告警推送 |
| `config.changed` | config | 所有模块 | 配置热加载 |
| `node.join` | discovery | eventbus | 节点加入 |
| `node.leave` | discovery | eventbus | 节点离开 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/eventbus/publish` | 发布消息 |
| `POST` | `/api/v1/eventbus/subscribe` | 创建订阅 |
| `DELETE` | `/api/v1/eventbus/subscribe/:id` | 取消订阅 |
| `GET` | `/api/v1/eventbus/topics` | 主题列表+订阅者数 |
| `GET` | `/api/v1/eventbus/topics/:topic/messages` | 消息回溯 |

## 21. 缓存管理模块 (`internal/services/cache/`)

统一缓存层，业务模块不直接操作 Redis，通过 cache 模块读写。

### 缓存策略

```go
type Cache interface {
    Get(ctx, key string, dest any) error
    Set(ctx, key string, val any, ttl time.Duration) error
    Delete(ctx, key string) error
    GetOrSet(ctx, key string, ttl time.Duration, fn func() (any, error), dest any) error
    // 批量操作
    MGet(ctx, keys []string) (map[string]any, error)
    MSet(ctx, items map[string]any, ttl time.Duration) error
    // 分布式锁
    Lock(ctx, key string, ttl time.Duration) (bool, error)
    Unlock(ctx, key string) error
}
```

### 多级缓存

```
L1: 内存缓存 (freecache / bigcache) → TTL 秒级，容量有限
L2: Redis                  → TTL 分钟~小时级
```

业务模块通过 `cache.GetOrSet` 一行代码完成"先查缓存 → 查到返回 → 未查到执行函数 → 写入缓存"。

### 失效策略

| 策略 | 触发方式 | 说明 |
|------|----------|------|
| TTL 过期 | 时间 | 自动淘汰 |
| 主动失效 | 事件 | 数据变更时 `cache.Delete(key)` |
| 批量失效 | 前缀 | `cache.DeletePrefix("user:*")` |
| 防雪崩 | 随机 TTL | `baseTTL + rand(0, 300s)` |
| 防穿透 | Bloom Filter | 不存在的数据也缓存空值短时间 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/cache/:key` | 读取缓存 |
| `PUT` | `/api/v1/cache/:key` | 写入缓存 |
| `DELETE` | `/api/v1/cache/:key` | 删除缓存 |
| `GET` | `/api/v1/cache/stats` | 缓存命中率/容量 |

## 22. 全文搜索模块 (`internal/services/search/`)

基于 Meilisearch 的统一全文搜索服务。

### 索引管理

```go
type SearchIndex struct {
    Name        string   // 索引名
    PrimaryKey  string   // 主键字段
    SearchableAttrs []string // 可搜索字段
    FilterableAttrs []string // 可过滤字段
}
```

业务模块在 Init 时注册索引：

```go
search.RegisterIndex(SearchIndex{
    Name:        "s1_orders",
    PrimaryKey:  "id",
    SearchableAttrs: []string{"order_no", "user_name"},
    FilterableAttrs: []string{"status", "created_at"},
})
```

### 同步方式

| 方式 | 说明 |
|------|------|
| 实时同步 | 数据变更时立即写入搜索索引 |
| 定时同步 | scheduler 触发全量重建索引 |
| 手动同步 | API 触发 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/search/:index/query` | 搜索 |
| `POST` | `/api/v1/search/:index/documents` | 索引文档 |
| `DELETE` | `/api/v1/search/:index/documents/:id` | 删除文档 |
| `POST` | `/api/v1/search/:index/reindex` | 重建索引 |
| `GET` | `/api/v1/search/indexes` | 索引状态 |

## 23. 通知渠道模块 (`internal/services/notify/`)

将原有 notify 占位扩展为完整的多渠道消息发送服务。

### 渠道支持

| 渠道 | 实现 | 配置项 |
|------|------|--------|
| **邮件** | `net/smtp` / SendGrid API | SMTP 服务器 / API Key |
| **短信** | 阿里云 / 腾讯云 SMS SDK | AccessKey + SignName |
| **Push** | FCM (Android) / APNs (iOS) / HMS (华为) | 各平台服务账号密钥 |
| **站内信** | PostgreSQL | 存储后用户拉取 |
| **小程序订阅消息** | 微信 SDK | template_id + 用户订阅关系 |

### 统一发送接口

```go
type Message struct {
    Channel  string           // email | sms | push | inapp | weapp
    To       []string         // 收件人（邮箱/手机号/device_token）
    Title    string           // 标题
    Body     string           // 正文（渲染后的文本）
    Template string           // 模板名称（可选）
    Data     map[string]any   // 模板数据
}

type Notifier interface {
    Send(ctx, msg Message) (string, error)
    SendBatch(ctx, msgs []Message) []BatchResult
}
```

### 配置

```yaml
notify:
  email:
    smtp_host: "smtp.example.com"
    smtp_port: 587
    from: "noreply@example.com"
  sms:
    provider: "aliyun"
    access_key: "${SMS_ACCESS_KEY}"
    sign_name: "MyApp"
  push:
    fcm:
      credentials_file: "/run/secrets/fcm.json"
    apns:
      key_id: "xxx"
      team_id: "xxx"
    hms:
      app_id: "100xxx"
```

## 24. API 密钥管理模块 (`internal/services/apikey/`)

管理客户端/第三方系统的 API 访问密钥，区别于用户 JWT 认证。

### 密钥模型

```go
type APIKey struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`       // 密钥名称（如 "微信小程序-生产"）
    Key       string    `json:"key"`        // 密钥本身（创建时返回一次，之后脱敏）
    HashedKey string    `json:"-"`          // bcrypt 哈希存储
    Scopes    []string  `json:"scopes"`     // 权限范围
    AllowedIPs []string `json:"allowed_ips"` // IP 白名单
    RateLimit int       `json:"rate_limit"` // 单独限流
    ExpiresAt *time.Time `json:"expires_at"`
    Status    string    `json:"status"`     // active | expired | revoked
    CreatedBy int64     `json:"created_by"`
}
```

### 使用场景

| 场景 | 密钥用途 | Scope 示例 |
|------|----------|-----------|
| 微信小程序 | 服务端调用凭证 | `miniapp:read` |
| Android App | 客户端身份标识 | `mobile:read`, `mobile:write` |
| iOS App | 客户端身份标识 | `mobile:read` |
| 第三方系统 | 外部服务对接 | `webhook:write`, `data:read` |
| Web 前端 | 公开 API 限流 | `public:read` |

### 中间件

```go
// 验证 HTTP Header: X-API-Key: sk_xxxxx
router.Use(apikey.Middleware(apikey.Config{
    HeaderName: "X-API-Key",
    ExcludePaths: []string{"/api/v1/auth/", "/api/v1/s0/"},
}))
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/apikey` | 密钥列表 |
| `POST` | `/api/v1/apikey` | 创建密钥 |
| `DELETE` | `/api/v1/apikey/:id` | 吊销密钥 |
| `GET` | `/api/v1/apikey/:id/logs` | 使用记录 |

## 25. 数据导入导出模块 (`internal/services/dataio/`)

提供统一的 CSV/Excel 导入导出能力，避免各业务模块各自实现。

### 导出

```go
type ExportRequest struct {
    Columns []string          // 列名
    Data    []map[string]any  // 数据行
    Format  string            // csv | xlsx
    Options ExportOptions
}

// 任意查询结果 → 导出文件
func Export(ctx, req ExportRequest) (io.Reader, error)
```

支持大数据量流式导出（chunked write），避免内存溢出。

### 导入

```go
type ImportResult struct {
    Total   int
    Success int
    Errors  []ImportError  // 行号 + 错误原因
}

func Import(ctx, file io.Reader, format string, handler ImportRowHandler) (*ImportResult, error)
```

业务模块注册导入处理器：

```go
dataio.RegisterHandler("s1_orders_import", func(row map[string]any) error {
    // 业务校验 + 写入 DB
    return nil
})
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/dataio/export` | 导出为 CSV/XLSX |
| `POST` | `/api/v1/dataio/import/:handler` | 上传文件导入 |
| `GET` | `/api/v1/dataio/handlers` | 注册的导入处理器 |

## 26. 特性开关模块 (`internal/services/feature/`)

按用户/环境/平台控制功能灰度发布。

```go
type FeatureFlag struct {
    Name        string            // 特性名
    Enabled     bool              // 全局启用
    Percentage  int               // 百分比灰度 0-100
    UserIDs     []int64           // 指定用户
    Platforms   []string          // 指定平台
    Env         string            // 指定环境
    Attributes  map[string]string // 自定义属性匹配
}
```

```go
// 业务代码中
if feature.IsEnabled(ctx, "new_checkout_flow") {
    // 新流程
} else {
    // 旧流程
}
```

## 27. 模板引擎模块 (`internal/services/template/`)

集中管理 Go template，供通知/邮件/导出使用。

```go
type TemplateManager struct {
    // 模板来源: DB | 文件 | 配置
    // 支持版本管理 + 预览 + 测试发送
}

func (tm *TemplateManager) Render(name, version string, data any) (string, error)
func (tm *TemplateManager) Validate(name, content string) error
```

## 28. 规则引擎模块 (`internal/services/rules/`)

可配置的业务规则，避免硬编码。

```go
type Rule struct {
    Name       string         // 规则名
    Conditions []Condition    // 条件列表（AND/OR）
    Actions    []Action       // 动作列表
    Priority   int            // 优先级
    Enabled    bool
}

type Condition struct {
    Field    string // 字段名
    Operator string // eq | neq | gt | lt | in | contains
    Value    any
}

type Action struct {
    Type   string         // set | notify | reject | webhook
    Params map[string]any
}
```

使用场景：风控规则、优惠计算、审批流。

## 29. 分布式限流模块 (`internal/services/ratelimit/`)

多副本部署时，在 Redis 中中心化限流，配合 middleware 使用。

```go
type RateLimitStrategy struct {
    Name     string // "api:user" | "api:ip" | "api:apikey"
    Limit    int    // 窗口内最大请求数
    Window   time.Duration // 时间窗口
    KeyFunc  func(ctx) string // 限流 key 提取
}
```

支持算法：**滑动窗口**（默认）、**令牌桶**、**漏桶**。

## 30. MQTT 协议模块 (`internal/services/mqtt/`)

为 Home Assistant 及 IoT 设备提供 MQTT 客户端能力。

```go
type MQTTClient struct {
    client mqtt.Client
}

type MQTTConfig struct {
    Broker   string   // tcp://ha.local:1883
    ClientID string
    Username string
    Password string
    Topics   []string // 订阅主题列表
    QoS      byte     // 0 | 1 | 2
}

func (c *MQTTClient) Publish(topic string, payload []byte, qos byte) error
func (c *MQTTClient) Subscribe(topic string, handler MessageHandler) error
func (c *MQTTClient) Unsubscribe(topic string) error
```

### 与 eventbus 集成

```go
// MQTT 消息 → 内部 EventBus
mqtt.Subscribe("homeassistant/#", func(msg MQTTMessage) {
    eventbus.Publish("mqtt:"+msg.Topic(), msg.Payload())
})

// 内部事件 → MQTT 发布
eventbus.Subscribe("ups.on_battery", func(evt Event) {
    mqtt.Publish("homeassistant/ups/status", evt.Payload, 1)
})
```

## 31. 文件系统模块 (`internal/services/filesystem/`)

监听、扫描、索引本地文件，为 Obsidian 同步、音乐库扫描、备份等场景提供统一文件操作层。

```go
type Watcher struct {
    dirs   []string
    events chan FileEvent  // 文件新增/修改/删除/重命名
}

type Scanner struct {
    extensions []string     // 限定扩展名
    maxDepth   int
}

// 启动监听
watcher.Watch("/data/obsidian")
// 全量扫描
entries, _ := scanner.Scan("/data/music")
// 索引结果 → eventbus 推送
eventbus.Publish("filesystem:file_created", event)
```

## 32. 标签系统模块 (`internal/services/tagging/`)

跨模块统一的标签/分类系统，物品管理、音乐风格、知识分类都复用同一套。

```go
type Tag struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`       // 标签名
    Color     string `json:"color"`      // 显示颜色
    Group     string `json:"group"`      // 分组（如 "music/genre", "inventory/category"）
    CreatedAt time.Time `json:"created_at"`
}

// 任何业务实体都可以打标签
type Taggable interface {
    TagID() int64
}

func Attach(ctx, tagID, entityID int64, entityType string) error
func Detach(ctx, tagID, entityID int64, entityType string) error
func GetByTag(ctx, tagID, entityType string) ([]int64, error) // 按标签查实体 ID
func GetByEntity(ctx, entityID int64, entityType string) ([]Tag, error)
```

## 33. 后台任务管理模块 (`internal/services/jobs/`)

在 scheduler 的触发基础上，增加长时间任务的进度上报、取消、重跑、日志查看。

```go
type Job struct {
    ID        string
    Type      string       // "media.scan" | "backup.run" | "search.reindex"
    Progress  float64      // 0.0 - 1.0
    Status    string       // pending | running | completed | failed | cancelled
    Log       []string     // 执行日志
    StartedAt *time.Time
    EndedAt   *time.Time
}

// 业务模块注册 JobHandler
func RegisterJobHandler(jobType string, handler func(ctx JobContext) error)

// JobContext 提供进度上报和取消监听
type JobContext struct {
    jobID     string
    cancelCh  chan struct{}
}

func (jc *JobContext) SetProgress(pct float64)
func (jc *JobContext) IsCancelled() bool
func (jc *JobContext) Logf(format string, args ...any)
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/jobs` | 任务列表 |
| `GET` | `/api/v1/jobs/:id` | 任务详情（进度/日志） |
| `POST` | `/api/v1/jobs/:id/cancel` | 取消任务 |
| `POST` | `/api/v1/jobs/:id/rerun` | 重跑任务 |
| `DELETE` | `/api/v1/jobs/:id` | 清理任务记录 |

## 34. 媒体管理模块 (`internal/services/media/`)

音乐库扫描、音频元数据读写、与 Navidrome 等媒体服务对接。

```go
type AudioFile struct {
    Path      string
    Title     string
    Artist    string
    Album     string
    Genre     string
    Year      int
    Track     int
    Duration  float64
    Bitrate   int
    Format    string  // mp3 | flac | wav | m4a
    CoverArt  []byte  // 封面图片
}

func ScanLibrary(ctx, rootPath string) (*ScanResult, error)
func ReadMetadata(ctx, filePath string) (*AudioFile, error)
func WriteTag(ctx, filePath string, tag *AudioFile) error
```

与 filesystem 模块联动：

```go
filesystem.Watch("/data/music") → eventbus "filesystem:created" → media.ScanLibrary
```

## 35. 报表/聚合引擎模块 (`internal/services/analytics/`)

为物品管理等业务模块提供统一的数据统计和报表生成能力。

```go
type Query struct {
    Table       string   // 数据表
    Metric      string   // count | sum | avg | max | min
    GroupBy     []string // 分组字段
    TimeField   string   // 时间字段
    TimeRange   TimeRange
    Filters     []Filter
}

type Report struct {
    Columns []string
    Rows    []map[string]any
    Total   int
}

func Aggregate(ctx, query Query) (*Report, error)
func ExportReport(ctx, query Query, format string) (io.Reader, error) // csv | xlsx
```

## 36. 插件系统模块 (`internal/services/plugin/`)

统一的插件注册和加载机制，外部集成（Dify/N8n/Home Assistant）以插件形式接入，不修改核心代码。

```go
type PluginManifest struct {
    Name        string   // "dify-integration"
    Version     string
    Description string
    Routes      []Route        // 插件注册的路由
    Events      []string       // 插件订阅的事件
    Tasks       []TaskDef      // 插件注册的后台任务
    Webhooks    []WebhookDef   // 插件注册的入站 webhook
}

type Plugin interface {
    Manifest() PluginManifest
    Init(cfg PluginConfig) error
    Close() error
}

// 插件加载方式
// 1. 内嵌：plugin.Register(&DifyPlugin{}) 在代码中注册
// 2. 外部：扫描 /plugins/*.so 动态加载（Go plugin，后期）
// 3. 远程：通过 HTTP 获取插件清单（预留）
```

### 插件示例

```go
// dify_plugin.go
type DifyPlugin struct{}

func (p *DifyPlugin) Manifest() PluginManifest {
    return PluginManifest{
        Name:    "dify",
        Version: "1.0",
        Routes: []Route{
            {Method: "POST", Path: "/dify/webhook", Handler: p.handleWebhook},
        },
        Events: []string{"ai:generated", "filesystem:created"},
        Tasks:  []TaskDef{{Type: "dify.sync", Schedule: "@every 10m"}},
    }
}
```
