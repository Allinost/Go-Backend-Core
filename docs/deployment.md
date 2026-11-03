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

### 前端镜像（GHCR）

前端仓库 `Go-Frontend-Core` 同样提供手动 GHCR 流水线：

```
ghcr.io/allinost/go-frontend-core:<tag>
ghcr.io/allinost/go-frontend-core:latest
ghcr.io/allinost/go-frontend-core:sha-<commit>
```

服务器拉取（GHCR 包默认私有，需要 `read:packages` 的 PAT；设为公开后可跳过登录）：

```bash
echo "$GHCR_PAT" | docker login ghcr.io -u <github-username> --password-stdin
docker pull ghcr.io/allinost/go-frontend-core:latest
docker run -d --name go-frontend-core \
  --restart unless-stopped \
  --network 1panel-network \
  -p 8080:80 \
  ghcr.io/allinost/go-frontend-core:latest
```

- 容器内 nginx 将 `/api/` 反代到 `http://gobackendcore:29090`，需与后端同处 `1panel-network`。
- nginx 使用 Docker DNS 延迟解析后端地址：后端暂未启动时前端容器也能正常启动，后端恢复后自动可用。
- 若后端服务名不是 `gobackendcore`，需修改 `deploy/nginx.conf` 后重新构建镜像。
- 前端 API 地址也可在页面「设置 → 前端设置」中改为公网后端地址。

### 应用设置持久化

- 设置文件：`data/settings.json`（0600，原子写入，可由 `APP_SETTINGS_FILE` 覆盖）
- Compose 已挂载 `./data:/app/data`，容器重建后设置保留
- 页面顶部「设置」→「后端设置」可修改公开地址/前端默认值/外部工具地址；数据库、Redis、JWT 等仍需改 `.env` 后重启

### 采集浏览器（chrome-browser）

`docker-compose.yml` 内置 `chrome-browser`（browserless/chrome），供 shopchecker / login3rd 的浏览器采集使用：

- 容器内地址：`ws://chrome-browser:3000`（compose 已通过 `CHROME_REMOTE_URL` 注入）
- 宿主机调试端口：`${CHROME_HOST_PORT:-3000}:3000`
- `shm_size: 8gb`，服务器内存较小时可调低到 `2g`
- 健康检查：`/json/version`，通过后才启动 app

## 1Panel 网络与外部工具

生产环境 `gobackendcore` 与 n8n / OpenClaw / LangBot 同处 `1panel-network`，容器之间使用「容器名 + 容器端口」访问；开发环境通过 EasyTier 入口 `et.hailong.site` + 宿主机发布端口访问。

| 服务 | 1panel-network 内 | 开发（EasyTier） |
|---|---|---|
| gobackendcore | `http://gobackendcore:29090` | `https://et.hailong.site:29090` |
| n8n | `http://n8n:5678` | `https://et.hailong.site:5678` |
| OpenClaw | `http://openclaw:18789` | `https://et.hailong.site:18789` |
| LangBot | `http://langbot:5300` | `https://et.hailong.site:5300` |

> 开发端口以 1Panel 实际映射或反向代理为准。后端对外地址由 `server.public_base_url` 控制（默认 `https://et.hailong.site:29090`），第三方登录回调与远程登录链接都会使用该值；若线上只发布 80/443，改这一个配置即可。

接入步骤、API Key 作用域、Webhook 验签、验收与排障见 [modules/integration/connectivity.md](modules/integration/connectivity.md)。

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
