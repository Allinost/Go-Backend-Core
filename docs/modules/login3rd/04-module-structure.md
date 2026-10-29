# 第三方登录模块结构

## 1. 文件组织

代码目录：`zzz/login3rd/`（10 个主包文件 + 2 个 provider + 2 个 browser）

| 文件 | 职责 | 测试 |
|------|------|------|
| `module.go` | 模块入口：Name/Init/Close/RegisterRoutes、自动建表与平台 seed、scheduler 任务注册 | ❌ |
| `handler.go` | HTTP 层：15 个 handler、统一错误处理、远程/本地登录页内嵌 HTML、WebSocket 指令转发 | ❌ |
| `service.go` | 业务层：账号 CRUD、加密、登录/验证/刷新/登出、webhook 通知、远程会话编排、Provider 注册 | ❌ |
| `repository.go` | 数据访问：原生 SQL（平台/账号 CRUD、session 更新、状态标记）、seed 数据 | ❌ |
| `model.go` | 数据结构：Platform/FormConfig/Account/SessionData/SessionCookie + 请求/响应 DTO | ❌ |
| `crypto.go` | AES-256-GCM 加解密（hex 编码输出） | ❌ |
| `errors.go` | `Login3rdError` 类型、错误码 1001–1014、构造函数 | ❌ |
| `session_check.go` | 会话健康检查降级链 `RunSessionCheck`（旧 `SessionChecker` 已弃用） | ❌ |
| `session_check_handler.go` | `init()` 内向 centralized scheduler 注册 handler `login3rd_session_check` | ❌ |
| `provider/provider.go` | `Provider` 接口 + 通用 `SessionData`/`Cookie` + `ErrNotSupported` | ❌ |
| `provider/jd.go` | 京东 Provider（Verify 为主）+ Cookie 注入辅助 | ❌ |
| `browser/login_flow.go` | chromedp 通用登录流：导航→填表→等待→抓 Cookie；本地/远程执行入口 | ❌ |
| `browser/remote.go` | 远程浏览器会话管理 `RemoteManager`：token 会话、本地 Chrome 拉起/连接、截图循环 | ❌ |

