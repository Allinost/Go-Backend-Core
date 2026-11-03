# 部署方案

## Docker Compose 架构（本地开发）

```yaml
services:
  app:
    build: .
    ports:
      - "29090:29090"
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./config.yaml:/app/config.yaml
    environment:
      - GIN_MODE=release

  mysql:
    image: mysql:8
    ports:
      - "3306:3306"
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: go_backend_core
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
    volumes:
      - mysqldata:/var/lib/mysql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]

volumes:
  mysqldata:
```

## 生产部署推荐

### 方案 1：Docker Compose（小规模）

- 单机部署，适合月活 < 10 万
- Nginx 反向代理 + TLS 终结
- MySQL + Redis 独立数据卷

### 方案 2：Kubernetes（大规模）

- 各模块可独立拆分为微服务后部署
- Helm Chart 管理
- Horizontal Pod Autoscaler
- Ingress Controller + Cert Manager

## GitHub 镜像构建与服务器拉取（GHCR）

### 触发条件（按 Actions 配额控制）

流水线：`.github/workflows/ci.yml`（`Manual Release`）。

- **仅手动触发** `workflow_dispatch`：push / tag 不会自动运行，不会意外消耗 Actions 分钟。
- **仓库变量开关**：`Settings → Secrets and variables → Actions → Variables` 新增 `ENABLE_GHCR_BUILD`。
  - 值必须为 `true` 才会执行；未设置或为 `false` 时任务直接 **skipped**，不分配 runner、不消耗分钟。
  - Actions 配额不足时：保持未设置/置为 `false`，无需改动代码。
- 手动运行时可选择：
  - `image_tag`：镜像标签（`latest` / `v1.0.0`）
  - `push_latest`：是否同时更新 `latest`
  - `run_checks`：构建前是否执行 `go vet` + `go test -short`（默认关闭以节省时间）

### 产物

```
ghcr.io/allinost/go-backend-core:<image_tag>
ghcr.io/allinost/go-backend-core:latest
ghcr.io/allinost/go-backend-core:sha-<commit>
```

镜像为 `linux/amd64`，构建阶段会生成 Swagger 并编译静态二进制。

### 服务器拉取最新镜像

```bash
# 私有包首次登录（PAT 需要 read:packages；公开包可跳过）
echo "$GHCR_PAT" | docker login ghcr.io -u <github-username> --password-stdin

cd /path/to/Go-Backend-Core
# .env 中 APP_IMAGE_TAG=latest 或指定版本
docker compose pull app
docker compose up -d --no-build app
docker image prune -f
```

只拉取不启动：

```bash
docker pull ghcr.io/allinost/go-backend-core:latest
```

固定版本发布建议把 `.env` 的 `APP_IMAGE_TAG` 改为具体标签（如 `v1.0.0`），避免服务器无意跟随 `latest`。

### 无 Actions 配额时的替代方案

直接在服务器用仓库源码构建，不依赖 GitHub Actions：

```bash
cd /path/to/Go-Backend-Core
docker compose build --pull app
docker compose up -d app
```

> 说明：`docker-compose.yml` 同时保留 `image` 与 `build`。`pull` 使用镜像，`build` 使用本地源码，二者按命令选择。

## 环境管理

| 环境 | 配置 | 数据库 |
|------|------|--------|
| local | config.local.yaml | Docker 本地 MySQL/Redis |
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
| 3306 | MySQL（仅内网） |
| 5432 | PostgreSQL NAS（仅内网） |
| 6379 | Redis（仅内网） |
| 9090 | Prometheus |
| 3000 | Grafana |
