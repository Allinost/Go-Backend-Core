---
title: 开发规划
parent: 开发指南
nav_order: 1
---

# 开发规划

## 开发优先级

根据业务上线顺序：**个人物品管理 → 知识管理 → 智能家居 → 自动化工作流 → 音乐管理**

每阶段先完成该业务依赖的公共模块，再开发业务模块本身。

---

## 阶段划分

### Phase 0 — 项目初始化（预计 1 周）

**目标**: 搭建脚手架，跑通 CI/CD

- [x] Go Module 初始化 (`go mod init github.com/Allinost/go-backend-core`)
- [x] 目录结构搭建
- [x] `config.yaml` + Viper 配置加载
- [x] Gin 引擎 + 基础中间件（Logger, CORS, Recovery, RateLimit）
- [x] 统一响应格式 + 错误码体系
- [x] 模块注册器框架
- [x] Dockerfile + docker-compose.yml（仅 Go 后端）
- [x] Makefile（build/run/test/lint 等命令）
- [x] golangci-lint 配置
- [x] GitHub Actions 基础流水线（lint → test → build，仅 tag 触发）
- [x] `air` 热重载配置

**交付物**: 能通过 `docker-compose up` 启动，`:29090/ping` 返回 pong

---

### Phase 1 — 核心基础设施（预计 3 周）

**目标**: 搭建所有业务模块依赖的公共基础能力

```
[依赖关系] 所有后续业务都依赖此阶段产出的模块
```

- [ ] `internal/database` — 统一数据访问层
  - MySQL / PostgreSQL / Redis / MinIO / RustFS 连接池
  - DBManager 全局管理器 + 多实例配置
  - 健康检查 + Prometheus 指标
- [ ] `internal/services/config` — 配置管理（Viper 热加载 + Reloader 接口）
- [ ] `internal/services/logger` — 日志管理（zerolog + 审计日志 + 多输出）
- [ ] `internal/services/monitor` — 监控模块（健康检查聚合 + /metrics）
- [ ] `internal/services/cache` — 缓存管理（多级缓存 + 防雪崩/穿透）
- [ ] `pkg/crypto/` + `internal/services/crypto` — 加解密与压缩
- [ ] `pkg/net/` + `internal/services/net` — 网络通信（HTTP/TCP/UDP/gRPC）
- [ ] `internal/services/eventbus` — 事件总线（Redis Stream，模块间解耦）
- [ ] `internal/services/scheduler` — 定时/周期任务调度（robfig/cron + asynq）
- [ ] `internal/services/jobs` — 后台任务管理（进度上报/取消/重跑/日志）
- [ ] `internal/services/auth` — JWT 签发/验证 + 刷新
- [ ] `internal/services/user` — 用户注册/登录/资料/多端设备绑定
- [ ] `internal/services/apikey` — API 密钥管理
- [ ] `internal/modules/s0` — 调试服务（ping/health/echo/config）
- [ ] `internal/services/migration` — 数据库迁移管理
- [ ] `migrations/` 目录 + golang-migrate 集成
- [ ] `internal/nas/` + `internal/cloud/` — NAS/云服务连接（引用 database 模块）

**交付物**: 所有基础模块可运行，`:29090/api/v1/s0/health` 返回全绿

---

### Phase 2 — 社交登录与多端认证（预计 2 周）

**目标**: 六平台 OAuth 登录，所有客户端共用

- [ ] 统一 `SocialProvider` 接口 + Provider 注册器
- [ ] 微信登录（App + 小程序 code 登录 + Web 扫码）
- [ ] 飞书登录
- [ ] QQ 登录
- [ ] Apple ID 登录（JWKS 签名验证）
- [ ] 华为账号登录
- [ ] 荣耀账号登录
- [ ] `social_accounts` 表 + 绑定/解绑/查询接口
- [ ] OAuth state 防 CSRF
- [ ] `internal/services/ratelimit` — 分布式限流（配合 middleware）
- [ ] 微信小程序手机号获取

**交付物**: 六平台第三方登录全流程可用

---

### Phase 3 — 个人物品管理（预计 4 周）

**目标**: 首个业务系统上线：物品 CRUD + 分类 + 统计 + 导入导出

```
依赖: Phase 1 + Phase 2
```

**公共模块准备**:

- [ ] `internal/services/tagging` — 通用标签系统
  - `Attach / Detach / GetByTag / GetByEntity`
  - 分组管理（物品分类、位置、状态等）
  - 组合标签查询（AND/OR）
