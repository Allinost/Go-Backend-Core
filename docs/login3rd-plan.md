# Login3rd 第三方应用登录管理模块 — 设计计划

## 1. 背景与问题

当前第三方平台采集器（JD、Taobao 等）的浏览器登录态依赖 chromedp UserDataDir 持久化到本地文件系统，存在以下问题：

| 问题 | 表现 |
|------|------|
| 脆弱性 | `%TEMP%` 目录随时可能被清理，浏览器配置损坏后 Chrome 进程崩溃 |
| 单机绑定 | 文件系统 session 无法在多实例/容器间共享 |
| 不透明 | 登录态何时过期、为何过期完全不可见 |
| 耦合深 | 浏览器生命周期与采集器强耦合，无法独立管理 |

**目标**：将所有第三方平台登录信息统一存储在数据库中，通过标准的接口管理登录、验证、刷新、通知。

## 2. 模块定位

```
zzz/login3rd/                     # Login3rd 模块
  ├── model.go                    # 数据模型
  ├── errors.go                   # 错误定义
  ├── handler.go                  # HTTP API
  ├── service.go                  # 业务逻辑
  ├── repository.go               # 数据库操作
  ├── module.go                   # 模块注册
  ├── provider/                   # 各平台登录提供者
  │   ├── provider.go             # Provider 接口
  │   ├── jd.go                   # 京东验证/刷新（仅复杂逻辑）
  │   └── taobao.go               # 淘宝（如需特化）
  ├── browser/                    # 浏览器辅助
  │   ├── login_flow.go           # 通用浏览器登录流（配置驱动）
  │   └── remote.go               # 远程浏览器会话管理
  ├── scheduler/
  │   └── session_check.go        # 定时校验调度器
  └── static/                     # 远程浏览器 Web 页面
      └── remote.html
```

## 3. 数据库表设计（2 张表）

### 3.1 第三方平台注册表（预置元数据 + 登录配置）

```sql
CREATE TABLE login3rd_platforms (
    id              VARCHAR(64)  PRIMARY KEY,        -- jd / taobao / pdd / _1688 / tjupt
    name            VARCHAR(128) NOT NULL,            -- 京东 / 淘宝 / ...
    category        VARCHAR(32)  NOT NULL,            -- shopping / pt / sign
    login_type      VARCHAR(32)  NOT NULL DEFAULT 'browser',  -- browser / oauth / qrcode
    has_auto        BOOLEAN      NOT NULL DEFAULT FALSE,       -- 支持自动填账号密码?
    login_url       TEXT,                              -- 登录页 URL
    home_url        TEXT,                              -- 登录后首页（提取 cookie 用）
    form_config     JSON,                              -- 自动填表单配置
    icon_url        TEXT,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

--- form_config JSON 结构：
{
  "tab_selector":       ".login-tab",             -- 切换到"账号密码"tab（JD 需要）
  "username_selector":  "#username",              -- 账号输入框
  "password_selector":  "#password",              -- 密码输入框
  "submit_selector":    "#loginBtn",              -- 登录按钮
  "captcha_selector":   "#captcha_img",           -- 验证码元素（如有）
  "success_cookies":    ["pt_key","pt_pin"],      -- 通过这些 cookie 判定登录成功
  "success_url_prefix": "https://home.jd.com",    -- 或通过 URL 判定成功
  "login_wait_selector":".qr-code-img",           -- 等待出现的元素（如二维码）
  "max_auto_retry":     3                         -- 自动填失败重试次数
}
```

预置数据：

| id | name | category | login_type | has_auto | login_url | home_url | form_config |
|----|------|----------|-----------|----------|-----------|----------|------------|
| jd | 京东 | shopping | browser | true | `https://plogin.m.jd.com/login` | `https://home.m.jd.com` | `{tab_selector, username_selector, ...}` |
| taobao | 淘宝 | shopping | browser | false | `https://login.taobao.com` | `https://www.taobao.com` | null |
| pdd | 拼多多 | shopping | browser | false | `https://mms.pinduoduo.com/login` | `https://mms.pinduoduo.com` | null |
| _1688 | 1688 | shopping | browser | true | `https://login.1688.com` | `https://www.1688.com` | `{...}` |
| tjupt | TJUPT | pt | browser | true | `https://tjupt.org/login.php` | `https://tjupt.org` | `{...}` |

