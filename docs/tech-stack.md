# 技术选型

## 后端语言及框架

| 层级 | 技术 | 说明 |
|------|------|------|
| 语言 | **Go 1.22+** | 高并发、编译快、跨平台、生态丰富 |
| HTTP 框架 | **Gin** | 性能极高、中间件机制完善、社区活跃（国内 Go 生态首选） |
| 配置管理 | **Viper** | 支持多数据源（文件/env/远程）、热加载 |
| 日志 | **zerolog** | 零分配 JSON 日志器，性能优于 zap 且 API 更简洁 |
| 依赖注入 | **Wire** | Google 出品，编译期注入，类型安全 |
| 数据库 ORM | **GORM** | 功能全面、自动迁移、钩子机制、关联查询 |
| 数据库驱动 | **pgx** (PostgreSQL) / **go-sql-driver/mysql** | 成熟稳定驱动 |
| Redis 客户端 | **go-redis/redis v9** | 官方推荐，支持集群/哨兵/管道 |
| 任务队列 | **asynq** | 基于 Redis 的分布式任务队列，延迟任务支持 |
| API 文档 | **swaggo/swag** | 注解自动生成 Swagger/OpenAPI 文档 |
| 认证鉴权 | **JWT (golang-jwt)** + Casbin (RBAC) | 无状态 JWT + 细粒度权限模型 |
| OAuth2 客户端 | **golang.org/x/oauth2** | 通用 OAuth2 授权码流程 |
| 微信登录 SDK | **github.com/silenceper/wechat/v2** | 微信开放平台/公众平台 Go SDK |
| 飞书 SDK | **github.com/larksuite/oapi-sdk-go** | 飞书开放平台官方 Go SDK |
| Apple Sign In | **github.com/tideland/golibs/oauth2/apple** / 自实现 | Sign in with Apple (OpenID Connect) |
| 参数校验 | **go-playground/validator v10** | Gin 内置，标签式校验 |
| 定时调度 | **github.com/robfig/cron/v3** | Cron 表达式调度，支持秒级/时区 |
| 任务队列 | **asynq** (已选) | 延迟任务、周期性任务也基于 asynq 实现 |
| UPS 监控 | **github.com/robbiet480/go.nut** | NUT 协议客户端（连接 upsd） |
| 数据库迁移 | **github.com/golang-migrate/migrate/v4** | 版本化迁移，支持 up/down/force |
| 加解密 | **golang.org/x/crypto** | AES-GCM / ChaCha20-Poly1305 / RSA / Argon2 |
| 压缩 | **github.com/klauspost/compress** | zstd / snappy / brotli 高性能压缩 |
| P2P / 节点通信 | **github.com/libp2p/go-libp2p** | 去中心化节点发现与加密通信 |
| Ollama 客户端 | **github.com/ollama/ollama/api** | Ollama 本地 LLM 推理 API 客户端 |
| OpenAI 客户端 | **github.com/sashabaranov/go-openai** | OpenAI 协议兼容客户端（One API / 原生） |
| SSH 远程执行 | **golang.org/x/crypto/ssh** | 远程执行关机命令 |
| MinIO 客户端 | **github.com/minio/minio-go/v7** | MinIO / S3 兼容对象存储 SDK |
| Prometheus 客户端 | **github.com/prometheus/client_golang** | 指标暴露 /metrics |
| gRPC | **google.golang.org/grpc** | gRPC 服务器与客户端 |
| HTTP 客户端 | **net/http** (标准库) + 重试封装 | HTTP/HTTPS 请求 |
| Meilisearch 客户端 | **github.com/meilisearch/meilisearch-go** | 全文搜索引擎客户端 |
| Excel 处理 | **github.com/xuri/excelize/v2** | Excel 读写 |
| MQTT 客户端 | **github.com/eclipse/paho.mqtt.golang** | MQTT v3.1.1 / v5 协议 |
| 文件监听 | **github.com/fsnotify/fsnotify** | 文件/目录变更事件监听 |
| 音频元数据 | **github.com/dhowden/tag** | 音频文件 ID3/FLAC/MP4 标签读写 |
| WOL 网络唤醒 | **github.com/sabhiram/go-wol** | 发送 Wake-on-LAN 魔术包 |
| 测试 | **testify** + **gomock** | 断言库 + 模拟生成 |
| 代码检查 | **golangci-lint** | 集成多种 linter，统一配置 |
| 热重载 | **air** | 开发时文件变更自动编译重启 |

## 微服务/模块通信

| 场景 | 方案 | 说明 |
|------|------|------|
| 模块间同步调用 | **gRPC** | 高性能、强类型、多语言 |
| 模块间异步通知 | **Redis Pub/Sub + asynq** | 轻量级消息 |
| 跨服务 HTTP | **RESTful JSON** | 对外统一 API 风格 |
| 服务发现 | **Static (Docker Compose)** | 初期用 Compose service name 直连 |

## 数据层

| 组件 | 选型 | 说明 |
|------|------|------|
| 主数据库 | **PostgreSQL 16** | 关系型、JSONB 支持、GIS 扩展 |
| 缓存 | **Redis 7** | 缓存/会话/队列/排行榜 |
| 对象存储 | **RustFS** | 自建云存储服务，S3 兼容 |
| 搜索引擎(预留) | **Meilisearch / Elasticsearch** | 后续按需引入 |

## DevOps / 部署

| 层级 | 技术 | 说明 |
|------|------|------|
| 容器化 | **Docker + Docker Compose** | 本地开发与生产部署一致 |
| CI/CD | **GitHub Actions** | 自动构建、测试、部署 |
| 反向代理 | **Nginx / Caddy** | TLS 终结、路由分发 |
| 监控 | **Prometheus + Grafana** | 指标采集与可视化 |
| 日志收集 | **Loki + Promtail** | 轻量日志聚合 |

## 客户端支持

| 平台 | 接入方式 |
|------|----------|
| 客户端 | 接入方式 | OAuth 登录支持 |
|--------|----------|----------------|
| 微信小程序 | HTTPS REST API + WebSocket | 小程序登录（code → session_key） |
| Android | REST API | 微信/QQ/AppleID/华为/荣耀 |
| iOS | REST API | 微信/QQ/AppleID（Apple 强制要求） |
| HarmonyOS | REST API | 华为账号登录 |
| Web (Vue/React) | REST API + WebSocket | 微信扫码/QQ/AppleID/飞书 |
| Desktop (WPF/Electron/Tauri) | REST API | 微信扫码/QQ/AppleID/飞书 |