- [ ] `internal/services/analytics` — 报表/聚合引擎
  - `Aggregate()` 统一聚合查询（count/sum/avg/group by）
  - 时间序列统计（月度/季度/年度）
  - 导出为 CSV/XLSX
- [ ] `internal/services/dataio` — 数据导入导出
  - CSV/Excel 批量导入（含校验和错误行反馈）
  - 导出模板定义
- [ ] `internal/services/search` — 全文搜索（Meilisearch）
  - 索引管理 + 实时同步
- [ ] `internal/services/filesystem` — 文件系统监听
  - 物品图片附件管理
- [ ] `internal/services/notify` — 通知渠道
  - 库存预警通知（站内信 + Push）
- [ ] `internal/services/template` — 模板引擎
  - 物品标签打印模板
  - 导出报表模板
- [ ] `internal/services/webhook` — 出站 Webhook
  - 库存变更通知外部

**业务模块开发**:

- [ ] `internal/modules/s1` — 物品管理模块
  - 物品 CRUD（名称/描述/分类/位置/数量/价格/图片）
  - 库存变动记录（入库/出库/盘点）
  - 多维度筛选 + 标签组合查询
  - 统计看板（总数/总值/分类分布/趋势）
  - 标签打印 / 二维码生成
  - 导入导出（CSV 批量导入/导出）
- [ ] s1 数据库表设计 + Migration
- [ ] 单元测试 + 集成测试

**交付物**: 物品管理系统 MVP — 支持 CRUD、标签分类、统计看板、导入导出

---

### Phase 4 — 知识管理（预计 2 周）

**目标**: 接入 Obsidian 知识库，全文搜索 + 标签体系

```
依赖: Phase 3 (filesystem / tagging / search 已有)
```

**公共模块增强**:

- [ ] `internal/services/filesystem` — 增强：Obsidian Vault 同步
  - 文件变更监听（fsnotify）
  - Markdown 文件解析 + 前置元数据提取
  - 双向同步策略
- [ ] `internal/services/search` — 增强：知识搜索
  - Markdown 内容全文索引
  - 按标签/目录范围搜索
- [ ] `internal/services/tagging` — 增强：知识标签
  - 标签继承（子目录继承父标签）
  - 标签聚合统计

**业务模块开发**:

- [ ] `internal/modules/s2` — 知识管理模块
  - Obsidian Vault 文件索引
  - 笔记全文搜索
  - 知识图谱（标签关联 + 双向链接）
  - 最近编辑/热门文档
  - 知识库统计

**交付物**: 知识库搜索与管理 — Obsidian 笔记实时索引、全文搜索

---

### Phase 5 — 智能家居（预计 2 周）

**目标**: 接入 Home Assistant，设备状态监控与自动化联动

```
依赖: Phase 1 (eventbus / net / monitor 已有)
```

- [ ] `internal/services/mqtt` — MQTT 协议客户端
  - MQTT Broker 连接管理
  - 主题订阅/发布
  - 遗嘱消息（LWT）断线检测
  - MQTT ↔ EventBus 双向桥接
- [ ] `internal/services/webhook` — 增强：Home Assistant 回调
  - Webhook 自动注册到 HA
  - HA 事件 → EventBus 转换
- [ ] `internal/services/ups` — UPS 状态监控（NUT 客户端）
- [ ] `internal/services/lanctl` — 局域网设备管理（SSH/HTTP/WOL）
- [ ] UPS + LAN Ctl → MQTT 联动
  - 断电 → MQTT 发布 → HA 自动化触发
- [ ] HA 设备状态 → 系统通知（notify 模块）

**交付物**: 智能家居联动 — UPS 断电自动推送到 Home Assistant

---

### Phase 6 — 自动化工作流（预计 2 周）

**目标**: 接入 Dify/N8n，AI 能力 + 可视化工作流

```
依赖: Phase 1 (webhook / jobs / eventbus 已有)
```

- [ ] `internal/services/ai` — AI 推理服务
  - Ollama 本地模型调用
  - OpenAI / One API 云端调用
  - 流式 SSE 输出
- [ ] `internal/services/plugin` — 插件系统
  - `Plugin` 接口 + `PluginManifest` 声明
  - 路由/事件/任务/Webhook 自动注册
  - Dify/N8n 以插件形式接入
- [ ] `internal/services/webhook` — 增强：动态 Webhook 端点
  - Dify 工作流触发回调
  - N8n webhook 接收
