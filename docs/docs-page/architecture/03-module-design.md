---
title: 模块化设计
parent: 架构设计
nav_order: 3
---

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
        auth.GET("/" + name + "/url", a.GetAuthURL(name))
        auth.POST("/" + name + "/callback", a.Callback(name))
        auth.POST("/" + name + "/bind", a.Bind(name))
        auth.POST("/" + name + "/unbind", a.Unbind(name))
    }
    auth.POST("/miniapp/login", a.MiniAppLogin)
}
```

#### 多账号绑定

支持一个本地用户绑定多个第三方账号（含同平台多个账号），详见社交登录接入方案文档。

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
    Name             string  `json:"name"`
    Model            string  `json:"model"`
    Status           string  `json:"status"`
    BatteryCharge    int     `json:"battery_charge"`
    BatteryRuntime   int     `json:"battery_runtime"`
    BatteryVoltage   float64 `json:"battery_voltage"`
    InputVoltage     float64 `json:"input_voltage"`
    OutputVoltage    float64 `json:"output_voltage"`
    LoadPercent      float64 `json:"load_percent"`
    TimeLeft         string  `json:"time_left"`
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
  driver: "nut"
  nut_host: "localhost"
  nut_port: 3493
  nut_username: "monuser"
  nut_password: "secret"
  ups_name: "myups"
  poll_interval: 5s
  events:
    on_battery_delay: 5s
    battery_low_threshold: 15
    online_stable_delay: 30s
    shutdown_timeout: 120s
```

## 7. 局域网设备管理模块 (`internal/services/lanctl/`)

管理局域网内设备，在 UPS 断电时按照预定顺序执行优雅关机。

### 设备注册

```go
type LANDevice struct {
    ID          int       `json:"id"`
    Name        string    `json:"name"`
    Host        string    `json:"host"`
    Port        int       `json:"port"`
    AuthMethod  string    `json:"auth_method"`
    Username    string    `json:"username"`
    Password    string    `json:"password,omitempty"`
    KeyPath     string    `json:"key_path,omitempty"`
    ShutdownCmd string    `json:"shutdown_cmd"`
    Timeout     int       `json:"timeout"`
    Order       int       `json:"order"`
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
  shutdown_timeout_per_device: 30
  concurrent: false
  ssh_connect_timeout: 10s
  auto_discovery: false
```

### 与 UPS 模块联动

```
ups (OnBattery) ──事件通知──→ lanctl (启动关机倒计时)
                                   │
ups (Online)  ──事件通知──→ lanctl (取消关机倒计时)
                                   │
ups (BatteryLow) ────→ lanctl (立即执行关机)
```

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
    Name        string          `json:"name"`
    Type        string          `json:"type"`
    Expression  string          `json:"expression"`
    Handler     string          `json:"handler"`
    Payload     json.RawMessage `json:"payload,omitempty"`
    Status      string          `json:"status"`
    MaxRetries  int             `json:"max_retries"`
    Timeout     int             `json:"timeout"`
    LastRunAt   *time.Time      `json:"last_run_at"`
    NextRunAt   *time.Time      `json:"next_run_at"`
    CreatedBy   int64           `json:"created_by"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

type TaskLog struct {
    ID        int64          `json:"id" gorm:"primaryKey"`
    TaskID    int64          `json:"task_id"`
    Status    string         `json:"status"`
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

### 处理器注册机制

```go
type TaskHandler interface {
    Name() string
    Execute(ctx context.Context, payload json.RawMessage) error
}

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
  enabled: true
  timezone: "Asia/Shanghai"
  worker_concurrency: 5
  default_timeout: 300
  default_max_retries: 3
  log_retention_days: 90
  health_check_cron: "@every 5m"
  cleanup_cron: "0 3 * * *"
```

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
    Name         string           `json:"name"`
    BackupType   string           `json:"backup_type"`
    Schedule     string           `json:"schedule"`
    TargetPath   string           `json:"target_path"`
    RemoteTarget string           `json:"remote_target,omitempty"`
    Retention    int              `json:"retention"`
    Compression  string           `json:"compression"`
    Encrypt      bool             `json:"encrypt"`
    Enabled      bool             `json:"enabled"`
    LastBackupAt *time.Time       `json:"last_backup_at"`
    CreatedAt    time.Time        `json:"created_at"`
}
```

### API

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

### 配置

