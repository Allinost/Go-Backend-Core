# 社交登录接入方案

支持六种第三方 OAuth 登录：微信、飞书、QQ、Apple ID、华为账号、荣耀账号。

支持一个用户绑定多个第三方账号，且同平台可绑定多个账号（如同时绑定个人微信和企业微信）。

---

## 1. 微信登录

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | 微信开放平台 (open.weixin.qq.com) + 微信公众平台 (mp.weixin.qq.com) |
| 协议 | OAuth 2.0 (Authorization Code) |
| 文档 | https://developers.weixin.qq.com/doc/ |

### 场景细分

#### ① 移动端（Android/iOS）— 微信 App 授权

```
App → 调起微信 SDK → 用户授权 → 返回 code → 后端 /api/v1/auth/wechat/callback
```

- 接入前提：在微信开放平台注册应用，获取 AppID + AppSecret
- SDK 返回 code，后端调用 `https://api.weixin.qq.com/sns/oauth2/access_token` 换 token
- UnionID：同一开放平台账号下所有应用 UnionID 一致，可用于打通多端用户

#### ② 微信小程序 — code 直换登录态

```
小程序 wx.login() → code → POST /api/v1/auth/miniapp/login
```

- 后端调用 `https://api.weixin.qq.com/sns/jscode2session` → openid + session_key + unionid
- 无需手动的 OAuth 授权页面跳转
- unionid 需要公众号绑定开放平台才能获取

#### ③ Web — 微信扫码登录

```
Web 显示二维码 → 用户扫码 → 回调 code → /api/v1/auth/wechat/callback
```

- 接入前提：微信开放平台申请网站应用
- 使用微信开放平台提供的扫码登录组件

#### ④ 手机号获取（小程序）

```
小程序: <button open-type="getPhoneNumber"> → POST /api/v1/auth/miniapp/phone
```

- 需企业认证的小程序
- 后端用 session_key 解密加密数据获取手机号

### 配置项

```yaml
auth:
  wechat:
    app_id: "wx_xxxxxxxx"
    app_secret: "xxxxxx"
    mini_program_app_id: "wx_yyyyyyy"    # 小程序 AppID
    mini_program_app_secret: "yyyyyy"
    open_platform_token: "zzzzz"         # 开放平台消息校验 Token
    open_platform_aes_key: "aaaaa"       # 开放平台 AES Key（可选）
```

---

## 2. 飞书登录

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | 飞书开放平台 (open.feishu.cn) |
| 协议 | OAuth 2.0 (Authorization Code) |
| SDK | larksuite/oapi-sdk-go |
| 文档 | https://open.feishu.cn/document/ |

### 接入流程

```
Web/App → 跳转飞书授权 → 用户确认 → 回调 code → 后端交换 token → 获取用户信息
```

### 关键 API

| 步骤 | API | 说明 |
|------|-----|------|
| 构造授权 URL | `connect.qwq28.com/connect/oauth2/authorize` | 携带 redirect_uri + state |
| code → token | `POST /open-apis/authen/v1/access_token` | 获取 access_token + refresh_token |
| 获取用户信息 | `GET /open-apis/authen/v1/user_info` | 获取 name, avatar, email, mobile |

### 特性

- **企业账号通信录同步**: 可获取用户所属企业/部门信息（需企业授权）
- **邮箱必填**: 飞书用户一定有邮箱，可用于自动匹配已有账号
- **手机号**: 企业通讯录可见范围内可获取

### 配置项

```yaml
auth:
  feishu:
    app_id: "cli_xxxxxxxxx"
    app_secret: "xxxxxx"
    redirect_uri: "https://api.example.com/api/v1/auth/feishu/callback"
```

---

## 3. QQ 登录

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | QQ 互联 (connect.qq.com) |
| 协议 | OAuth 2.0 (Authorization Code) |
| 文档 | https://wiki.connect.qq.com/ |

### 接入流程

```
App/Web → 跳转 QQ 授权 → 用户确认 → 回调 code → 后端 /api/v1/auth/qq/callback
```

