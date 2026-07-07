# 部署方案

## Docker Compose 架构（本地开发）

```yaml
services:
  app:
    build: .
    ports:
      - "29090:29090"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./config.yaml:/app/config.yaml
    environment:
      - GIN_MODE=release

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]

volumes:
  pgdata:
```

## 生产部署推荐

### 方案 1：Docker Compose（小规模）

- 单机部署，适合月活 < 10 万
- Nginx 反向代理 + TLS 终结
- PostgreSQL + Redis 独立数据卷

### 方案 2：Kubernetes（大规模）

- 各模块可独立拆分为微服务后部署
- Helm Chart 管理
- Horizontal Pod Autoscaler
- Ingress Controller + Cert Manager

## 环境管理

| 环境 | 配置 | 数据库 |
|------|------|--------|
| local | config.local.yaml | Docker 本地 PG/Redis |
| dev | config.dev.yaml | 开发服务器 |
| staging | config.staging.yaml | 预发布环境 |
| production | config.prod.yaml | 生产环境 |

多环境通过 `APP_ENV` 环境变量切换，Viper 自动加载对应配置：

```bash
APP_ENV=production docker-compose up
```

## CI/CD 流水线

```yaml
# .github/workflows/ci.yml
on: [push, pull_request]
jobs:
  lint:
    - golangci-lint run
  test:
    - go test ./... -race -coverprofile=coverage.out
  build:
    - docker build -t app .
    - docker push registry/app:${GITHUB_SHA}
```

## 端口映射

| 端口 | 用途 |
|------|------|
| 29090 | 主服务 HTTP API |
| 5432 | PostgreSQL（仅内网） |
| 6379 | Redis（仅内网） |
| 9090 | Prometheus |
| 3000 | Grafana |