```yaml
backup:
  storage_path: "/data/backups"
  temp_path: "/tmp/backups"
  remote:
    rustfs:
      bucket: "backups"
      prefix: "db/"
    nas:
      path: "//nas/backup/share"
  encryption_key_path: "/run/secrets/backup_key"
  default_retention: 30
  restore_confirm_ttl: 5m
  executor:
    pg_dump: "pg_dump"
    mysqldump: "mysqldump"
    redis_cli: "redis-cli"
```

## 10. 数据库迁移管理模块 (`internal/services/migration/`)

基于 `golang-migrate/migrate` 的版本化迁移管理，提供 HTTP API 执行、查看、回滚迁移。

### 与 GORM AutoMigrate 的定位

| 方式 | 适用环境 | 用途 |
|------|----------|------|
| GORM AutoMigrate | 开发/测试 | 自动同步模型，快速原型 |
| golang-migrate | 生产/预发布 | 版本化 SQL，可回滚，可 Review |

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

## 11. 节点发现与 P2P 连接模块 (`internal/services/discovery/`)

管理多节点间的自动发现、健康监测和点对点加密通信。

### 技术方案

| 组件 | 技术 | 说明 |
|------|------|------|
| **节点发现** | **mDNS**（局域网）+ **Redis Registry**（广域网） | 局域网零配置发现，广域网通过注册中心 |
| **P2P 传输** | **libp2p** | 去中心化、NAT 穿透、加密传输、多路复用 |
| **健康监测** | 自定义心跳 + Gossip | 节点状态实时感知 |
| **数据同步** | **gRPC stream** + **PubSub** | 状态同步 / 事件广播 |

### 发现方式

| 方式 | 网络 | 配置 | 说明 |
|------|------|------|------|
| **mDNS** | 局域网 | 零配置 | 自动发现同网段节点，适合 NAS 内网环境 |
| **Redis Registry** | 广域网 | 需 Redis 地址 | 节点注册到 Redis，定期心跳续期 |
| **Static** | 任何网络 | 手动配置节点地址列表 | 兜底方案，指定已知节点 |

### 节点模型

```go
type NodeInfo struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    PeerID      string            `json:"peer_id"`
    Addrs       []string          `json:"addrs"`
    Version     string            `json:"version"`
    Uptime      int64             `json:"uptime"`
    Load        float64           `json:"load"`
    Roles       []string          `json:"roles"`
    Status      string            `json:"status"`
    LastSeen    time.Time         `json:"last_seen"`
    Labels      map[string]string `json:"labels"`
    Metadata    json.RawMessage   `json:"metadata"`
}
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/discovery/nodes` | 节点列表（含状态） |
| `GET` | `/api/v1/discovery/nodes/:id` | 节点详情 |
| `GET` | `/api/v1/discovery/peers` | 已建立 P2P 连接的节点 |
| `POST` | `/api/v1/discovery/connect` | 手动连接指定节点 |
| `POST` | `/api/v1/discovery/disconnect/:id` | 断开与指定节点的 P2P 连接 |
| `GET` | `/api/v1/discovery/self` | 本节点信息 |

## 12. 加密解密与压缩解压缩模块 (`internal/services/crypto/` + `pkg/crypto/`)

提供统一的加解密、哈希、压缩解压缩能力。

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

func Encrypt(plaintext []byte, key *[32]byte) ([]byte, error)
func Decrypt(ciphertext []byte, key *[32]byte) ([]byte, error)
func EncryptFile(src, dst string, key *[32]byte) error
func DecryptFile(src, dst string, key *[32]byte) error
func GenerateKeyPair(bits int) (*PrivateKey, *PublicKey, error)
func EncryptRSA(plaintext []byte, pub *PublicKey) ([]byte, error)
func DecryptRSA(ciphertext []byte, priv *PrivateKey) ([]byte, error)
func HashPassword(password string) (string, error)
func VerifyPassword(password, hash string) bool
func Compress(data []byte, algo Algorithm) ([]byte, error)
func Decompress(data []byte) ([]byte, error)
func EncryptAndCompress(data []byte, key *[32]byte) ([]byte, error)
func DecryptAndDecompress(data []byte, key *[32]byte) ([]byte, error)
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/crypto/encrypt` | 加密数据 |
| `POST` | `/api/v1/crypto/decrypt` | 解密数据 |
| `POST` | `/api/v1/crypto/compress` | 压缩数据 |
| `POST` | `/api/v1/crypto/decompress` | 解压数据 |
| `POST` | `/api/v1/crypto/hash` | 计算哈希 |
| `POST` | `/api/v1/crypto/keys` | 列出可用密钥名称 |
| `POST` | `/api/v1/crypto/keygen` | 生成新密钥 |

## 13. AI 推理服务模块 (`internal/services/ai/`)

统一接入多种 AI 推理后端。

### 支持的推理后端

| 后端 | 连接方式 | 适用场景 |
|------|----------|----------|
| **Ollama** | HTTP API (`ollama/api`) | 本地私有化 LLM |
| **Direct GPU** | gRPC (Triton / vLLM) | 高性能自研模型 |
| **OpenAI API** | HTTP | GPT-4o 等云端模型 |
| **One API** | HTTP (OpenAI 兼容) | 聚合多供应商 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/ai/models` | 可用模型列表 |
| `POST` | `/api/v1/ai/chat` | 对话补全（支持 stream SSE） |
| `POST` | `/api/v1/ai/embeddings` | 向量嵌入 |
| `GET` | `/api/v1/ai/providers` | Provider 状态 |

