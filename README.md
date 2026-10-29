# Go Backend Core

模块化 Go 后端服务（Modular Monolith），基于 **Gin** 的模块注册表架构，提供统一 API 网关、多业务模块与公共服务，支撑 Web / 小程序 / App 等多端接入。

- **模块化架构** — `Module` 接口 + 全局注册表，15 个模块按注册顺序初始化、逆序关闭
- **统一三层结构** — Handler → Service → Repository，模块间经 Service Interface 解耦
- **双引擎** — 主引擎 `:29090` 业务 API，调试引擎 `:29092` Swagger UI + 健康检查
- **多数据源** — MySQL / PostgreSQL / Redis / MinIO / RustFS，统一 Manager + 熔断保护
- **可观测** — Prometheus 指标、zerolog 结构化日志、独立审计日志

## 技术栈

| 层级 | 选型 |
|------|------|
| 语言 / 框架 | Go 1.26 · Gin |
| 数据库 | MySQL（主库）· PostgreSQL · Redis · MinIO · RustFS |
| ORM | GORM + 原生 `database/sql`（业务模块多为手写 SQL） |
| 任务调度 | robfig/cron · asynq · 模块内 ticker |
| 认证 | JWT（golang-jwt）· RBAC 中间件 |
| 日志 / 监控 | zerolog · Prometheus + Grafana |
| 文档 | swaggo Swagger |
| 其他 | chromedp（浏览器自动化）· bleve（全文检索）· WebSocket |

## 快速开始

### 环境要求

- Go 1.26+
- MySQL 8（必需）；Redis / PostgreSQL / MinIO 按需（`docker compose up` 可拉起 `app` + `chrome` 服务）

### 运行

```bash
# 1. 配置（优先级：Docker Secret > 环境变量 APP_* > config.yaml > 默认值）
cp .env.example .env        # 填写数据库等连接信息
vi config.yaml              # 或直接修改配置文件

# 2. 启动（build + 运行，自动先生成 Swagger 文档）
make run

# 3. 访问
#    业务 API    http://localhost:29090/api/v1/{module}
#    Swagger UI  http://localhost:29092/swagger/index.html   （调试端口，支持环境预设切换）
```

### 常用命令

| 命令 | 说明 |
|------|------|
| `make run` | 构建并启动服务（debug） |
| `make build` / `make release` | 构建 debug / release 二进制 |
| `make swagger` | 重新生成 Swagger API 文档 |
| `make test` / `make test-race` | 运行全部单元测试 / 竞态检测 |
| `make coverage` / `make coverage-html` | 生成覆盖率报告 |
| `make lint` | golangci-lint 静态检查 |
| `make docker-build` | 构建 Docker 镜像 |
| `make help` | 查看全部命令 |

脚本版本见 [`scripts/`](scripts/)（`run.sh` / `test.sh` / `lint.sh` / `coverage.sh` / `ci.sh`）。

## 项目结构

```
go-backend-core/
├── cmd/                  # 入口：server / migrate / backup
├── internal/             # 私有核心代码
│   ├── config/           #   配置加载与热更新（Viper + fsnotify）
│   ├── database/         #   多数据源 Manager + 熔断
│   ├── middleware/       #   logger / CORS / ratelimit / rbac / metrics
│   ├── modules/          #   Module 接口与注册表 + 基础业务模块
│   ├── services/         #   20 个公共服务（ai/cache/eventbus/scheduler/tracing/ws...）
│   └── pkg/              #   logger / response / errors / crypto
├── zzz/                  # 业务域模块（垂直切片）
│   ├── ia/               #   投资分析（行情/筛选/告警/建议）
│   ├── login3rd/         #   第三方登录（chromedp 浏览器自动化）
│   ├── shopchecker/      #   店铺采集（淘宝/京东/拼多多）
│   └── goodser/          #   商品服务
├── pkg/                  # 公开可复用库（apierror/crypto/net/pagination，均含测试）
├── api/                  # swagger 生成物 + proto 定义
├── migrations/           # 版本化迁移脚本（mysql/ postgres/）
├── deploy/               # Docker、Compose、Prometheus、Grafana
├── docs/                 # 项目文档 ← 从 docs/README.md 开始
├── scripts/              # 运维脚本
└── config.yaml           # 主配置文件
```

## 已注册模块

路由统一挂载在 `/api/v1/{name}`：

| 类型 | 模块 |
|------|------|
| 基础 | `s0`（调试/健康检查，同时挂调试端口）· `s1` · `s2` · `auth` · `migrate` · `ops` · `webhook` · `servicenav` |
| 服务 | `scheduler` · `net` · `crypto` |
| 业务 | `ia` · `login3rd` · `shopchecker` · `goodser` |

模块接口与注册机制见 [`docs/module-design.md`](docs/module-design.md)，完整模块清单见 [`docs/README.md`](docs/README.md)。

## 测试与 CI

```bash
make test          # 全部单元测试
make test-race     # 竞态检测（需 CGO_ENABLED=1）
make coverage-html # 生成可打开的 HTML 覆盖率报告
make lint          # golangci-lint
```

CI 流程见 [.github/workflows/ci.yml](.github/workflows/ci.yml)（测试 + 构建镜像），本地可用 `scripts/ci.sh` 模拟。

## 部署

```bash
docker compose up -d                        # 应用编排（app + chrome）
docker compose -f docker-compose.monitoring.yml up -d   # 监控编排（Prometheus + Grafana）
```

详见 [`docs/deployment.md`](docs/deployment.md) 与 [`deploy/`](deploy/)。

## 文档

**入口：[`docs/README.md`](docs/README.md)** — 架构设计、业务模块文档、API 索引、发布记录的统一导航。

- 架构：[docs/architecture.md](docs/architecture.md) · [docs/tech-stack.md](docs/tech-stack.md)
- 模块：[docs/module-design.md](docs/module-design.md) · [docs/modules/](docs/modules/)
- API：[docs/API/index.md](docs/API/index.md) · 在线 Swagger（`:29092/swagger`）

## License

[Apache License 2.0](LICENSE)