### 关键 API

| 步骤 | API | 说明 |
|------|-----|------|
| code → token | `GET /oauth2.0/token` | 获取 access_token |
| token → openid | `GET /oauth2.0/me` | 获取用户 OpenID |
| 用户信息 | `GET /user/get_user_info` | 获取昵称、头像、性别 |

### 注意事项

- QQ 登录的 access_token 不过期，但需调用 `/oauth2.0/token` 校验有效性
- QQ 没有 UnionID 概念，不同应用的 OpenID 不一致
- 头像 URL 有多种尺寸：`/qq头像路径/132`（100x100）、`/qq头像路径/0`（最大）

### 配置项

```yaml
auth:
  qq:
    app_id: "101xxxxxx"
    app_key: "xxxxxx"
    redirect_uri: "https://api.example.com/api/v1/auth/qq/callback"
```

---

## 4. Apple ID 登录（Sign in with Apple）

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | Apple Developer (developer.apple.com) |
| 协议 | OpenID Connect (基于 OAuth 2.0) |
| 文档 | https://developer.apple.com/sign-in-with-apple/ |

### 接入流程

```
iOS/Web → Apple 授权 → 返回 identity_token (JWT) → POST /api/v1/auth/apple/callback
```

### 实现要点

- Apple 返回的是 **identity_token**（JWT 格式），后端需验证其签名
- 后端需获取 Apple 的公共密钥（JWKS）验证 token 的 `iss`, `aud`, `exp` 等字段
- Apple Public Key URL: `https://appleid.apple.com/auth/keys`
- 后端应缓存 JWKS 并按 `kid` 匹配使用

### 字段映射

| JWT Claim | 说明 |
|-----------|------|
| `sub` | Apple 用户唯一标识（对应 OpenID） |
| `email` | 用户邮箱（可匿名转发，格式如 xxx@privaterelay.appleid.com） |
| `is_private_email` | 是否为匿名邮箱 |
| `aud` | 客户端 ID（bundle ID / service ID） |

### 平台要求

- iOS App **必须**提供 Apple 登录（如果使用了其他第三方登录）
- 需要 Apple Developer Program 会员资格（$99/年）

### 配置项

```yaml
auth:
  apple:
    client_id: "com.example.app"        # Bundle ID (iOS) 或 Service ID (Web)
    team_id: "TEAMID12345"              # Apple Team ID
    key_id: "ABC123DEFG"                # 密钥 ID
    private_key: |
      -----BEGIN PRIVATE KEY-----
      xxxxxx
      -----END PRIVATE KEY-----
    redirect_uri: "https://api.example.com/api/v1/auth/apple/callback"
```

---

## 5. 华为账号登录

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | 华为开发者联盟 (developer.huawei.com) |
| 协议 | OAuth 2.0 |
| SDK | HMS Core (Account Kit) |
| 文档 | https://developer.huawei.com/consumer/cn/doc/ |

### 接入流程

```
Android/HarmonyOS → 调起 Account Kit → 用户授权 → 返回 authorization code → 后端处理
```

### 关键 API

| 步骤 | API | 说明 |
|------|-----|------|
| code → token | `POST /oauth2/v3/token` | 获取 access_token + refresh_token |
| token → 用户信息 | `GET /oauth2/v3/userinfo` | 获取用户信息（OpenID、昵称、头像、手机号等） |
| 刷新 token | `POST /oauth2/v3/token` | grant_type=refresh_token |

### 特性

- **手机号获取**: 用户授权后可获取手机号（需申请权限）
- **实名认证**: 华为账号可获取用户实名状态
- **HarmonyOS 原生集成**: Account Kit 在 HarmonyOS 上有更好的系统级体验

### 配置项

```yaml
auth:
  huawei:
    client_id: "100xxxxxx"
    client_secret: "xxxxxx"
    redirect_uri: "https://api.example.com/api/v1/auth/huawei/callback"
```

---

## 6. 荣耀账号登录

### 平台资料