## 14. 统一数据访问模块 (`internal/database/`)

统一管理所有数据存储的连接池、健康检查和指标采集。

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
    Postgres map[string]*pgxpool.Pool
    MySQL    map[string]*sql.DB
    Redis    map[string]*redis.Client
    S3       map[string]*minio.Client
}

var DB *DBManager

func InitAll(cfg *config.Config) error
func CloseAll()
func Health() map[string]HealthStatus
```

## 15. 日志管理模块 (`internal/services/logger/`)

集中管理所有日志输出、存储、查询与审计。

### 日志分层

```
┌─────────────┐
│  业务日志    │ ← 各模块通过 Logger 接口写入
├─────────────┤
│  访问日志    │ ← Gin middleware 自动记录
├─────────────┤
│  系统日志    │ ← 服务启动/关闭/错误
├─────────────┤
│  审计日志    │ ← 敏感操作
└──────┬──────┘
       │
       ▼
┌──────────────────────┐
│    Log Processor     │
├──────────────────────┤
│    Log Exporter      │
└──────────────────────┘
       │
       ├── stdout
       ├── 文件 (日志轮转)
       └── Loki
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/logger/level` | 获取当前日志级别 |
| `PUT` | `/api/v1/logger/level` | 动态调整日志级别 |
| `GET` | `/api/v1/logger/search` | 搜索日志 |
| `GET` | `/api/v1/logger/audit` | 审计日志查询（管理员） |
| `GET` | `/api/v1/logger/stats` | 日志量统计 |

## 16. 监控模块 (`internal/services/monitor/`)

聚合所有健康检查、性能指标，暴露 Prometheus 端点。

### 健康检查聚合

```go
type HealthChecker interface {
    Name() string
    Check(ctx) HealthResult
}

type HealthResult struct {
    Status   string            `json:"status"`
    Latency  int64             `json:"latency_ms"`
    Message  string            `json:"message,omitempty"`
    Details  map[string]any    `json:"details,omitempty"`
}
```

### Prometheus 指标

| 指标 | 类型 | 说明 |
|------|------|------|
| `go_*` | 内置 | Go 运行时指标 |
| `http_requests_total` | Counter | 请求总数 |
| `http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `db_connections_open` | Gauge | 数据库连接数 |
| `db_query_duration_seconds` | Histogram | 查询延迟 |
| `ups_battery_charge` | Gauge | UPS 剩余电量 |
| `node_health_status` | Gauge | 各节点健康状态 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/monitor/health` | 全服务健康状态聚合 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/v1/monitor/status` | 模块运行状态 |
| `GET` | `/api/v1/monitor/alerts` | 当前活跃告警列表 |
| `GET` | `/api/v1/monitor/dashboard` | 关键指标摘要 |

## 17. 配置信息管理模块 (`internal/services/config/`)

集中管理所有配置的读取、热加载、版本查询和敏感字段加解密。

### 功能

| 功能 | 说明 |
|------|------|
| **配置读取** | Viper 加载 config.yaml + 环境变量覆盖 |
| **热加载** | 监听配置文件变更，自动重载 |
| **配置查询** | 通过 API 查看当前生效配置（管理员） |
| **配置校验** | 启动时校验配置完整性 |
| **敏感加密** | 配置中的密码/密钥字段自动加解密 |
| **版本追踪** | 记录配置变更历史 |

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/config` | 当前配置（管理员，敏感字段脱敏） |
| `GET` | `/api/v1/config/:key` | 按路径查询特定配置 |
| `POST` | `/api/v1/config/reload` | 手动触发配置重载 |
| `GET` | `/api/v1/config/history` | 配置变更历史 |
| `GET` | `/api/v1/config/schema` | 配置结构定义文档 |
| `GET` | `/api/v1/config/validate` | 验证当前配置完整性 |

## 18. Webhook 模块 (`internal/services/webhook/`)

管理出站 Webhook 和入站 Webhook，支持签名验证、重试、模板化。

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

## 19. 网络通信模块 (`internal/services/net/` + `pkg/net/`)

提供统一的 HTTP/TCP/UDP/gRPC 客户端与服务器封装。

### pkg/net/ — 核心客户端

```go
type HTTPClient struct {
    client *http.Client
}