> **核心价值**：新增平台只需 INSERT 一行，无需写代码。需要特化逻辑再单独写 provider。

### 3.2 第三方账号表

```sql
CREATE TABLE login3rd_accounts (
    id                   BIGINT       AUTO_INCREMENT PRIMARY KEY,
    platform_id          VARCHAR(64)  NOT NULL,          -- FK → login3rd_platforms.id
    label                VARCHAR(128) NOT NULL,           -- 自定义名称

    -- 凭证（AES-256-GCM 加密存储）
    username             VARCHAR(255),
    encrypted_password   TEXT,
    encrypted_token      TEXT,

    -- Webhook 通知
    webhook_ids          JSON,                            -- ["wh_xxx","wh_yyy"]

    -- Session（当前登录态）
    session_data         JSON,                            -- {cookies:[...], headers:{...}}
    login_method         VARCHAR(32),                     -- auto / remote / manual
    session_expires_at   TIMESTAMP,
    last_verified_at     TIMESTAMP,
    verified_status      VARCHAR(32)  DEFAULT 'unknown',  -- valid / expired / unknown

    -- 状态
    status               VARCHAR(32)  NOT NULL DEFAULT 'active',
    last_login_at        TIMESTAMP,
    last_error           TEXT,
    created_at           TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (platform_id) REFERENCES login3rd_platforms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 4. 远程浏览器 —— webhook 点击即开 Chrome

这是满配版的核心体验：

```
webhook 收到:
  ┌─────────────────────────────────────────┐
  │ 🔑 京东登录已过期                        │
  │ 账号: 我的京东主号                       │
  │ 点击下方链接打开浏览器完成登录:           │
  │ 👉 http://myserver:29090/login3rd/remote │
  │     /session/a1b2c3d4                   │
  │ 链接 5 分钟后失效                         │
  └─────────────────────────────────────────┘

用户点击链接 → 浏览器打开 → 加载 remote.html
  │
  ├─ 页面建立 WebSocket 连接到服务端
  ├─ 服务端分配一个临时浏览器会话（browserless/chrome）
  │
  ├─ 服务端控制 Chrome:
  │   ├─ Navigate(platform.login_url)
  │   ├─ 若 form_config 存在 → 自动填账号密码
  │   ├─ 等待二维码/验证码出现（截图推送到页面）
  │   └─ 轮询检查 success_cookies / success_url_prefix
  │
  ├─ 用户看到实时截图:
  │   ├─ 二维码 → 手机扫码
  │   ├─ 验证码 → 在页面上输入
  │   └─ 滑块 → 远程操作或用户介入
  │
  ├─ 检测到登录成功:
  │   ├─ 提取 cookies → session_data
  │   ├─ 更新 DB accounts 行
  │   ├─ 关闭浏览器会话
  │   └─ 页面显示 "✅ 登录成功"
  │
  └─ 用户关闭页面
```

### 技术实现

**服务端 → `browser/remote.go`**：
- 维护一个临时浏览器会话池（基于 `browserless/chrome`）
- `CreateSession(accountID, platform)` → 分配唯一 token，启动远程 Chrome
- `GetSession(token)` → WebSocket URL
- `DestroySession(token)` → 清理

**前端 → `static/remote.html`**：
- 纯静态页面（无框架依赖）
- 通过 WebSocket 接收服务端推送的截图
- 用户输入（验证码）通过 WebSocket 发回服务端
- 使用 CDP `Page.captureScreenshot` 推送帧（1-2fps 即可）

**交互方式（验证码输入）**：
```
页面: [实时截图区域]
        ┌─────────────────────┐
        │  京东登录页截图      │
        │  ┌─────────────┐   │
        │  │ 验证码:      │   │
        │  │ [输入框___]  │   │
        │  └─────────────┘   │
        └─────────────────────┘
        [ 输入验证码: _________ ]  ← 用户在这里输入
        [    ✅ 确认发送    ]
```

## 5. Provider 接口（精简）

```go
package provider

