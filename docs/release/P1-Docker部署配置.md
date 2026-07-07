# P1 — Docker 部署配置

开发期直接用 `go run` 启动，容器化后需注意以下差异。

---

## Dockerfile

| # | 检查项 | 开发期现状 | 发布要求 | 涉及文件 |
|---|--------|-----------|---------|----------|
| 1 | 基础镜像 | 无 | 使用 `alpine:3.21` 运行镜像，最终镜像约 15MB | `deploy/docker/Dockerfile` |
| 2 | 多阶段构建 | 无 | 构建阶段用 `golang:1.26-alpine`，运行阶段仅复制二进制和配置 | `deploy/docker/Dockerfile` |
| 3 | 二进制压缩 | 无 | 使用 `-ldflags="-s -w"` 去除调试信息，减小体积 | `deploy/docker/Dockerfile` |
| 4 | CGO 禁用 | 无 | `CGO_ENABLED=0` 确保静态编译，镜像无需 glibc | `deploy/docker/Dockerfile` |
| 5 | 时区与证书 | 无 | 安装 `ca-certificates` 和 `tzdata`，否则 HTTPS 调用和时区会失败 | `deploy/docker/Dockerfile` |
| 6 | 配置文件打包 | 开发期 `config.yaml` 在本地 | 禁止将 `config.yaml` 打包进镜像，通过 volume 挂载覆盖 | `deploy/docker/Dockerfile`, `docker-compose.yml` |

## Docker Compose

| # | 检查项 | 开发期现状 | 发布要求 | 涉及文件 |
|---|--------|-----------|---------|----------|
| 1 | 外部数据库依赖 | 无（开发期本地直连） | `docker-compose.yml` 应通过 `external_network` 或 `extra_hosts` 指向现有 PG/Redis | `docker-compose.yml` |
| 2 | 健康检查 | 无 | 添加 `healthcheck` 探测 `/api/v1/s0/health`，确保 k8s/编排工具可判断服务就绪 | `docker-compose.yml` |
| 3 | 资源限制 | 无限制 | 设置 `deploy.resources.limits` 防止容器占用全部主机资源 | `docker-compose.yml` |
| 4 | 重启策略 | 无 | 设置 `restart: unless-stopped` 保证意外退出后自动拉起 | `docker-compose.yml` |
| 5 | 日志驱动 | 默认 json-file | 生产环境改为 `local` 或对接 Loki，避免磁盘被日志占满 | `docker-compose.yml` |
| 6 | 网络模式 | bridge 默认 | 生产环境应创建独立网络 `backend-net`，仅暴露 29090 端口 | `docker-compose.yml` |

## 构建与部署流程

```
# 本地开发
go run ./cmd/server/

# 构建镜像
docker compose build

# 启动服务（依赖已有 PG/Redis，通过环境变量指定地址）
docker compose up -d

# 查看日志
docker compose logs -f app
```

## 快速验证

```bash
# 检查镜像大小（应 < 20MB）
docker images go-backend-core-app

# 检查是否依赖外部 glibc
docker run --rm go-backend-core-app ldd /app/server 2>&1 || echo "静态编译 ✓"

# 验证健康检查
curl http://localhost:29090/api/v1/s0/health
```