type HTTPConfig struct {
    Timeout           time.Duration
    RetryMax          int
    RetryWaitMin      time.Duration
    RetryWaitMax      time.Duration
    MaxIdleConns      int
    IdleConnTimeout   time.Duration
    DisableKeepAlives bool
    ProxyURL          string
    TLSConfig         *TLSConfig
}
```

| 特性 | 说明 |
|------|------|
| 自动重试 | 指数退避 + jitter |
| 连接池 | 复用连接 |
| 代理支持 | HTTP/HTTPS/SOCKS5 |
| TLS 配置 | 自定义证书、双向 TLS |
| 断路器 | 连续失败阈值熔断 |
| 限速器 | 每秒请求数限制 |
| 指标 | 请求数 / 延迟 / 错误率 → Prometheus |

### Services/net — 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/net/http/request` | 发送 HTTP 请求 |
| `POST` | `/api/v1/net/http/check` | HTTP 健康检查 |
| `POST` | `/api/v1/net/tcp/check` | TCP 端口连通性检测 |
| `POST` | `/api/v1/net/udp/check` | UDP 端口检测 |
| `POST` | `/api/v1/net/dns/lookup` | DNS 解析查询 |
| `POST` | `/api/v1/net/traceroute` | 路由追踪 |
| `GET` | `/api/v1/net/proxy` | 代理配置查看 |
| `PUT` | `/api/v1/net/proxy` | 更新代理配置 |

## 20. 事件总线模块 (`internal/services/eventbus/`)

为模块间提供正式的发布/订阅消息通道。

### 技术选型

| 组件 | 用途 |
|------|------|
| **Redis Stream** | 持久化消息队列，支持消费者组 |
| **asynq** | 延迟/定时消息 |
| **内存 Channel** | 同进程模块间同步通信 |

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

统一缓存层，业务模块不直接操作 Redis。

### 缓存策略

```go
type Cache interface {
    Get(ctx, key string, dest any) error
    Set(ctx, key string, val any, ttl time.Duration) error
    Delete(ctx, key string) error
    GetOrSet(ctx, key string, ttl time.Duration, fn func() (any, error), dest any) error
    MGet(ctx, keys []string) (map[string]any, error)
    MSet(ctx, items map[string]any, ttl time.Duration) error
    Lock(ctx, key string, ttl time.Duration) (bool, error)
    Unlock(ctx, key string) error
}
```

### 多级缓存

```
L1: 内存缓存 (freecache / bigcache) → TTL 秒级，容量有限
L2: Redis                  → TTL 分钟~小时级
```

### 失效策略

| 策略 | 触发方式 | 说明 |
|------|----------|------|
| TTL 过期 | 时间 | 自动淘汰 |
| 主动失效 | 事件 | 数据变更时 `cache.Delete(key)` |
| 批量失效 | 前缀 | `cache.DeletePrefix("user:*")` |
| 防雪崩 | 随机 TTL | `baseTTL + rand(0, 300s)` |
| 防穿透 | Bloom Filter | 不存在的数据也缓存空值短时间 |

## 22. 全文搜索模块 (`internal/services/search/`)

基于 Meilisearch 的统一全文搜索服务。

### 索引管理

```go
type SearchIndex struct {
    Name        string
    PrimaryKey  string
    SearchableAttrs []string
    FilterableAttrs []string
}
```

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

## 24. API 密钥管理模块 (`internal/services/apikey/`)

管理客户端/第三方系统的 API 访问密钥。

### 密钥模型

```go
type APIKey struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Key       string    `json:"key"`
    HashedKey string    `json:"-"`
    Scopes    []string  `json:"scopes"`
    AllowedIPs []string `json:"allowed_ips"`
    RateLimit int       `json:"rate_limit"`
    ExpiresAt *time.Time `json:"expires_at"`
    Status    string    `json:"status"`
    CreatedBy int64     `json:"created_by"`
}
```

### API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/apikey` | 密钥列表 |
| `POST` | `/api/v1/apikey` | 创建密钥 |
| `DELETE` | `/api/v1/apikey/:id` | 吊销密钥 |
| `GET` | `/api/v1/apikey/:id/logs` | 使用记录 |