// SessionData 序列化后的会话数据
type SessionData struct {
    Cookies   []Cookie          `json:"cookies,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
    Extra     map[string]any    `json:"extra,omitempty"`
    ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

type Cookie struct {
    Name     string    `json:"name"`
    Value    string    `json:"value"`
    Domain   string    `json:"domain,omitempty"`
    Path     string    `json:"path,omitempty"`
    Secure   bool      `json:"secure,omitempty"`
    HTTPOnly bool      `json:"http_only,omitempty"`
    Expires  time.Time `json:"expires,omitempty"`
}

// Provider 接口
type Provider interface {
    ID() string

    // AutoLogin 使用账号密码自动登录（只有 login3rd_platforms.has_auto=true 的才需要实现）
    AutoLogin(ctx context.Context, username, password string) (*SessionData, error)

    // Verify 验证 session 是否有效
    Verify(ctx context.Context, session *SessionData) (bool, error)

    // Refresh 尝试刷新 session（如 refresh_token）
    Refresh(ctx context.Context, session *SessionData) (*SessionData, error)
}
```

**关键变化**：`BrowserLogin()` 和 `GetLoginURL()` 不再需要每个平台实现。浏览器登录由通用 `browser/login_flow.go` 驱动，流程完全由 `platform.login_url` + `form_config` 控制。

## 6. API 设计

```
# 平台
GET    /api/v1/login3rd/platforms                       # 列出所有平台

# 账号 CRUD
POST   /api/v1/login3rd/accounts                        # 新增账号
GET    /api/v1/login3rd/accounts                        # 账号列表
GET    /api/v1/login3rd/accounts/:id                    # 账号详情
PUT    /api/v1/login3rd/accounts/:id                    # 更新账号
DELETE /api/v1/login3rd/accounts/:id                    # 删除账号

# 登录
POST   /api/v1/login3rd/accounts/:id/login              # 服务端自动登录（不依赖用户）
POST   /api/v1/login3rd/accounts/:id/remote-login       # 创建远程浏览器会话，返回 token
GET    /api/v1/login3rd/accounts/:id/status             # 查看登录状态

# 远程浏览器 WebSocket
GET    /api/v1/login3rd/remote/session/:token           # 远程浏览器页面
WS     /api/v1/login3rd/remote/session/:token/ws        # WebSocket 通道

# 操作
POST   /api/v1/login3rd/accounts/:id/refresh            # 刷新 session
POST   /api/v1/login3rd/accounts/:id/logout             # 登出（清除 session）
```

## 7. 核心流程

### 7.1 远程登录（webhook 触发的完整链路）

```
Session 过期 → Scheduler 检测到
  │
  ├─ Refresh/AutoLogin 尝试 → 失败
  │
  ├─ Service.CreateRemoteSession():
  │   ├─ 生成唯一 token
  │   ├─ 通过 chromedp.NewRemoteAllocator 连接 browserless/chrome
  │   ├─ Navigate(platform.login_url)
  │   ├─ 若有 form_config 且平台 has_auto:
  │   │   ├─ 点击 tab_selector（切换到账号密码模式）
  │   │   ├─ 填入 username / password
  │   │   └─ 点击 submit_selector
  │   ├─ 截图当前状态 → 缓存最新截图
  │   └─ 返回 session token
  │
  ├─ Webhook 发送消息:
  │   "京东登录已过期，点击链接在浏览器中完成登录:
  │    http://host:29090/login3rd/remote/session/{token}"
  │
  ├─ 用户点击链接 → remote.html
  │   ├─ WebSocket 连接 → 服务端推送截图
  │   ├─ 用户扫码/输入验证码 → 通过 WebSocket 发送指令
  │   └─ 页面显示登录进度
  │
  ├─ Service 轮询检测登录成功:
  │   ├─ 检查 document.cookie 是否包含 success_cookies
  │   └─ 或检查当前 URL 是否匹配 success_url_prefix
  │
  ├─ 成功:
  │   ├─ 提取所有 cookies → SessionData
  │   ├─ 更新 accounts.session_data
  │   ├─ 通知前端 "✅ 登录成功"
  │   ├─ 30s 后自动关闭浏览器会话
  │   └─ 结束
  │
  └─ 5 分钟超时:
      ├─ 关闭浏览器会话
      ├─ token 失效
      ├─ 通知前端 "⏰ 链接已过期，请重新触发登录"
      └─ 结束
```

### 7.2 采集器使用 session

```
JDCollector.FetchPrice(url)
    │
    ├─ Login3rd.GetValidAccount("jd") → DB 查 accounts
    │   ├─ session_data → cookies
    │   └─ 注入到 chromedp 上下文
    │
    ├─ 若 session 有效:
    │   ├─ 导航到商品页 → 采集
    │   └─ 返回
    │
    ├─ 若 session 过期:
    │   ├─ 尝试 AutoLogin（若有凭证）
    │   ├─ 成功 → 更新 DB → 重试采集
    │   └─ 失败 → 触发远程登录 → 发 webhook
    │
    └─ 返回结果
```

### 7.3 Session 健康检查

```
Scheduler（每小时）
    │
    └─ 遍历 verified_status != 'valid' 的账号
        ├─ Provider.Verify(session_data)
        ├─ 有效 → 更新 timestamp
        ├─ 过期:
        │   ├─ 尝试 Provider.Refresh() / AutoLogin()
        │   ├─ 成功 → 更新 session_data
        │   └─ 失败:
        │       ├─ status = expired
        │       ├─ 创建远程浏览器会话（预备好登录页）
        │       ├─ 遍历 webhook_ids → 发 webhook（含远程浏览器的链接）
        │       └─ 记录到 webhook_delivery_logs
        └─ 继续下一个
```

## 8. 远程浏览器会话管理

```
Session 生命周期:
  ┌─────────┐    ┌──────────┐    ┌───────────┐    ┌──────────┐
  │ 创建    │───→│ 等待用户  │───→│ 登录完成  │───→│ 销毁     │
  │ (分配  │    │ (推截图)  │    │ (提取    │    │ (清理    │
  │  token) │    │          │    │  cookie)  │    │  资源)    │
  └─────────┘    └──────────┘    └───────────┘    └──────────┘
                        │
                        └──→ 超时 5 分钟 → 销毁

限制: 同时最多 3 个活跃远程会话（防止浏览器容器资源耗尽）
清理: 无论成功/超时，会话结束后立即关闭 Chrome 标签页
```

## 9. 安全性

- 密码/token AES-256-GCM 加密，密钥来自服务配置
- session_data 中 cookie value 加密存储
- 远程浏览器链接基于 UUID token，5 分钟过期
- 每个 token 只能使用一次（创建后可访问一次，断开即失效）
- API 需要认证（绑定当前用户只能操作自己的账号）
- 远程浏览器进程中不保存任何文件（UserDataDir 用临时目录，会话结束即删）

## 10. 接入计划

```
Phase 1 (核心骨架)
  ├─ 数据库迁移（2 张表）
  ├─ model.go / repository.go / service.go / handler.go
  ├─ Provider 接口定义
  └─ HTTP API（平台列表 + 账号 CRUD）

Phase 2 (通用浏览器登录)
  ├─ browser/login_flow.go（配置驱动，支持远程 + 自动）
  ├─ browser/remote.go（会话管理）
  ├─ remote.html（前端页面 + WebSocket）
  └─ 远程登录 API + WebSocket endpoint

Phase 3 (JD Provider + 采集器对接)
  ├─ provider/jd.go（AutoLogin + Verify + Refresh）
  ├─ 采集器改为从 Login3rd 拿 cookie
  └─ 移除 jd.go 旧版 sync.Once + UserDataDir

Phase 4 (扩展平台)
  ├─ taobao / pdd / _1688 / tjupt provider
  └─ 配置数据 seed（INSERT platform 行）

Phase 5 (自动化运维)
  ├─ scheduler session_check.go
  ├─ webhook 通知模板
  └─ 全链路验证：session 过期 → webhook → 点链接 → 远程登录 → 恢复采集
```

## 11. 浏览器策略对比

| 现在 | 改后 |
|------|------|
| sync.Once 持久浏览器进程 | 无状态，用完即释放 |
| UserDataDir 固定目录持久化 cookie 文件 | 临时目录，session 存 DB |
| 浏览器与采集器强耦合 | 浏览器仅登录/采集时临时启动 |
| session 不可见，过期无通知 | session 存 DB，过期发 webhook |
| 登录失败原因模糊 | 精确区分网络 / 登录过期 / 反爬 |
| 登录需要手动写 provider 代码 | 配置驱动，INSERT 一行即可新增平台 |
| 无法远程交互 | webhook 点链接 → 远程 Chrome 实时操作 |