| 项目 | 说明 |
|------|------|
| 平台 | 荣耀开发者服务 (developer.hihonor.com) |
| 协议 | OAuth 2.0 |
| SDK | 荣耀账号 SDK |
| 文档 | https://developer.hihonor.com/cn/ |

### 接入流程

```
Android → 调起荣耀账号 SDK → 用户授权 → 返回 authorization code → 后端处理
```

### 关键 API

| 步骤 | API | 说明 |
|------|-----|------|
| code → token | `POST /v3/token` | 获取 access_token |
| token → 用户信息 | `GET /v3/userinfo` | 获取 OpenID、昵称、头像、手机号 |
| 刷新 token | `POST /v3/token` | grant_type=refresh_token |

### 特性

- 荣耀账号与华为账号已切割，**独立 AppID + 独立接口域名**
- 可获取手机号（需权限申请）
- 主要面向荣耀品牌 Android 设备用户

### 配置项

```yaml
auth:
  honor:
    client_id: "100xxxxxx"
    client_secret: "xxxxxx"
    redirect_uri: "https://api.example.com/api/v1/auth/honor/callback"
```

---

## 统一配置结构

```yaml
# config.yaml
auth:
  jwt:
    secret: "your-jwt-secret"
    issuer: "go-backend-core"
    access_token_ttl: 2h        # 短 token
    refresh_token_ttl: 30d      # 长 token

  oauth2:
    state_salt: "random-state-salt"  # state 参数签名盐
    state_ttl: 10m                   # state 有效期

  providers:
    wechat:
      enabled: true
      app_id: "wx_xxx"
      app_secret: "xxx"
    feishu:
      enabled: true
      app_id: "cli_xxx"
      app_secret: "xxx"
    qq:
      enabled: true
      app_id: "101xxx"
      app_key: "xxx"
    apple:
      enabled: true
      client_id: "com.example.app"
      team_id: "xxx"
      key_id: "xxx"
      private_key_path: "/run/secrets/apple_key.p8"
    huawei:
      enabled: true
      client_id: "100xxx"
      client_secret: "xxx"
    honor:
      enabled: true
      client_id: "100xxx"
      client_secret: "xxx"
```

所有 Provider 支持 `enabled` 开关，部署时可按需启用/禁用。

---

