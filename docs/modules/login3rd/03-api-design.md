# 第三方登录 API 设计

## 1. 统一规范

- 路由前缀：`/api/v1/login3rd`（挂载于 `module.go:133-156`）
- 统一响应格式同项目规范（`response.Response`：`code` / `message` / `data`）
- 分页响应结构：`{ "items": [...], "has_more": bool, "total": int }`
- 错误码段：`1001–1014`（见第 5 节）
- **鉴权**：路由组未挂 JWT 中间件（主引擎仅 Logger/CORS/Recovery/RateLimit），依赖网络层隔离

---

## 2. 平台 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/platforms` | 列出全部支持的平台（seed 的 7 个） |

---

## 3. 账号 API

### 3.1 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts` | 创建账号（密码明文提交，**服务端 AES-256-GCM 加密入库**） |
| GET | `/accounts` | 分页/筛选/关键词查询（`page`/`page_size`/`platform_id`/`status`/`keyword`） |
| GET | `/accounts/:id` | 账号详情（密文字段 `json:"-"` 不回传） |
| PUT | `/accounts/:id` | 局部更新（传 password 则重新加密） |
| DELETE | `/accounts/:id` | 删除账号 |

### 3.2 登录与会话

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts/:id/login` | **自动登录**：解密密码 → `Provider.AutoLogin` → 写 session |
| POST | `/accounts/:id/remote-login` | **创建远程浏览器会话**：返回 `token` + 登录页 URL，goroutine 跑 LoginFlow |
| GET | `/accounts/:id/status` | 会话状态：`session_valid` / `session_expires_in` / `can_auto_login` |
| POST | `/accounts/:id/verify` | `Provider.Verify` 校验 Cookie 是否有效，写回 `verified_status` |
| POST | `/accounts/:id/refresh` | `Provider.Refresh` 续期（京东不支持则原样返回） |
| POST | `/accounts/:id/logout` | 清空 session、状态重置 |

### 3.3 远程登录页面

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/remote/session/:token` | 返回内嵌 HTML 页面（远程/本地两种模式） |
| GET | `/remote/session/:token/status` | 本地模式轮询 `{done, success, error}` |
| GET | `/remote/session/:token/ws` | **WebSocket**：推截图（2s/帧），收 `input`/`click`/`close` 指令驱动 chromedp |

> 远程会话 token 有效期 **5 分钟**（`browser/remote.go:163`）。

---

## 4. 典型交互流程

### 4.1 远程登录（京东等无自动登录平台）

```
1. POST /accounts/:id/remote-login
   → { "token": "xxxx", "url": "/api/v1/login3rd/remote/session/xxxx" }
2. 浏览器打开 url，建立 WS 连接
3. 服务端 chromedp 打开平台登录页，截图循环推送（本地模式则直接弹窗）
4. 用户在页面输入账号密码 → WS 下发 input/click 指令 → chromedp 填表/点击
5. LoginFlow 轮询判定成功（命中 SuccessCookies / SuccessURLPrefix）
6. 抓取 Cookie → 写入 account.session_data → WS 通知完成
```

### 4.2 会话健康检查（服务端定时）

```
scheduler (24h) → login3rd_session_check handler → RunSessionCheck
   verify → refresh → auto login → remote-login 链接 + webhook 告警
```

---

## 5. 业务错误码

类型 `Login3rdError{Code, Message, Err}`（`errors.go:6-10`），Handler 用 `errors.As` 识别并映射（`handler.go:459-472`）：

| 错误码 | 常量 | 说明 |
|--------|------|------|
| 1001 | `ErrCodePlatformNotFound` | 平台不存在 |
| 1002 | `ErrCodeAccountNotFound` | 账号不存在 |
| 1003 | `ErrCodeAccountDisabled` | 账号已禁用 |
| 1004 | `ErrCodeSessionExpired` | 登录会话已过期 |
| 1005 | `ErrCodeAutoLoginNotSupported` | 该平台不支持自动登录 |
| 1006 | `ErrCodeAutoLoginFailed` | 自动登录失败 |
| 1007 | `ErrCodeBrowserLoginFailed` | 浏览器登录失败 |
| 1008 | `ErrCodeSessionVerifyFailed` | 会话验证失败 |
| 1009 | `ErrCodeRemoteSessionCreateFailed` | 创建远程浏览器会话失败 |
| 1010 | `ErrCodeRemoteSessionNotFound` | 远程浏览器会话不存在 |
| 1011 | `ErrCodeRemoteSessionExpired` | 远程浏览器会话已过期 |
| 1012 | `ErrCodeWebhookSendFailed` | Webhook 发送失败 |
| 1013 | `ErrCodeEncryptFailed` | 加密失败 |
| 1014 | `ErrCodeDecryptFailed` | 解密失败 |

Provider 层另有独立错误：`provider.ErrNotSupported`（`provider/provider.go:43`）、`provider.ProviderError`。