## 25. 数据导入导出模块 (`internal/services/dataio/`)

提供统一的 CSV/Excel 导入导出能力。

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
    Name        string
    Enabled     bool
    Percentage  int
    UserIDs     []int64
    Platforms   []string
    Env         string
    Attributes  map[string]string
}
```

## 27. 模板引擎模块 (`internal/services/template/`)

集中管理 Go template，供通知/邮件/导出使用。

```go
type TemplateManager struct {
    // 模板来源: DB | 文件 | 配置
}

func (tm *TemplateManager) Render(name, version string, data any) (string, error)
func (tm *TemplateManager) Validate(name, content string) error
```

## 28. 规则引擎模块 (`internal/services/rules/`)

可配置的业务规则，避免硬编码。

```go
type Rule struct {
    Name       string
    Conditions []Condition
    Actions    []Action
    Priority   int
    Enabled    bool
}

type Condition struct {
    Field    string
    Operator string
    Value    any
}

type Action struct {
    Type   string
    Params map[string]any
}
```

## 29. 分布式限流模块 (`internal/services/ratelimit/`)

多副本部署时，在 Redis 中中心化限流，配合 middleware 使用。

```go
type RateLimitStrategy struct {
    Name     string
    Limit    int
    Window   time.Duration
    KeyFunc  func(ctx) string
}
```

支持算法：**滑动窗口**（默认）、**令牌桶**、**漏桶**。

## 30. MQTT 协议模块 (`internal/services/mqtt/`)

为 Home Assistant 及 IoT 设备提供 MQTT 客户端能力。

```go
type MQTTClient struct {
    client mqtt.Client
}

func (c *MQTTClient) Publish(topic string, payload []byte, qos byte) error
func (c *MQTTClient) Subscribe(topic string, handler MessageHandler) error
func (c *MQTTClient) Unsubscribe(topic string) error
```

## 31. 文件系统模块 (`internal/services/filesystem/`)

监听、扫描、索引本地文件。

```go
type Watcher struct {
    dirs   []string
    events chan FileEvent
}

type Scanner struct {
    extensions []string
    maxDepth   int
}

watcher.Watch("/data/obsidian")
entries, _ := scanner.Scan("/data/music")
eventbus.Publish("filesystem:file_created", event)
```

## 32. 标签系统模块 (`internal/services/tagging/`)

跨模块统一的标签/分类系统。

```go
type Tag struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Color     string `json:"color"`
    Group     string `json:"group"`
    CreatedAt time.Time `json:"created_at"`
}

func Attach(ctx, tagID, entityID int64, entityType string) error
func Detach(ctx, tagID, entityID int64, entityType string) error
func GetByTag(ctx, tagID, entityType string) ([]int64, error)
func GetByEntity(ctx, entityID int64, entityType string) ([]Tag, error)
```

## 33. 后台任务管理模块 (`internal/services/jobs/`)

在 scheduler 的触发基础上，增加长时间任务的进度上报、取消、重跑、日志查看。

```go
type Job struct {
    ID        string
    Type      string
    Progress  float64
    Status    string
    Log       []string
    StartedAt *time.Time
    EndedAt   *time.Time
}

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
    Format    string
    CoverArt  []byte
}

func ScanLibrary(ctx, rootPath string) (*ScanResult, error)
func ReadMetadata(ctx, filePath string) (*AudioFile, error)
func WriteTag(ctx, filePath string, tag *AudioFile) error
```

## 35. 报表/聚合引擎模块 (`internal/services/analytics/`)

为物品管理等业务模块提供统一的数据统计和报表生成能力。

```go
type Query struct {
    Table       string
    Metric      string
    GroupBy     []string
    TimeField   string
    TimeRange   TimeRange
    Filters     []Filter
}

type Report struct {
    Columns []string
    Rows    []map[string]any
    Total   int
}

func Aggregate(ctx, query Query) (*Report, error)
func ExportReport(ctx, query Query, format string) (io.Reader, error)
```

## 36. 插件系统模块 (`internal/services/plugin/`)

统一的插件注册和加载机制。

```go
type PluginManifest struct {
    Name        string
    Version     string
    Description string
    Routes      []Route
    Events      []string
    Tasks       []TaskDef
    Webhooks    []WebhookDef
}

type Plugin interface {
    Manifest() PluginManifest
    Init(cfg PluginConfig) error
    Close() error
}

// 插件加载方式
// 1. 内嵌：plugin.Register(&DifyPlugin{})
// 2. 外部：扫描 /plugins/*.so 动态加载
// 3. 远程：通过 HTTP 获取插件清单（预留）
```
