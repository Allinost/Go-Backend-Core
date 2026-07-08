---
title: 部署方案
parent: 运维部署
nav_order: 1
---

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