- [ ] `internal/services/jobs` — 增强：工作流任务
  - 长时间任务进度跟踪
  - 失败自动重试 + 人工介入
- [ ] `internal/services/feature` — 特性开关
  - 灰度发布工作流版本

**交付物**: AI + 工作流引擎集成 — Dify/N8n 插件化接入

---

### Phase 7 — 音乐管理（预计 2 周）

**目标**: 音乐库扫描、元数据管理、对接 Navidrome

```
依赖: Phase 3 (filesystem / tagging / search 已有)
```

- [ ] `internal/services/media` — 媒体管理模块
  - 音频元数据读写（ID3/FLAC/MP4 via dhowden/tag）
  - 音乐库递归扫描
  - 专辑封面提取与管理
  - 重复文件检测
- [ ] `internal/services/filesystem` — 增强：音乐扫描
  - 扩展名过滤（mp3/flac/wav/m4a）
  - 自动触发扫描（文件变更 → eventbus）
- [ ] `internal/services/tagging` — 增强：音乐标签
  - 音乐风格/年代/评分标签
  - 智能推荐标签
- [ ] `internal/services/search` — 增强：音乐搜索
  - 按歌手/专辑/曲名/风格搜索
  - 模糊匹配

**业务模块开发**:

- [ ] `internal/modules/s3` — 音乐管理模块
  - 音乐库展示（歌手/专辑/曲目）
  - 元数据编辑（批量修改标签）
  - 播放列表管理
  - Navidrome Subsonic API 桥接（可选）

**交付物**: 音乐库管理 — 自动扫描、元数据编辑、全文搜索

---

### Phase 8 — 多端联调与运维（持续）

**目标**: 各客户端接入验证 + 可观测体系

- [ ] 微信小程序联调
- [ ] Android/iOS 联调
- [ ] HarmonyOS 联调
- [ ] Web 前端联调
- [ ] Desktop 应用联调
- [ ] Prometheus + Grafana 大盘配置（独立 Docker Compose）
- [ ] Loki 日志采集
- [ ] 告警规则配置
- [ ] 备份策略验证
- [ ] 性能压测 + 优化
- [ ] `internal/services/discovery` — 多节点部署（按需启用）
- [ ] `internal/services/backup` — 全量备份方案落地

**交付物**: 全端可用 + 可观测体系就绪

---

## 依赖关系图

```
Phase 0 脚手架
  │
  ▼
Phase 1 核心基础设施 ──────────────────────────────────┐
  │                      │          │          │        │
  ▼                      ▼          ▼          ▼        ▼
Phase 2          Phase 3     Phase 5    Phase 6   Phase 7
社交登录          物品管理     智能家居    自动化     音乐管理
  │                │          │          │
  ▼                ▼          ▼          ▼
            Phase 4 知识管理   └── 均复用 Phase 1 基础设施 ──► Phase 8 多端联调+运维
```

## 版本规划

| 版本 | 时间 | 内容 |
|------|------|------|
| v0.1.0 | 第 1 周 | Phase 0 脚手架 |
| v0.2.0 | 第 2-4 周 | Phase 1 核心基础设施 |
| v0.3.0 | 第 5-6 周 | Phase 2 社交登录 |
| v0.4.0 | 第 7-10 周 | Phase 3 物品管理 MVP |
| v0.5.0 | 第 11-12 周 | Phase 4 知识管理 |
| v0.6.0 | 第 13-14 周 | Phase 5 智能家居 |
| v0.7.0 | 第 15-16 周 | Phase 6 自动化工作流 |
| v0.8.0 | 第 17-18 周 | Phase 7 音乐管理 |
| v1.0.0 | 第 19-20 周 | Phase 8 多端联调 + 运维就绪 |

## 仓库管理策略

### Monorepo（推荐，覆盖全阶段）

```
go-backend-core/
├── internal/
│   ├── services/      # 全部公共模块
│   └── modules/
│       ├── s0/        # 调试
│       ├── s1/        # 物品管理
│       ├── s2/        # 知识管理
│       └── s3/        # 音乐管理
├── pkg/               # 核心库
└── ...
```

### 拆分时机

- 任一模块代码量超过 1.5 万行
- 需要独立扩缩容（如 AI 服务需要 GPU 节点）
- 团队分工明确

## 技术债务管理

- 每次提交前运行 `make lint`
- 所有公开函数必须有注释
- 新功能必须附带测试
- 代码覆盖率目标 ≥ 70%
- 每周 Code Review
