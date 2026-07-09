# Go-Backend-Core 开发记录

## Objective
API 密钥管理 (11) — ApiKey 模型 + Store + Service + 中间件 + 路由

## Completed
- `internal/services/auth/apikey.go`:
  - `ApiKey` 模型: Name, KeyPrefix, KeyHash(SHA-256), UserID, Scopes, LastUsedAt, ExpiresAt, Status, 软删除
  - `ApiKeyStore` 接口: Create, FindByHash, ListByUser, ListAll, Update, Delete
  - `InMemoryApiKeyStore`: 完整实现
  - `MySQLApiKeyStore`: GORM AutoMigrate + 完整实现
  - `ApiKeyService`: GenerateKey (32B随机+hex+gbk_前缀), ValidateKey (hash+status+expiry+LastUsedAt)
- `internal/middleware/apikey.go`:
  - `APIKeyAuth(apiKeySvc)` — 仅 `X-API-Key` header 认证
  - `APIKeyOrJWT(apiKeySvc, authSvc, bl)` — 优先 API Key, 回退 JWT Bearer
- `internal/modules/auth/module.go`:
  - API key 初始化 (MySQL/InMemory 回退)
  - POST/GET/PUT/DELETE `/api/v1/auth/api-keys`
  - 本人只能管理自己的 key; admin 可管理全部
- `docs/release/P11-API密钥管理.md`

## Tests
- auth 服务: 39 个 (新增 4 个: GenerateAndValidate, InvalidKey, ExpiredKey, ListAndDelete)
- module 模块: 26 个 (新增 4 个: CreateAPIKey, ListOwn, ListAdmin, DeleteAPIKey)
- 全量 24 包 `go test ./... -short` ✅
