# 第三方登录模块架构概览

## 1. 模块定位

第三方登录模块（`login3rd`，代码目录 `zzz/login3rd`）统一管理多电商平台与 PT 站的**登录态（Cookie 会话）**，为上层采集模块（`shopchecker`）提供可用会话。

### 1.1 解决的问题

| 问题 | 方案 |
|------|------|
| 多平台登录态散落、易过期难维护 | 集中存 `zzz_login3rd_accounts`，统一 CRUD 与状态跟踪 |
| 手工反复登录效率低 | 三种登录方式：自动填表 / 远程浏览器人工登录 / 手动抓 Cookie |
| Cookie 静默过期导致采集失败 | 24h 会话健康检查降级链 + webhook 过期告警 |
| 凭据明文存储风险 | 密码 AES-256-GCM 加密入库，API 不回传密文 |
| 各平台登录协议差异大 | `Provider` 接口抽象（AutoLogin/Verify/Refresh），按平台实现 |

### 1.2 分层架构

```
HTTP / WS (/api/v1/login3rd/*)
        │
        ▼
handler.go          15 个 handler · 内嵌登录页 HTML · WebSocket 指令转发
        │
        ▼
service.go          账号 CRUD · 加解密 · 登录/验证/刷新/登出 · 远程会话编排
   ├─ crypto.go          AES-256-GCM 加解密
   ├─ session_check.go   RunSessionCheck 降级链（scheduler 驱动）
   ├─ session_check_handler.go  向 centralized scheduler 注册 handler
   └─ provider/          Provider 接口 + JD 实现（Verify 为主）
        │                 └─ browser/  chromedp 登录流 + 远程浏览器管理
        ▼
repository.go       原生 SQL · zzz_login3rd_platforms / zzz_login3rd_accounts
        │
        ▼
MySQL main 连接池

旁路依赖:
  webhook 模块 ── 会话过期/登录失败告警（Init 强依赖，必须先注册）
  scheduler 模块 ── 24h 会话检查任务（缺失仅 Warn，不致命）
  config ── crypto.master_key_hex（加密密钥）· net.chrome_remote_url（远程浏览器）
```

### 1.3 命名空间

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | `zzz/login3rd/` | `zzz/login3rd/service.go`、`provider/`、`browser/` |
| 表名 | `login3rd_{表名}` | `zzz_login3rd_platforms`、`zzz_login3rd_accounts` |
| 路由前缀 | `/api/v1/login3rd` | `/api/v1/login3rd/accounts` |
| 模块名 | `login3rd` | `modules.Register(&login3rd.Module{})`（`module.go:32`） |
| 错误码 | `1001–1014` | 1004 登录会话已过期 |
| scheduler 任务 | `login3rd_session_check` | 24h 间隔，Timeout 600s |

---

## 2. 核心能力

### 2.1 三种登录方式

| 方式 | 流程 | 适用 |
|------|------|------|
| **auto 自动登录** | 解密密码 → `Provider.AutoLogin` → 写 session | 支持填表的平台（_1688、tjupt） |
| **remote 远程登录** | 创建 token 会话（5min）→ chromedp 打开登录页 → WS 推截图，用户 `input/click` 指令驱动 → 抓 Cookie 回填 | 京东系、淘宝、拼多多等 |
| **manual 手动登录** | 用户自行登录后提交 Cookie | 无选择器配置的平台 |

**浏览器层连接策略**（`browser/remote.go`）：

- 配置 `net.chrome_remote_url` → 连远程 browserless/Chrome（`isLocal=false`，启动截图循环）
- 否则拉起本地 Chrome/Edge（固定 profile `%LOCALAPPDATA%\login3rd-chrome` 复用登录态、`--remote-debugging-port=9222`，先探测复用已运行实例）

### 2.2 会话健康检查降级链（`session_check.go:11-83`）

```
每轮取前 100 个账号（跳过 disabled/expired）
   │
   ├─ 1. VerifySession  有效 → 结束
   ├─ 2. RefreshSession 失效时刷新，verified_status=valid → 结束
   ├─ 3. AutoLogin      自动登录成功 → 结束
   └─ 4. CreateRemoteSession  生成 5 分钟远程登录链接
          账号标记 status=expired
          → webhook 告警（正文含远程登录链接）
```

由 centralized scheduler 的 `login3rd_session_check` 任务（24h 间隔）驱动；`session_check.go` 中旧的 `SessionChecker` 已弃用。

### 2.3 平台 seed 数据

| 平台 ID | 站点 | 自动登录 |
|---------|------|----------|
| `jd` / `qixian` / `jdm` | 京东 / 七鲜 / 京东移动版 | ❌（复用同一 JD Provider） |
| `taobao` | 淘宝 | ❌ |
| `pdd` | 拼多多 | ❌ |
| `_1688` | 1688 | ✅（含表单选择器） |
| `tjupt` | TJUPT（PT 站） | ✅ |

`SeedPlatforms` 使用 `ON DUPLICATE KEY UPDATE` 幂等更新（`repository.go:52-61`）。

### 2.4 跨模块服务

`GetService()` 暴露给 `shopchecker`（`zzz/shopchecker/service.go:39-44`、`collector/jd.go:513`、`collector/taobao.go:632`）：

| 方法 | 作用 |
|------|------|
| `MarkExpired` | 采集发现会话失效时同步状态 |
| `TouchSession` | 采集成功后会话延寿 |
| `NotifyLoginExpired` | 发送过期通知 |
| `GetProvider` / `GetDecryptedPassword` | 获取 Provider / 解密后的密码 |

---

## 3. 依赖关系

```
login3rd Module
  ├── internal/database                  # MySQL main 连接池（原生 *sql.DB）
  ├── internal/config                    # crypto.master_key_hex · net.chrome_remote_url
  ├── internal/modules                   # 模块注册表
  ├── internal/modules/webhook           # ⚠️ 强依赖（Init 失败即退出），必须先于 login3rd 注册
  ├── internal/services/scheduler        # 24h 会话检查任务（缺失仅 Warn）
  ├── internal/pkg/{logger,response,errors}
  ├── github.com/gin-gonic/gin           # 路由
  ├── github.com/chromedp/chromedp       # 浏览器自动化
  ├── github.com/gorilla/websocket       # 远程登录 WS
  └── github.com/google/uuid             # 主键与会话 token
```

**注册顺序约束**（`cmd/server/main.go`）：`webhook`(行 102) 必须在 `login3rd`(行 105) 之前注册，否则 `Init` 失败 panic。

---

## 4. 测试覆盖

⚠️ **整个 `zzz/login3rd/`（含 `provider/`、`browser/`）目前没有任何 `_test.go`**，是本模块的主要工程短板。建议优先补：

1. `crypto.go` 加解密往返（含错误密钥、篡改密文）
2. `session_check.go` 降级链各分支（mock Provider）
3. `provider/jd.go` 的 `Verify` 重定向判定逻辑
4. `service.go` 账号 CRUD 与加解密边界
