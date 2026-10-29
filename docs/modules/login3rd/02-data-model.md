# 第三方登录数据模型

## 1. 数据库

- **数据库**: MySQL `main` 连接池（`database.DB.MySQL["main"]`）
- **表名**: `login3rd_platforms` / `login3rd_accounts`（2 张）
- **建表方式**: 模块启动时 `initTables()`（`module.go:159-210`）幂等 `CREATE TABLE IF NOT EXISTS`，随后 `repo.SeedPlatforms` 写入 7 个平台
- **ORM**: 不用 GORM；原生 `database/sql` + 手写 SQL，model 用 `db:`/`json:` tag
- **不走 `migrations/` 目录**（与 `ia` 模块一致）

---

## 2. 表结构

### 2.1 `login3rd_platforms` — 平台（`module.go:162-173`）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(64) PK | 平台 ID：`jd` / `qixian` / `jdm` / `taobao` / `pdd` / `_1688` / `tjupt` |
| name | VARCHAR | 平台名称 |
| category | VARCHAR | 分类：`shopping` / `pt` |
| login_type | VARCHAR | 登录方式：`browser` / `oauth` / `qrcode` |
| has_auto | BOOL | 是否支持自动登录 |
| login_url | VARCHAR | 登录页地址 |
| home_url | VARCHAR | 登录成功跳转首页（判定用） |
| form_config | JSON | 表单选择器配置（`FormConfig`） |
| icon_url | VARCHAR | 图标 |

**`FormConfig`**（`model.go:24-34`，存于 `form_config` JSON 列）：

| 字段 | 用途 |
|------|------|
| `TabSelector` | 登录方式 Tab（账号密码）选择器 |
| `UsernameSelector` / `PasswordSelector` | 用户名/密码输入框 |
| `SubmitSelector` / `CaptchaSelector` | 登录按钮 / 验证码输入框 |
| `SuccessCookies []string` | **登录成功判定 Cookie**（全部出现即成功） |
| `SuccessURLPrefix` | 登录成功 URL 前缀判定 |
| `LoginWaitSelector` | 登录后等待出现的元素 |
| `MaxAutoRetry` | 自动填表最大重试次数 |

### 2.2 `login3rd_accounts` — 账号（`module.go:175-194`，含外键 → platforms.id）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR PK | UUID |
| platform_id | VARCHAR FK | 所属平台 |
| label / username | VARCHAR | 备注名 / 登录用户名 |
| **encrypted_password** | TEXT | **AES-256-GCM 加密**的密码（`json:"-"`） |
| **encrypted_token** | TEXT | 加密的备用 token（`json:"-"`） |
| webhook_ids | JSON | 告警接收的 webhook ID 数组 |
| session_data | JSON | 会话（`SessionData`，**明文 Cookie JSON**） |
| login_method | VARCHAR | `auto` / `remote` / `manual` |
| session_expires_at | DATETIME | 会话过期时间 |
| last_verified_at | DATETIME | 最近验证时间 |
| verified_status | VARCHAR | `valid` / `expired` / `unknown` |
| status | VARCHAR | `active` / `expired` / `disabled` |
| last_login_at / last_error | — | 最近登录时间 / 最近错误 |

**`SessionData`**（`model.go:68-73`，存于 `session_data` JSON 列）：

```go
type SessionData struct {
    Cookies   []SessionCookie `json:"cookies"`    // Name/Value/Domain/Path/Secure/HTTPOnly/Expires
    Headers   map[string]string `json:"headers,omitempty"`
    Extra     map[string]string `json:"extra,omitempty"`
    ExpiresAt *time.Time       `json:"expires_at,omitempty"`
}
```

> ⚠️ `session_data` 为**明文 Cookie**存储（仅密码/Token 加密），依赖数据库访问控制。

---

## 3. ER 概览

```
login3rd_platforms (7, seed)
        │ 1
        │
        │ N   外键 platform_id
login3rd_accounts
   ├── session_data JSON（Cookie 会话）
   ├── encrypted_password / encrypted_token（AES-256-GCM）
   └── webhook_ids JSON ──► webhook 模块（告警投递）
```

---

## 4. Go 数据模型

实体定义在 `model.go`，tag 为 `db:"..."` + `json:"..."`，**无 gorm tag**：

| struct | 行号 | 说明 |
|--------|------|------|
| `Platform` | 10-21 | 平台，`FormCfg *FormConfig` 映射 JSON 列 |
| `FormConfig` | 24-34 | 表单选择器与成功判定配置 |
| `Account` | 37-65 | 账号；`EncryptedPwd`/`EncryptedToken` 打 `json:"-"` |
| `SessionData` | 68-73 | 会话容器 |
| `SessionCookie` | 76-84 | 单条 Cookie |

---

## 5. DTO

```go
// 创建账号（model.go:88-95）— password 为明文，Service 层加密
type CreateAccountReq struct {
    PlatformID string   `json:"platform_id" binding:"required"`
    Label      string   `json:"label" binding:"required"`
    Username   string   `json:"username"`
    Password   string   `json:"password"`
    Token      string   `json:"token"`
    WebhookIDs []string `json:"webhook_ids"`
}

// 更新账号（model.go:97-104）— 指针字段表示部分更新
type UpdateAccountReq struct {
    Label    *string  `json:"label"`
    Username *string  `json:"username"`
    Password *string  `json:"password"`   // 重新加密
    Token    *string  `json:"token"`
    Status   *string  `json:"status"`
    WebhookIDs []string `json:"webhook_ids"`
}

// 账号列表查询（model.go:106-125）
// page/page_size/platform_id/status/keyword + Normalize()（默认 1/20，上限 100）+ Offset()

// 账号状态响应（model.go:128-134）
type AccountStatusResp struct {
    Account         Account   `json:"account"`
    Platform        Platform  `json:"platform"`
    SessionValid    bool      `json:"session_valid"`
    SessionExpiresIn int64    `json:"session_expires_in"`  // 剩余秒数
    CanAutoLogin    bool      `json:"can_auto_login"`
}

// 通用分页响应（model.go:137-141）
type PaginatedResp[T any] struct {
    Items   []T  `json:"items"`
    HasMore bool `json:"has_more"`
    Total   int  `json:"total"`
}
```
