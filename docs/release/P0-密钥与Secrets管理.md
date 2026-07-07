# P0 — 密钥与 Secrets 管理

开发期密码和密钥写在 `config.yaml` 中明文存储，发布前必须迁移到安全方案。

---

## 密钥类型

| 密钥 | config.yaml 默认值 | 风险级别 |
|------|-------------------|---------|
| JWT 签名密钥 | `change-me-in-production` | 严重 — 可伪造任意 JWT |
| MySQL 密码 | `root` | 严重 — 数据库完全暴露 |
| Redis 密码 | 空 | 高 — 缓存数据可被读取 |

## 防护方案对比

| 方案 | 复杂度 | 安全等级 | 适用场景 |
|------|--------|---------|---------|
| `.env` 文件 + `godotenv` | ★ | 中 | **本地开发**（已实现） |
| 环境变量 `APP_*` 注入 | ★ | 中高 | Docker Compose / CI |
| Docker Secrets (`/run/secrets/*`) | ★★ | 高 | **生产 Docker** 部署 |
| Kubernetes Secrets | ★★★ | 高 | K8s 部署 |
| 外部密钥管理 (Vault/AWS KMS) | ★★★★ | 极高 | 企业级生产 |

## 开发期（当前方案）

```
config.yaml        # 提交到 Git，仅含非敏感配置（主机/端口/库名等），密码字段留空
.env               # 本地真实密钥，已 gitignored（不提交）
.env.example       # 环境变量模板，可提交（占位值）
```

`config.yaml` 中密码字段全部留空（`password: ""`），所有敏感值通过 `.env` 中的 `APP_*` 变量注入。`godotenv` 在 `config.Load()` 启动时自动读取 `.env`，Viper 的环境变量覆盖机制确保 `.env` > `config.yaml`。

## Docker 生产方案

```yaml
# docker-compose.yml
services:
  app:
    environment:
      - APP_DATABASE_POSTGRES_MAIN_PASSWORD=${PG_PASSWORD}
      - APP_AUTH_JWT_SECRET=${JWT_SECRET}
    # 或使用 Docker Secrets：
    secrets:
      - db_password
      - jwt_secret

secrets:
  db_password:
    file: ./secrets/db_password.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
```

## 检测脚本

```bash
# 检查是否有密钥明文提交到 Git
git grep -E "(password|secret|key)\s*[:=]\s*['\"]?[a-zA-Z0-9_-]{8,}" -- config.yaml

# 检查 .env 是否被意外跟踪
git ls-files --error-unmatch .env 2>&1 || echo ".env 未被跟踪 ✓"

# 检查 gitignore 是否正确
grep -q "^\.env$" .gitignore && echo ".gitignore 已忽略 .env ✓"
```

## 密码复杂度要求

| 密钥 | 最低长度 | 推荐生成方式 |
|------|---------|-------------|
| JWT 密钥 | 32 字符 | `openssl rand -base64 32` |
| 数据库密码 | 16 字符 | `openssl rand -base64 16` |
| Redis 密码 | 16 字符 | `openssl rand -base64 16` |
