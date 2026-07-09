# P11 — API 密钥管理

---

## 已实现的功能

| 功能 | 状态 | 说明 |
|------|------|------|
| ApiKey 模型 | ✅ | ID / Name / KeyPrefix / KeyHash(SHA-256) / UserID / Scopes / LastUsedAt / ExpiresAt / Status / 软删除 |
| 密钥生成 | ✅ | 32 字节随机数 + hex 编码，前缀 `gbk_`，仅创建时明文返回一次 |
| 密钥验证 | ✅ | SHA-256 哈希查询 + status 检查 + expires_at 检查 |
| 最后使用时间 | ✅ | ValidateKey 自动更新 LastUsedAt |
| 密钥 CRUD | ✅ | 创建/列表/更新/删除(软删除) |
| ApiKeyStore 接口 | ✅ | InMemoryApiKeyStore + MySQLApiKeyStore (GORM AutoMigrate) |
| API Key 中间件 | ✅ | `APIKeyAuth` + `APIKeyOrJWT` (可混合使用) |
| 路由 | ✅ | 本人可管理自己的密钥，admin 可查看/管理所有密钥 |
| 密钥作用域 | ✅ | scopes 字段 (如 `task:read` / `*:*`) |

## API 路由

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| `POST` | `/api/v1/auth/api-keys` | 🔒 | 创建 API 密钥 (返回 `raw_key` 仅一次) |
| `GET` | `/api/v1/auth/api-keys` | 🔒 | 本人: 自己的密钥列表; admin: 全部密钥分页 |
| `PUT` | `/api/v1/auth/api-keys/:id` | 🔒 | 更新密钥 (name/scopes/status) |
| `DELETE` | `/api/v1/auth/api-keys/:id` | 🔒 | 删除密钥 |

## 中间件

| 函数 | 说明 |
|------|------|
| `APIKeyAuth(apiKeySvc)` | 仅通过 `X-API-Key` header 认证，设置 `user_id` |
| `APIKeyOrJWT(apiKeySvc, authSvc, bl)` | 优先 API Key，回退 JWT Bearer |

用法示例:
```go
r.GET("/api/v1/some-endpoint", middleware.APIKeyOrJWT(m.apiKeys, m.svc, m.blacklist), handler)
```

## 安全设计

- 密钥前缀 `gbk_` + 64 位 hex → 明文仅创建时展示
- 数据库中只存 SHA-256 哈希值 → 泄露数据库也无法获取原始密钥
- 支持过期时间 (`expires_at`) 和手动禁用 (`status`)
- 中间件 `APIKeyAuth` 验证密钥合法性，设置 `user_id` 供下游使用

## 参考文档

- `internal/services/auth/apikey.go` — ApiKey 模型 + ApiKeyStore 接口 + ApiKeyService
- `internal/middleware/apikey.go` — APIKeyAuth / APIKeyOrJWT 中间件
- `internal/modules/auth/module.go` — 路由 + handlers
- `internal/config/config.go` — (无需额外配置，复用 auth 模块的 UserStore/MySQL)

## 尚存的妥协项

| 妥协项 | 优先级 | 说明 |
|--------|--------|------|
| API 密钥作用域验证 | P3 | scopes 字段已存储，但 `APIKeyAuth` 中间件尚未做细粒度作用域校验 |
| 密钥审计日志 | P3 | EventBus 暂未发布 `apikey.*` 事件 |
