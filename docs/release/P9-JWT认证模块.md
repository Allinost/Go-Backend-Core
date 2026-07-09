# P9 — JWT 认证模块生产就绪清单

---

## 已实现的功能

| 功能 | 状态 | 说明 |
|------|------|------|
| 双 Token 机制 | ✅ | `access_token`（短效, 默认30m）+ `refresh_token`（长效, 默认7d） |
| JWT 签发/验证 | ✅ | HS256 签名，TokenType 防类型混淆 |
| Token 刷新 | ✅ | `POST /api/v1/auth/refresh` — refresh_token → 新 token pair |
| Token 撤销 | ✅ | `POST /api/v1/auth/logout` → Redis/InMemory 黑名单 |
| 用户注册/登录 | ✅ | bcrypt 密码哈希 + token pair |
| 用户信息 | ✅ | `GET /api/v1/auth/me` — 含 roles 信息 |
| OAuth2 授权 URL | ✅ | `GET /api/v1/auth/{provider}/url` — 6 个 provider |
| OAuth2 回调登录 | ✅ | `POST /api/v1/auth/{provider}/callback` — code → token |
| OAuth2 绑定/解绑 | ✅ | `POST /api/v1/auth/{provider}/bind|unbind` — 🔒 |
| 社交账号列表 | ✅ | `GET /api/v1/auth/accounts` — 🔒 已绑定的第三方账号 |
| RBAC 权限控制 | ✅ | Role(admin/user) + Permission(resource:action) + `RequirePermission` 中间件 |
| 角色分配 | ✅ | `POST /api/v1/auth/roles/assign` — 🔒 admin only |
| MySQL 外键 | ✅ | TaskLog.TaskID → Task 的 GORM constraint (CASCADE) |
| UserStore DB | ✅ | MySQLUserStore (GORM AutoMigrate) + InMemoryUserStore 回退 |
| Auth 中间件 | ✅ | AuthRequired + AuthRequiredWithBlacklist + RequirePermission |
| 登录审计事件 | ✅ | EventBus: `user.registered/login/logout` |
| Provider 接口 | ✅ | SocialProvider(Name/AuthURL/Exchange/GetUserInfo) |
| OAuth2 提供商 | ✅ | 微信 / 飞书 / QQ / Apple / 华为 / 荣耀 |
| 社交账号持久化 | ✅ | SocialStore + InMemorySocialStore |
| RBAC 持久化 | ✅ | RBACStore 接口 + MySQLRBACStore (GORM) + InMemoryRBACStore |
| 配置驱动 | ✅ | `config.yaml` 中 `auth` 段含 `oauth2.*` 子段 |
| Module 集成 | ✅ | 挂载到 `/api/v1/auth/*` |
| 测试覆盖 | ✅ | auth 服务(25) + 中间件(7) + 模块(15) + 调度器(25) = **72 个测试** |

---

## 已修复妥协项

| 妥协项 | 实现 |
|--------|------|
| OAuth2 社交登录 | SocialProvider 接口 + 6 个 OAuth2 实现 + URL/回调/绑定/解绑 4 个 API |
| RBAC 权限系统 | RBACStore 接口 + MySQLRBACStore (GORM AutoMigrate) 持久化 |
| MySQL 外键约束 | TaskLog.Task 字段 + GORM `constraint:OnUpdate:CASCADE,OnDelete:CASCADE` |

## 尚存的妥协项

| 妥协项 | 优先级 | 说明 |
|--------|--------|------|
| 更多的 OAuth2 Provider | — | 6 个已够用 |
| 更细粒度 Role 模型 | P3 | 权限持久化已就绪，Role 权限管理 API 待需时添加 |
| Token 黑名单自动清理 | P3 | Redis 已有 TTL，InMemory 版需后台协程清理过期条目 |

---

## OAuth2 提供商配置示例

```yaml
auth:
  jwt_secret: your-secret
  jwt_expire: 30m
  oauth2:
    wechat:
      client_id: wx_appid
      client_secret: wx_secret
      redirect_url: https://example.com/auth/wechat/callback
    feishu:
      client_id: feishu_appid
      client_secret: feishu_secret
      redirect_url: https://example.com/auth/feishu/callback
    qq:
      client_id: qq_appid
      client_secret: qq_appkey
      redirect_url: https://example.com/auth/qq/callback
    apple:
      client_id: apple_service_id
      team_id: apple_team_id
      key_id: apple_key_id
      private_key: "-----BEGIN PRIVATE KEY-----\n..."
      redirect_url: https://example.com/auth/apple/callback
    huawei:
      client_id: huawei_client_id
      client_secret: huawei_secret
      redirect_url: https://example.com/auth/huawei/callback
    honor:
      client_id: honor_client_id
      client_secret: honor_secret
      redirect_url: https://example.com/auth/honor/callback
```

## 参考文档

- `internal/services/auth/auth.go` — JWT 双 token + 类型校验
- `internal/services/auth/provider.go` — SocialProvider 接口 + SocialStore
- `internal/services/auth/wechat.go` — 微信 OAuth2
- `internal/services/auth/feishu.go` — 飞书 OAuth2
- `internal/services/auth/qq.go` — QQ OAuth2
- `internal/services/auth/apple.go` — Apple ID OAuth2
- `internal/services/auth/huawei.go` — 华为 OAuth2
- `internal/services/auth/honor.go` — 荣耀 OAuth2
- `internal/services/auth/rbac.go` — RBACStore 接口 + MySQLRBACStore + RBACService
- `internal/services/auth/store.go` — UserStore + MySQLUserStore
- `internal/services/auth/blacklist.go` — Token 黑名单
- `internal/middleware/auth.go` — AuthRequired / AuthRequiredWithBlacklist
- `internal/middleware/rbac.go` — RequirePermission 中间件
- `internal/modules/auth/module.go` — 11 个 API 路由 + 6 provider 注册
- `internal/config/config.go` — AuthConfig + OAuth2Config
- `internal/services/scheduler/task.go` — TaskLog GORM 外键约束