> ⚠️ **全目录无任何 `_test.go`**（见 [01-architecture-overview.md 第 4 节](01-architecture-overview.md#4-测试覆盖)）。

---

## 2. 分层实现

```
handler.go                  service.go                    repository.go
  ListPlatforms   ─────────► ListPlatforms   ────────────► ListPlatforms / GetPlatform
  CreateAccount   ─────────► CreateAccount                  CreateAccount
                             encryptPassword (crypto.go)    CreateAccount(密文)
  AutoLogin       ─────────► AutoLogin                      GetAccount / GetPlatform
                             decryptPassword                UpdateSession
                             provider.AutoLogin
  VerifySession   ─────────► VerifySession   ──────────────► UpdateVerifiedStatus
                             provider.Verify
  CreateRemoteSession ─────► CreateRemoteSession            GetPlatform / UpdateSession
                             RemoteManager.CreateSession
                             LoginFlow.Execute (goroutine)
  RunSessionCheck ─────────► (scheduler 24h 驱动)
```

核心方法索引：

- **service.go**：`ListPlatforms`:62、`CreateAccount`:69（加密入库）、`GetAccount`:98、`UpdateAccount`:115（重新加密）、`DeleteAccount`:159、`GetAccountStatus`:164、`AutoLogin`:195、`CreateRemoteSession`:246、`VerifySession`:321、`RefreshSession`:353、`Logout`:385；跨模块方法 `SendWebhookNotification`:403、`MarkExpired`:434、`NotifyLoginExpired`:441、`TouchSession`:499、`GetProvider`:528、`GetDecryptedPassword`:533
- **repository.go**：`ListPlatforms`:24、`GetPlatform`:36、`SeedPlatforms`:44、`GetAccount`:173、`CreateAccount`:149、`ListAccounts`:199、`UpdateAccount`:239、`DeleteAccount`:258、`UpdateSession`:264、`TouchSession`:289、`MarkExpired`:299、`UpdateVerifiedStatus`:275

---

## 3. 模块生命周期（`module.go`）

```go
func (m *Module) Name() string { return "login3rd" }   // module.go:32

Init(cfg):                                             // module.go:37-74
  1. 取 MySQL main 连接池（nil 则报错）
  2. NewRepository(pool.DB)
  3. modules.Get("webhook") → GetService()   // ⚠️ 强依赖，缺失则 Init 失败
  4. NewService(repo, whSvc)
  5. provider.NewJDCollector()，以 jd / qixian / jdm 三个平台 ID 注册同一实例
  6. initTables(pool.DB)  // 2 张表 DDL（module.go:159-210）
     repo.SeedPlatforms   // 7 个平台，ON DUPLICATE KEY UPDATE 幂等
  7. NewHandler(m.svc)
  8. sessionCheckSvc = m.svc            // 供 scheduler handler 使用
  9. registerSessionCheckTask()         // 24h 任务 login3rd_session_check（scheduler 缺失仅 Warn）

Close(): m.sch.Stop()                  // module.go:77-82（m.sch 未赋值，实际为空操作）
```

**注册位置**：`cmd/server/main.go:105`；**顺序约束**：`webhook`(行 102) 必须先注册。

---

## 4. Provider 接口（`provider/provider.go:26-40`）

```go
type Provider interface {
    ID() string                                                                         // 平台标识
    AutoLogin(ctx context.Context, username, password string) (*SessionData, error)     // 账号密码自动登录
    Verify(ctx context.Context, session *SessionData) (bool, error)                     // 验证 session
    Refresh(ctx context.Context, session *SessionData) (*SessionData, error)            // 刷新 session
}
```

- 通用结构：`SessionData`（Cookies/Headers/Extra/ExpiresAt unix）、`Cookie`（Name/Value/Domain/Path/Secure/HTTPOnly/Expires）
- 不支持的操作返回 `ErrNotSupported`（`ProviderError` 包装）

### JD 实现（`provider/jd.go`）

| 方法 | 行为 |
|------|------|
| `AutoLogin` | **直接返回 `ErrNotSupported`**（京东无公开密码登录 API，必须走远程登录） |
| `Verify` | 带 Cookie GET `https://home.m.jd.com`（UA 伪装），若重定向到含 `plogin`/`passport` 的地址 → 失效，否则有效（`:42-78`） |
| `Refresh` | 返回 `ErrNotSupported`（京东不支持 refresh token） |
| `InjectCookiesToHTTP` | 按 Domain 后缀匹配注入任意 `http.Request`（供 shopchecker 采集器复用） |

> `jd`/`qixian`/`jdm` 三平台共用同一 `JDCollector` 实例（`module.go:56-59`）。

---

## 5. 浏览器层（`browser/`，chromedp）

### 5.1 登录流 `login_flow.go`

`LoginFlow{LoginURL, HomeURL, FormCfg}` 的 `Execute(ctx, browserCtx, username, password)`（`:35-105`）：

1. 导航登录页 + `WaitReady("body")` + 2s 等待（30s 超时）
2. 有凭据且有 FormCfg 时自动填表：点 Tab → Clear/SendKeys 用户名密码（10s 超时）→ 无验证码选择器时自动点登录；**任一步失败降级为手动登录**
3. `waitLogin` 轮询（`:142-201`）：120s 超时、2s tick 抓 Cookie，命中 `SuccessCookies`（全含）或 `SuccessURLPrefix` 即判成功
4. `extractCookies`（`:204-225`）经 CDP `network.GetCookies()` 转 `provider.Cookie`

执行入口：`ExecuteWithChrome(chromePath)`（临时 profile + 本地 Chrome，NoSandbox、禁 AutomationControlled）、`ExecuteRemote(remoteURL)`（`chromedp.NewRemoteAllocator` 连远程 browserless）。

### 5.2 会话管理 `remote.go`

| 组件 | 职责 |
|------|------|
| `RemoteSession`（`:21-35`） | Token、AccountID、`ExpiresAt`（**5 分钟**）、BrowserCtx、截图缓存、Done 通道、Result、IsLocal |
| `RemoteManager`（`:45-56`） | 会话 map + remoteURL；`GetSession`/`DestroySession`/`MarkDone`/`CleanupExpired` |
| 连接策略（`:133-183`） | 配置 `net.chrome_remote_url` → 远程浏览器（有截图循环）；否则 `findLocalBrowser()`（Chrome/Edge 多路径，Windows+Linux）→ `launchLocalChrome`（固定 profile `%LOCALAPPDATA%\login3rd-chrome` 复用登录态、`--remote-debugging-port=9222`、先 TCP 探测复用已运行实例、15s 就绪轮询） |
| `screenshotLoop`（`:234-257`） | 每 2s `FullScreenshot` 质量 80，供 WS 推送 |

**职责划分**：`login_flow.go` 负责"怎么登录"（导航/填表/判成功），`remote.go` 负责"浏览器从哪来、会话怎么管"。handler 的 WS（`handler.go:379-455`）把用户 `input/click` 指令直接转成 `chromedp.SendKeys/Click`。

---

## 6. 加密与安全（`crypto.go`）

- **算法**：AES-256-GCM，随机 nonce 前置于密文，输出 **hex 编码**（加密 `:21-26`，解密 `:43-48`）
- **加密对象**：账号明文密码、备用 token（`CreateAccount`/`UpdateAccount` 加密，`AutoLogin`/远程会话凭据注入/`GetDecryptedPassword` 解密）
- **密钥**：`config.Get().Crypto.MasterKeyHex` hex 解码（`service.go:27-31`）；**为空回退 32 字节零密钥（仅非生产）**（`:32-34`）
- **未加密**：`session_data`（明文 Cookie JSON）、`webhook_ids`
- **API 脱敏**：`EncryptedPwd`/`EncryptedToken` 打 `json:"-"` 不回传

### 安全观察项

1. 模块路由未挂 JWT 中间件，依赖网络层隔离（调试端口仅内网）
2. `wsUpgrader.CheckOrigin` 恒返回 `true`（`handler.go:19-21`）
3. 远程会话 token 仅 8 位 uuid（`remote.go:134`），有效期 5 分钟
4. `session_check.go:110` 硬编码 webhook 回调 host `https://et.hailong.site:29090`
5. 生产环境务必配置 `crypto.master_key_hex`（否则使用零密钥）

---

## 7. 配置项

| 配置 | 定义 | 用途 |
|------|------|------|
| `crypto.master_key_hex` | `internal/config/config.go:40-44` | 登录 3rd 加密主密钥 |
| `net.chrome_remote_url` | `config.go:46-50`（示例 `ws://chrome:29091`） | 远程浏览器地址，空则回落本地 Chrome |
| `scheduler.*` | `config.yaml:89-94` | 间接影响 24h 会话检查任务 |

> `config.yaml` 的 `auth:` 段仅含 `jwt_secret`/`jwt_expire`，与本模块无关；`crypto`/`net` 段需经环境变量 `APP_*` 或 Docker Secret 注入。

---

## 8. 已知限制与改进建议

1. **零测试覆盖** — 优先补 crypto 往返、降级链分支、JD Verify 判定
2. `Close()` 中 `m.sch` 从未赋值，实际为空操作（会话检查已改由 centralized scheduler 驱动，建议清理死代码与旧 `SessionChecker`）
3. DDL 内联在 `module.go`，未纳入 `migrations/` 版本化
4. `session_data` 明文存储 Cookie，建议评估加密或限制 DB 访问
5. `getHost()` 硬编码外部地址，应改为从配置读取
6. 单实例假设（远程会话存内存 map），多实例部署需外置会话存储