## API 接口总览

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/auth/:provider/url` | 获取第三方授权 URL |
| `POST` | `/api/v1/auth/:provider/callback` | 授权回调（首次登录 → 自动注册 + 登录） |
| `POST` | `/api/v1/auth/:provider/bind` | 已登录用户绑定第三方账号 |
| `POST` | `/api/v1/auth/:provider/unbind` | 已登录用户解绑第三方账号 |
| `GET` | `/api/v1/auth/bindings` | 获取当前用户已绑定的第三方账号列表 |
| `POST` | `/api/v1/auth/miniapp/login` | 微信小程序登录（code 直换） |
| `POST` | `/api/v1/auth/miniapp/phone` | 微信小程序获取手机号 |
| `POST` | `/api/v1/auth/refresh` | 刷新 JWT Token |
| `POST` | `/api/v1/auth/logout` | 注销（黑名单当前 token） |

### `:provider` 取值范围

核心: `wechat` | `feishu` | `qq` | `apple` | `huawei` | `honor`

扩展（预留 Provider 接口，按需开启配置即可）:

| Provider | 平台 | 适用场景 |
|----------|------|----------|
| `google` | Google Sign-In | 国际用户、Android 原生集成 |
| `github` | GitHub OAuth App | 开发者社区、后台管理 |
| `microsoft` | Microsoft Entra ID | 企业用户、Office 365 集成 |
| `weibo` | 微博开放平台 | 国内社交媒体用户 |
| `dingtalk` | 钉钉开放平台 | 企业内部办公场景 |
| `alipay` | 支付宝开放平台 | 支付场景用户身份 |

扩展 Provider 仅需实现 `SocialProvider` 接口，注册到 Provider 数组即可，无需改动其他代码。

---

## 多账号绑定设计

### 设计目标

一个本地用户可以绑定多个第三方账号，支持以下使用场景：

| 场景 | 说明 |
|------|------|
| 多平台覆盖 | 同一用户同时绑定微信 + 微信小程序 + QQ + Apple ID，任选方式登录 |
| 同平台多账号 | 个人微信 + 企业微信 / 个人飞书 + 企业飞书 |
| 渐进式绑定 | 先用微信登录，之后在设置中补充绑定 QQ 和 Apple ID |
| 账号找回 | 通过任一绑定的第三方账号可重新登录 |

### 数据模型（`social_accounts` 表）

```sql
CREATE TABLE social_accounts (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider       VARCHAR(32) NOT NULL,   -- wechat/feishu/qq/...
    open_id        VARCHAR(255) NOT NULL,   -- 平台用户唯一 ID
    union_id       VARCHAR(255),             -- 平台跨应用统一 ID
    display_name   VARCHAR(128),             -- 第三方平台昵称（冗余展示）
    access_token   TEXT,
    refresh_token  TEXT,
    token_expiry   TIMESTAMPTZ,
    raw_user_info  JSONB,                    -- 原始用户信息快照
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, open_id)                -- 同一第三方账号不能重复绑定
);
```

**约束规则**:
- `UNIQUE(provider, open_id)`: 同一个第三方账号只能被一个本地用户绑定
- `UNIQUE(user_id, provider)`(可选加): 若限制同平台只能绑一个，可额外加此约束；默认**不限制**，允许同平台绑定多个不同账号
- `ON DELETE CASCADE`: 用户注销时自动解绑所有第三方账号

### 绑定 / 解绑逻辑

#### 登录时自动绑定（callback）

```
callback →
  ① Exchange(code) → TokenInfo
  ② GetUserInfo(token) → SocialUser
  ③ social_accounts 表查询 (provider, open_id)
     ├─ 已存在 → 返回该记录关联的 user_id → 签发 JWT
     └─ 不存在 → 创建新用户 + 写入 social_accounts → 签发 JWT
```

#### 已有账号主动绑定（bind，需 JWT）

```
用户已登录 + 第三方 code →
  ① 验证 JWT → 获取当前 user_id
  ② Exchange(code) → TokenInfo
  ③ 检查 (provider, open_id) 是否已被其他用户绑定
     ├─ 已绑定 → 返回错误（"该第三方账号已被其他用户绑定"）
     └─ 未绑定 → 写入 social_accounts(user_id=当前用户) → 绑定成功
```

#### 解绑（unbind，需 JWT）

```
用户已登录 →
  ① 检查是否为该用户最后一个可登录方式
     ├─ 若本地密码为空 且 只剩一个社交绑定 → 拒绝解绑（防止无法登录）
     └─ 通过 → 删除 social_accounts 记录
```

### 查询已绑定账号

```
GET /api/v1/auth/bindings  (需 JWT)
→ 返回当前用户的所有 social_accounts（仅展示 provider + display_name + 绑定时间）
```

### 冲突合并策略

当同一第三方账号试图绑定到另一个本地用户时：

| 策略 | 说明 | 推荐 |
|------|------|------|
| **报错提示** | "该账号已被其他用户绑定，请先解绑" | ✅ 默认 |
| 自动合并 | 将两个本地用户合并（数据迁移复杂） | ❌ 不推荐 |
| 接管确认 | 发送验证码到原用户确认后再合并 | 视业务需求 |

### 前端展示建议

用户中心「账号绑定」页面参考布局：

```
┌─────────────────────────────────┐
│  账号绑定                        │
│                                 │
│  [微信] 昵称1        已绑定  [解绑]│
│  [微信] 昵称2        已绑定  [解绑]│
│  [+ 绑定微信]                    │
│                                 │
│  [QQ]  昵称3         已绑定  [解绑]│
│  [+ 绑定QQ]                     │
│                                 │
│  [Apple ID] 邮箱@...  已绑定  [解绑]│
│                                 │
│  密码登录: 已设置 [修改]          │
└─────────────────────────────────┘
```
