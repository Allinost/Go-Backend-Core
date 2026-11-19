# Go-Backend-Core V3 规划：PostgreSQL 基座与项目化管控

> 版本：v1.0（新增配置中心：能配置的项目/能力尽量可配置；多主机/备份/外部服务排期不变）
> 日期：2026-11-18
> 分支：`feature-V3`（从 `feature-V2` 当前 HEAD 拉出）
> 状态：**范围、上线批次与边缘/1Panel 方案已确认，待 P0 启动**
> 关联台账：[docs/projects/go-backend-core/v3/README.md](../projects/go-backend-core/v3/README.md)
> V3 SOP：[docs/projects/go-backend-core/v3/SOP.md](../projects/go-backend-core/v3/SOP.md)
> 适用仓库：`Go-Backend-Core`（本仓库）、`Go-Frontend-Core`（V3 管控页面）、`NKUI`（前端模板）

---

## 0. 一页纸摘要

### 0.1 结论

V3 采用 **“新机器、PG-only、基座先行、V2 并行、业务模块后迁移”** 的路线：

- **V3.0 只做基座 + 公共能力 + 项目管理模块**；`goodser / ia / login3rd / shopchecker` 暂不进入 V3 构建与部署，继续在 V2 老机器运行。
- V3 部署到新机器，使用 PostgreSQL 16+；V2 老机器继续使用 MySQL 提供现有业务，**老机器在 V3 稳定并完成业务模块迁移后才下线**。
- 不做一次性大迁移。V3.0 只迁移“基座 + 公共能力 + 项目管理”所需数据；业务模块按模块逐个迁移（V3.1 goodser、V3.2 ia、V3.3 login3rd、V3.4 shopchecker），每个模块迁移时再做数据冻结、增量同步、路由切换。
- GORM **保留**：继续用于 auth/RBAC/API Key/scheduler 等 CRUD 场景，但切换到 PostgreSQL 驱动，**生产禁用 AutoMigrate**，Schema 由迁移唯一管理。
- 项目管理按**完整规划**建设：里程碑、任务/WBS、依赖、任务日志、风险、发布、基线、迭代、评论、检查项、标签、任务资产、路线图、看板、燃尽、导入导出。
- V3 由 **AI 主执行、人工评审与验收**：任务契约、上下文包、自动测试、子代理评审、人工合并、台账留痕。详见 V3 SOP。
- **V3 开放公网**：新建一个**二级域名指向新机器**；新机器上的 **1Panel v2** 管理证书/HTTPS/反向代理，回源本机 V3 容器 80；管理员创建公网用户并配置角色/权限；默认不开放自助注册。
- **V2 保持原样**：老二级域名仍指向老机器，V2 继续运行；V2 全面冻结，只保留运行与必要安全修复。
- **上线目标 2026-12-15**：**核心上线**已确认；PM-07 高级前端 12/31 前补齐。
- **1Panel 接入暂停**：1Panel API/数据源/MCP/多实例/密钥入库**暂不进入 V3.0**，后续单独立项；V3.0 不依赖 1Panel API。
- **公网反代/证书仍需要**：实现方式待定（1Panel 边缘或 OpenResty/Nginx 均可）；V3 应用只依赖容器内 80、转发头、HTTPS 终止与域名。
- **V3.0 主线收敛**：PG-only 数据层、基座/公共能力、完整项目管理、多主机台账（最小）、公网部署、数据 bootstrap、测试/CI、SOP/AI 开发。
- **多主机管理（新增）**：云服务器 + 主机 + 备份机；V3.0 先做主机台账、SSH 连通/状态、权限/审计；完整运维操作后续。
- **数据库迁移/备份（低优先级）**：主机间迁移、冷备份；等容器迁移到新机器后再做，列入 V3.1，不进 12.15。
- **外部服务（低优先级）**：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS 等，列入 V3.2，按需接入；已有能力保留，不阻塞 12.15。
- **模块调整**：`s0` 改名为 `health`（保留 ping/health/metrics/info/modules）；删除 `s1/s2` 占位模块。
- **配置中心（新增架构约束）**：模块声明配置 Schema；统一默认值、校验、优先级、热更新、审计、密钥脱敏、管理 API；运行参数尽量 DB 动态化，启动/安全密钥仍走 Secret。
- **V2 冻结**：V3 是唯一开发线；老机器/老域名继续运行 V2。
- **上线目标 2026-12-15 核心上线**：PM-07 高级前端 12/31 前补齐。
- **分支**：已切换到 `feature-V3` 开发。

### 0.2 原始需求与第二轮决策

**原始需求（2026-11-18）：**

> 我现在想重构Go-Backend-Core到V3
> 1 新版本重构至PGSQL代替MYSQL
> 2 总结现在后端的基础模块、公共能力和模组模块
> 3 我希望Go-Backend-Core的项目管理能完整追踪和规划Go-Backend-Core V3
> 帮我做一份规划

**第二轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 1 | V3 确定 **PG-only**。 |
| 2 | V3 部署到**新机器**，V2 继续在**老机器**运行。 |
| 3 | 老机器在 V3 稳定后才下线；V3.0 不安排老机器下线。 |
| 4 | GORM **保留**，需要详细方案。 |
| 5 | 项目管控**按完整规划**建设，不做缩水 MVP。 |
| 6 | V3 由 **AI 开发**（AI 主执行 + 人工评审）。 |
| 7 | 保持 `/api/v1`；前端后续也会重构。 |
| 8 | 业务模块**先不切换**。当前优先做 V3 SOP、基础能力、公共能力、项目管理模块；项目管理做好后，再逐步重构业务模块，业务模块也会有需求修改与新增。 |

**第三轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 9 | V3 需要**开放公网**：OpenResty 反代，管理员给公网用户配置权限。 |
| 10 | V3 域名未定；容器内端口 **80**；HTTPS 由 OpenResty 处理。 |
| 11 | **V2 全面冻结**：V2 只运行与必要安全修复，不再新增功能；V3 是唯一开发线。 |
| 12 | V3 上线预期 **2026-12-15**。 |
| 13 | 可考虑接入 **1Panel V2**（Docker、数据库管理、MCP 管理、系统监控等），作为可选外部能力适配；V3.0 不阻塞上线，先调研/预留。 |

**第四轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 14 | 12.15 按**核心上线**执行；PM-07 高级前端 12/31 前补齐。 |
| 15 | **证书与反向代理都由 1Panel 管理**；V3 不手工维护 OpenResty 配置。 |
| 16 | 公网用户**先由管理员创建**，后续再完善注册/审批/验证流程。 |
| 17 | ~~1Panel（V2 版本）仍在老机器，回源到新机器~~ → 已被第五轮修正。 |

**第五轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 18 | **新建一个二级域名指向新机器**；老二级域名仍指向老机器，V2 保持原样。 |
| 19 | **新机器已部署 1Panel v2**；V3 直接使用新机器 1Panel v2 作为边缘、数据来源、MCP 支持。 |
| 20 | 新旧机器在**同一局域网**、可互访；用于 bootstrap/运维，不用于生产反向代理。 |
| 21 | 1Panel 数据源/MCP **先只读 + 工具白名单**；受限操作后续再开，且必须审计/二次确认。 |

> **修正后的拓扑**：新二级域名 → 新机器公网 IP → 新机器 1Panel v2（证书/HTTPS/反代）→ 本机 V3:80。老二级域名 → 老机器 → V2。两机同局域网，仅用于数据迁移/运维访问；生产入口不跨机器回源。

**第六轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 22 | 1Panel **不止一台**，后续需要多实例管理；API Key **存数据库并加密**，不写配置文件/代码。 |
| 23 | 当前云服务器 1Panel：`gtp.hailong.site`，Key 名 `GTPanel`（值不落文档；放 Secret/DB）。 |
| 24 | 实测 API：带 Key 返回 `401 API 接口禁止访问`；不带 Key 返回“会话已过期”。需启用 API/放行 IP 后复测。 |
| 25 | 先按 **1Panel-skills** 的只读模块（monitoring/containers/websites/apps/logs/cronjobs/task-center/nodes）接入。 |
| 26 | 域名仍未定，**不阻塞 P0/P1**；P8-06 边缘配置等域名就绪后再做。 |

**第七轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 27 | **1Panel 接入暂停**：API/数据源/MCP/多实例/密钥入库均不进入 V3.0，后续单独立项。 |
| 28 | 公网反代/证书仍需要，但实现方式待定（1Panel 边缘或 OpenResty/Nginx）；V3 只依赖容器 80 + 转发头 + HTTPS。 |
| 29 | V3.0 不排 1Panel 任务；P0-04/P8-07/P8-08 移出本期，相关文档保留为后续评估资料。 |
| 30 | V3.0 重新聚焦：PG-only、基座/公共、完整项目管理、数据 bootstrap、测试/CI、SOP/AI、公网上线。 |

**第八轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 31 | **新增多主机管理**：云服务器 + 主机 + 备份机；V3.0 做主机台账、SSH 连通/状态、权限/审计；完整运维后续。 |
| 32 | **新增数据库迁移/备份**：主机间迁移、冷备份；低优先级，等容器迁移到新机器后做，列入 V3.1。 |
| 33 | **外部服务低优先级**：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS 等，列入 V3.2；已有能力保留。 |
| 34 | **s0 → health**：保留 ping/health/metrics/info/modules；删除 s1/s2 占位模块。 |
| 35 | 本次开发切到 **`feature-V3`** 分支；V2 冻结。 |

**第九轮确认（2026-11-18）：**

| # | 决策 |
|---|---|
| 36 | **能配置的项目/能力尽量可配置**：新增配置中心，模块声明 Schema，统一校验/默认值/优先级/热更新/审计/脱敏。 |
| 37 | 运行参数尽量 DB 动态化；启动/安全密钥仍走 Secret，不入库明文、不在接口回显。 |
| 38 | 配置中心进 V3.0（框架 + 核心配置 + 管理 API）；前端表单可后补，不阻塞上线。 |

### 0.3 范围划分（V3.0 vs 后续）

| 范围 | V3.0（本次） | V3.1+（后续按模块） |
|---|---|---|
| 运行时数据库 | PostgreSQL 16+ | 同左 |
| 部署 | 新机器，与 V2 老机器并行 | 同左，逐步接管业务 |
| 基础模块 | auth/migrate/ops/settings/integration/webhook/servicenav/scheduler/net/crypto/**health**；删除 s1/s2 | 同左 |
| 公共能力 | 23 个公共服务中与 V3.0 相关的部分（auth/audit/cache/config/eventbus/feature/fileref/migrate/monitor/outbox/scheduler/storage/tracing/ws 等） | 按需补齐 |
| 配置中心 | 配置 Schema/默认值/校验/优先级/热更新/审计/脱敏 + 管理 API | 全模块配置注册、前端表单、版本回滚 |
| 多主机 | 主机台账（云服务器/主机/备份机）+ SSH 连通/状态 + 权限/审计（最小） | 完整运维、批量操作、监控 |
| DB 迁移/备份 | 不在 V3.0 | V3.1：主机间迁移、冷备份（低优先级） |
| 外部服务 | 不在 V3.0 | V3.2：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS（低优先级） |
| 项目管理 | 完整项目管理（里程碑/任务/依赖/风险/发布/基线/迭代/看板/报表） | 持续增强 |
| 业务模块 | **不进入 V3 构建与部署** | goodser → ia → login3rd → shopchecker，逐个迁移 |
| 前端 | 保持现有前端；新增/完善项目管理页面 | 前端重构另行立项 |
| 数据迁移 | 只迁移基座 + 公共能力 + 项目管理数据 | 每个业务模块迁移时单独做数据迁移 |
| 老机器 | 继续运行 V2，不下线；V2 功能冻结 | 全部模块迁移并稳定后下线 |
| 暴露方式 | 新二级域名 → 新机器反代/证书（1Panel 或 OpenResty，待定）→ 本机 V3:80 | 新域名不变，按模块切路由 |
| 老入口 | 老二级域名 → 老机器 → V2，不变 | 模块迁移后逐步切换 |
| 公网权限 | 管理员创建用户并配置角色/权限；默认不开放自助注册；后续完善注册/审批 | 按模块细化 |
| 上线日期 | 2026-12-15 核心上线；PM-07 12/31 前补齐 | 按模块里程碑 |
| 1Panel 接入 | **暂停，不进入 V3.0** | 后续单独立项（API/数据/MCP/多实例/密钥入库） |
| 域名 | 待定，不阻塞 P0/P1 | 就绪后配置反代/证书 |

### 0.4 V3.0 成功标准

1. 新机器上的 V3 只连接 PostgreSQL；运行时无 MySQL 连接。
2. V3.0 二进制只注册基座 + 公共能力 + 项目管理 + 多主机台账；业务模块不进入构建/路由；**s0 改名 health，s1/s2 移除**。
3. `migrations/postgres` 可从空库一键建出 V3.0 约 35 张基础表 + 13 张项目管理扩展表 + 3 张运维/配置表；迁移可重复、可校验、可回滚。
4. V3.0 基座/公共/项目管理数据从 V2 完成一次性 bootstrap 迁移，校验通过；业务模块数据留在 V2。
5. GORM 在 PG 上正常工作；生产 `auto_migrate=false`，Schema 只由迁移管理。
6. 项目管理完整可用：里程碑、任务、依赖、日志、风险、发布、基线、迭代、评论、检查项、标签、任务资产、路线图、看板、燃尽、导入导出。
7. V3 SOP 落地：AI 开发流程、PG 规范、模块接入流程、测试/验收/迁移/部署清单可执行。
8. V3 新机器连续稳定运行 7 天；V2 老机器保持可用；未到下线条件。
9. `/api/v1` 契约不变；前端后续重构不受阻。
10. 新二级域名通过新机器 HTTPS 反代访问 V3；容器内仅监听 80；管理员创建公网用户并配置角色/权限；默认无自助注册。
11. V2 冻结生效：老二级域名/老机器保持 V2；V2 无新增功能；V3 为唯一开发线。
12. 12.15 **核心上线**完成；PM-07 12/31 前补齐。
13. 反代/证书完成（1Panel 边缘或 OpenResty，实现待定）；V3 只依赖 80 + 转发头 + HTTPS。
14. **1Panel API/数据/MCP/多实例/密钥入库明确不在 V3.0 范围**；不阻塞上线，后续单独立项。
15. 多主机台账可用：云服务器/主机/备份机可登记、SSH 连通/状态可查、权限/审计生效。
16. DB 迁移/备份与外部服务明确列为 V3.1/V3.2 低优先级，不阻塞 12.15。
17. 配置中心可用：核心配置项有 Schema、校验、默认值、热更新、审计、密钥脱敏；管理 API 可用。

---

## 1. 现状盘点（V2 基线）

### 1.1 技术基线

| 项 | 现状 |
|---|---|
| 语言 / 框架 | Go 1.26.4 · Gin 1.12 |
| 运行时主库 | MySQL 8（`database.mysql.main`） |
| 其他数据源 | PostgreSQL（仅 NAS 场景）、Redis、MinIO、RustFS |
| ORM / SQL | GORM（auth/scheduler 等）+ 大量手写 `database/sql` |
| 迁移 | 自研 `migrate` 模块，`migrations/mysql/NNN_*.sql`，`_migrations` 记录版本与 checksum |
| 入口 | 仅 `cmd/server`（文档中的 `cmd/migrate`、`cmd/backup` 实际不存在） |
| 模块 | 18 个已注册模块（文档仍写 15 个，已过期） |
| 公共服务 | 23 个包 |
| 代码规模 | 约 6.0 万行生产代码 + 1.3 万行测试；396 个 Go 文件；946 个 Test 函数、13 个 Benchmark |
| 当前质量门 | `go build ./...` 通过；`go test -short -count=1 ./...` 全绿（2026-11-18 实测） |

### 1.2 代码规模（按目录）

| 目录 | 生产代码行 | 测试行 | 包数 | 说明 |
|---|---:|---:|---:|---|
| `internal/modules` | 8,273 | 1,788 | 14 | 基础/平台模块 |
| `internal/services` | 13,435 | 5,463 | 23 | 公共服务 |
| `internal/database` | 558 | 346 | 6 | 连接池、熔断、健康检查 |
| `internal/middleware` | 391 | 499 | 1 | 日志/CORS/限流/认证/RBAC/指标 |
| `internal/pkg` | 648 | 704 | 5 | 内部工具库 |
| `zzz` | 35,507 | 3,544 | 11 | 业务域模块（V3.0 只保留 developmentManager） |
| `pkg` | 1,278 | 788 | 4 | 公开可复用库 |
| `cmd` | 306 | 0 | 1 | 服务入口与 Swagger 页面 |

### 1.3 基础模块（V3.0 保留）

| 模块 | 路径 | 路由数（约） | 职责 | 与 MySQL 的耦合 | V3 动作 |
|---|---|---:|---|---|---|
| `auth` | `internal/modules/auth` | 36 | JWT 登录/刷新/登出、用户、RBAC、API Key、6 种第三方登录、admin 权限管理 | GORM MySQL 三套 Store（User/RBAC/ApiKey）+ AutoMigrate | PG 驱动 + 显式迁移 |
| `migrate` | `internal/modules/migrate` | 13 | 表列表/结构、dump/restore/transfer/backup、schema 迁移 | 默认优先 MySQLAdapter；`SHOW TABLES`/`SHOW COLUMNS`；MySQL DDL dump | PGAdapter 默认 |
| `ops` | `internal/modules/ops` | 21 | 运维状态、RustFS 文件操作、扫描/清理/统计、文件引用 | 直接使用 MySQL 连接；`information_schema`/反引号 SQL | `database.Main()` + PG |
| `webhook` | `internal/modules/webhook` | 13 | Webhook 配置、订阅、投递、HMAC、去重、限流、重试 | `JSON_CONTAINS(tags, ?)`；MySQL 仓库 | JSONB + 占位符改造 |
| `servicenav` | `internal/modules/servicenav` | 10 | 服务导航条目/类型、从部署接入点导入 | `JSON_CONTAINS(types, ?)`；MySQL 仓库 | JSONB `@>` |
| `integration` | `internal/modules/integration` | 17 | 动作网关、MCP 端点、连接器、外盒事件、幂等、限流 | 动作日志/连接器表；MySQL outbox | PG 仓库 + JSONB |
| `settings` | `internal/modules/settings` | 3 | 应用设置（公开地址、前端参数、Browser/n8n/OpenClaw/LangBot 地址） | 无（JSON 文件 `data/settings.json`） | 不动 |
| `developmentManager` | `zzz/developmentManager` | 66 | 项目台账、关联、部署资产、数据字典、需求、生成、权限矩阵、seed | 15 张表；`SELECT DATABASE()`、`information_schema.COLUMNS`、2 处 `ON DUPLICATE KEY` | 重点改造 + 完整项目管理扩展 |
| `scheduler` | `internal/services/scheduler` | 10 左右 | cron/asynq 任务、执行日志、内置清理任务 | GORM MySQL Store + AutoMigrate；`gorm.Expr("NOW()")` | PG 驱动 + 显式迁移 |
| `net` | `internal/services/net` | 少量 | HTTP/TCP/UDP/DNS/gRPC/熔断/重试 | 无 | 不动 |
| `crypto` | `internal/services/crypto` | 少量 | 加解密、哈希、压缩、密钥管理 | 无 | 不动 |

> **模块调整**：`s0` 改名为 `health`，保留 ping/health/echo/metrics/info/modules 与调试端口能力；`s1/s2` 占位模块删除。

### 1.4 公共能力（V3.0 保留）

| 服务 | 职责 | 数据耦合 | V3 关注点 |
|---|---|---|---|
| `action` | 动作注册与网关调用编排 | 通过各模块 Service | 无 |
| `ai` | OpenAI / Ollama Provider，Chat/Embedding | 无 | 无 |
| `audit` | 审计日志服务，内存/MySQL/SQLite Store | `audit/mysql.go` 自建 MySQL DDL | PG Store + 迁移 |
| `auth` | JWT、密码哈希、用户/RBAC/API Key Store、社交登录 Provider | GORM MySQL Store + 内存实现 | 重点改造 |
| `cache` | 两级缓存、防穿透/雪崩、Redis | Redis | 无 |
| `config` | 配置服务与校验 | 无 | 增加 PG 配置校验 |
| `connector` | Webhook/MQ/OpenClaw 连接器抽象 | 无 | 无 |
| `crypto` | 加解密/压缩/密钥轮换 | 无 | 无 |
| `discovery` | 节点发现（mDNS/Redis/etcd 预留） | Redis | 无 |
| `eventbus` | 本地事件总线 + Redis Stream + 死信 | Redis | 无 |
| `feature` | 特性开关，内存/MySQL/SQLite Store | `REPLACE INTO`、MySQL DDL | PG `ON CONFLICT` |
| `fileref` | 文件引用登记与扫描目标 | `SELECT DATABASE()`、MySQL DDL、`INSERT IGNORE` | 迁移 + PG |
| `migrate` | 迁移引擎、dump/restore/transfer/backup、PG/MySQL Adapter | MySQL 方言为主 | 重点改造 |
| `monitor` | 健康检查、运行时指标、DB 指标 | 遍历连接 | PG 标签 |
| `net` | 网络通信服务 | 无 | 无 |
| `notification` | 邮件/站内通知 | 无 | 无 |
| `outbox` | 外盒事件持久化与分发 | `zzz_integration_outbox`，MySQL Store | PG |
| `scheduler` | 任务调度（同上） | GORM MySQL | 重点改造 |
| `search` | Bleve 全文检索 | 本地索引 | 无 |
| `storage` | S3/MinIO/RustFS 存储抽象 | S3 | 无 |
| `template` | 模板渲染 | 无 | 无 |
| `tracing` | OpenTelemetry 兼容追踪 | 无 | 无 |
| `ws` | WebSocket/SSE Hub | 无 | 无 |

### 1.5 业务模块（V3.0 不切换，留在 V2）

| 模块 | 路由前缀 | 路由数（约） | 主要表 | 依赖 | V3 计划 |
|---|---|---:|---|---|---|
| `goodser` | `/api/v1/zzz-goodser` | 91 | 19 张 `zzz_goodser_*` | MySQL、RustFS、webhook、integration | V3.1 迁移 |
| `ia` | `/api/v1/zzz-ia` | 65 | 13 张 `zzz_ia_*` | MySQL、scheduler、外部行情 HTTP | V3.2 迁移 |
| `login3rd` | `/api/v1/login3rd` | 17 | 2 张 `zzz_login3rd_*` | MySQL、webhook、chromedp | V3.3 迁移 |
| `shopchecker` | `/api/v1/shopchecker` | 11 | 3 张 `zzz_shopchecker_*` | MySQL、webhook、collector、scheduler | V3.4 迁移 |

> V3.0 代码库可以保留这些包以便后续移植，但 `cmd/server` **不注册**它们；V3 二进制不包含业务模块路由。建议在 `feature-V3` 上先将它们从 `cmd/server` 的 import/注册表移除，后续按模块从 V2 分支移植并重构。

### 1.6 数据与迁移现状

| 项 | 现状 |
|---|---|
| 迁移目录 | `migrations/mysql/001–017`（17 个文件）；`migrations/postgres/001_init.sql` 仅建 `_migrations` |
| DDL 表 | 59 张（`008` 建 30 张后 `009` 重命名 6 张；`002–017` 新增） |
| GORM 自动建表 | 约 12 张（users/roles/permissions/role_permissions/user_roles/api_keys/scheduler_tasks/scheduler_task_logs/feature_flags/audit_log/file_references/scan_targets） |
| 合计（V2 全量） | 约 **71 张表** |
| V3.0 需要 | 约 **35 张基础/公共/PM 表** + PM 扩展 13 张 + 运维/配置 3 张 ≈ **51 张** |
| 业务模块表 | goodser 19 + ia 13 + login3rd 2 + shopchecker 3 = 37 张，留在 V2，后续按模块迁移 |

### 1.7 MySQL 耦合清单（V3.0 范围）

| 类别 | 位置/数量 | 说明 | PG 替代 |
|---|---|---|---|
| 主库连接 | `database.mysql.main` + 20 处 `database.DB.MySQL[...]` | 13 个模块/服务直接取 MySQL | `database.Main()` / `database.PG()` |
| GORM 驱动 | 4 个 Store（User/RBAC/ApiKey/Scheduler） | `gorm.io/driver/mysql` + AutoMigrate | `gorm.io/driver/postgres` + 显式迁移 |
| 占位符 | 基座/公共/项目管理仓库约 15 个文件、数百处 `?` | `database/sql` 写法 | 逐语句改 `$1..$n`（codemod + 人工复核） |
| UPSERT | 项目管理 2 处、auth/scheduler 少量 | `ON DUPLICATE KEY UPDATE` | `ON CONFLICT (...) DO UPDATE ... EXCLUDED` |
| JSON 查询 | webhook/servicenav 4 处 `JSON_CONTAINS` | 标签/类型过滤 | `jsonb @> $1::jsonb` |
| 日期函数 | auth/scheduler/ops 少量 `NOW()` | 时间字段 | `now()` / `timestamptz` |
| 自建 DDL | `audit/mysql.go`、`fileref/mysql.go`、`migrate/schema.go`、`migrate/adapter.go`、`migrate/dumper.go` | `AUTO_INCREMENT`、`ENGINE=InnoDB`、`utf8mb4`、`TINYINT(1)`、`ON UPDATE CURRENT_TIMESTAMP` | 统一迁到 `migrations/postgres` |
| 元数据查询 | `SELECT DATABASE()` 2 处；`information_schema` 3 处；`SHOW TABLES/COLUMNS` 2 处 | DM 字典扫描、fileref、migrate | `current_database()`/`current_schema()`、`pg_catalog` |
| REPLACE/IGNORE | `REPLACE INTO`（feature）、`INSERT IGNORE`（fileref） | 特性开关、扫描目标 seed | `INSERT ... ON CONFLICT DO UPDATE/NOTHING` |
| 反引号 | ops 动态扫描 SQL、migrate MySQLAdapter | MySQL 标识符引用 | 双引号或统一走 `Quote()` |
| 布尔/时间 | `enabled = 1`、`DATETIME`、`NOW()`、`ON UPDATE CURRENT_TIMESTAMP` | scheduler/各表 | `boolean`、`timestamptz`、应用层/触发器 |

### 1.8 现状问题（不只是 MySQL）

1. **文档与实现漂移**：README/architecture 写 15 个模块、`cmd/migrate`/`cmd/backup`，实际 18 个模块、仅 `cmd/server`。
2. **建表双轨制**：一部分走 `migrations/`，一部分靠 GORM `AutoMigrate`，还有 `fileref`/`audit` 启动时自建表；生产环境不可审计、不可回滚。
3. **测试数据库与生产不一致**：`audit`/`feature` 的 Store 测试用 SQLite，SQLite 能过的 SQL 在 PG 上未必能过。
4. **SQL 分散**：同一类分页/IN 查询/JSON 查询在多个仓库重复实现，迁移时容易漏改。
5. **无 PG 集成测试**：当前 CI 只有 `go test -short`，没有真实 PG 服务。
6. **项目管控只到需求级**：developmentManager 已有项目/需求/版本/资产/生成，但缺少里程碑、任务、依赖、风险、发布、进度，无法完整追踪 V3 这种多阶段工程。
7. **双环境并行缺少规范**：V2 老机器与 V3 新机器并存时，路由、认证、数据归属、模块切换都需要明确 SOP。

### 1.9 现状结论

V3.0 的范围比“全量重构”小得多，风险也更可控：只改基座/公共/项目管理的数据层，业务模块继续在 V2 运行。真正的工作量集中在 **PG Schema 基线、基础/公共模块方言改造、项目管理完整模型、双环境并行 SOP、AI 开发流程**。业务模块迁移作为后续独立里程碑，按模块逐个做。

---

## 2. V3 目标与非目标

### 2.1 目标

1. **V3 新机器 PG-only**：PG 16+ 作为唯一主库；V3 运行时不连接 MySQL。
2. **基座先行**：V3.0 完成基座、公共能力、项目管理模块；业务模块不切换。
3. **Schema 基线化**：V3.0 约 35 张基础表 + 13 张 PM 扩展表 + 3 张运维/配置表，全部由迁移创建。
4. **GORM 保留**：PG 驱动 + 关闭生产 AutoMigrate；保留现有 CRUD 开发效率。
5. **项目管理完整化**：里程碑/任务/依赖/日志/风险/发布/基线/迭代/评论/检查项/标签/资产/路线图/看板/燃尽/导入导出。
6. **SOP 与 AI 开发**：形成可执行的 V3 SOP，AI 主执行、人工评审、自动验收。
7. **双环境并行**：V3 新机器与 V2 老机器同时运行，老机器在 V3 稳定并完成模块迁移后下线。
8. **契约稳定**：保持 `/api/v1`；前端后续重构不被数据库改造阻塞。
9. **文档同步**：架构、技术选型、数据模型、部署、模块文档与代码同提交。
10. **公网可用**：1Panel 托管反代/证书/HTTPS；容器内 80；管理员创建公网用户并配置权限；默认不开放自助注册。
11. **V2 冻结**：V2 只运行与安全修复；V3 为唯一开发线。
12. **12.15 上线**：按批次交付，核心能力 12.15 上线，非关键增强 12.15 后补齐。
13. **1Panel 边缘必需 + API 可选**：1Panel 托管反代/证书；Docker/DB/MCP/监控 API 完成调研与适配边界，不阻塞 V3.0 上线。

### 2.2 非目标（V3.0 不做）

- 不迁移 `goodser / ia / login3rd / shopchecker` 的代码、数据与路由。
- 不做 V2 → V3 全量数据大迁移；只做基座/公共/项目管理 bootstrap。
- 不关闭 V2 老机器；V3.0 不安排老机器下线。
- 不重写业务逻辑、不改 `/api/v1` 契约。
- 不做 MySQL/PostgreSQL 运行时双写或双活。
- 不强制把所有 GORM 改成手写 SQL。
- 不在 V3.0 内完成前端整体重构；只做项目管理页面与必要适配。
- 不追求一次性清完历史债务；P9 只做 V3.0 上线与并行运行。
- 不在 V3.0 内完成 1Panel 全部能力接入；只做调研、接口预留与可选的只读适配。
- 不开放公网自助注册；公网用户由管理员创建/授权（如需注册，另行设计审批流）。
- 不在 V2 上新增功能；V2 只允许安全/数据修复。

---

## 3. 目标架构与部署拓扑

### 3.1 双环境并行

```
       老二级域名                              新二级域名
            │                                       │
            ▼                                       ▼
┌─────────────────────────────┐        ┌─────────────────────────────────────┐
│  老机器（V2，冻结）          │        │  新机器（V3.0，PG-only）             │
│                             │        │                                     │
│  反代/证书（现有，保持不变） │        │  反代/证书（1Panel 或 OpenResty，待定）│
│   ├─ 站点/证书/HTTPS         │        │   ├─ 站点/证书/HTTPS                 │
│   └─ 反代 → V2:80            │        │   └─ 反代 → 本机 V3:80               │
│                             │        │                                     │
│  Go-Backend-Core V2         │        │  （1Panel API/数据/MCP 接入暂停）     │
│   ├─ 基础模块/公共能力       │        │                                     │
│   ├─ goodser / ia / login3rd│        │  Go-Backend-Core V3  :80（容器内）    │
│   └─ shopchecker            │        │   ├─ 基础模块（PG）                   │
│                             │        │   ├─ 公共能力（PG）                   │
│  MySQL 8 / Redis / S3        │        │   ├─ developmentManager              │
│                             │        │   └─ 完整项目管理扩展                 │
│  仅运行/安全修复，不新增功能  │        │                                     │
└──────────────┬──────────────┘        │  PostgreSQL 16+ / Redis / S3         │
               │                       │  只开 80（本机/内网）                 │
               │  同一局域网，可互访    └─────────────────────────────────────┘
               └─────────────── 用于 bootstrap / 运维，不做生产反代 ───────────┘

后续（V3.1+）：按模块把业务从老机器“绞杀式”迁移到 V3
  goodser → V3.1
  ia      → V3.2
  login3rd→ V3.3
  shopchecker → V3.4
  全部稳定后，评估老机器下线（老域名/老 1Panel 另行迁移）
```

> 关键点：**新二级域名/证书/HTTPS 在新机器边缘**（1Panel 或 OpenResty 实现待定）；V3 只监听容器内 80。老机器保留老域名/现有反代/V2，两者互不跨机器回源。1Panel API/数据/MCP 接入暂停，不影响公网入口。

### 3.2 架构原则

1. **一个 V3 主库**：V3 运行时只有 PG；MySQL 只存在于老机器 V2。
2. **一个 V3 Schema 源**：V3.0 的基座/公共/项目管理表全部由 `migrations/postgres` 创建；生产禁用 AutoMigrate。
3. **一个占位符风格**：V3 运行时 SQL 使用 `$1..$n`。
4. **一个时间标准**：数据库 `timestamptz`，应用 UTC。
5. **一个测试基线**：V3 仓库层必须有真实 PG 集成测试。
6. **一个追踪入口**：V3 所有需求/任务/依赖/风险/发布/进度在 developmentManager 维护。
7. **一个模块切换 SOP**：业务模块迁移必须按 V3 SOP 执行，不允许直接改老机器或跳过验收。
8. **契约优先**：`/api/v1` 不变；V3 新能力通过新增端点/模块扩展，不破坏旧客户端。

### 3.3 认证与数据归属（并行期）

| 项 | 决策 |
|---|---|
| JWT | V3 使用独立 `jwt_secret`（不再与 V2 共用）；V2 冻结后无跨环境 token 需求 |
| 用户/RBAC | **V3 为唯一用户/权限事实源**；bootstrap 一次性复制 V2 的 users/roles/permissions/role_permissions/user_roles/api_keys；之后由 V3 管理员维护 |
| 公网用户 | V3.0 **先由管理员创建用户并分配角色**；后续再完善注册/审批/邮箱或手机验证；默认不开放自助注册 |
| 登录安全 | 限流、失败锁定、审计、可选验证码；1Panel/反代层再加 IP/频率限制 |
| 项目管理数据 | **V3 为项目管理唯一事实源**；V2 developmentManager 在上线前冻结并只读 |
| 业务数据 | 留在 V2；V3.0 不读写业务模块表 |
| 文件存储 | V3 使用独立 bucket/前缀；bootstrap 只复制 PM 相关附件 |
| 路由 | **新二级域名 → 新机器反代/证书（待定）→ 本机 V3:80**；老二级域名 → 老机器 → V2，互不影响 |
| 外部能力 | V3.0 不接入 1Panel API/数据/MCP；1Panel 只可能作为边缘实现之一（或改用 OpenResty） |
| 局域网 | 新旧机器同局域网可互访，仅用于 bootstrap/迁移/运维；生产反代不跨机器 |

> **认证策略变化（第三轮）**：V3 要开放公网，因此不能假设“V3 只对内”。V3 必须自己掌握用户/RBAC，管理员为公网用户配置权限；V2 冻结后不再作为身份源。bootstrap 只做一次性迁移，不做双向同步。

### 3.4 公网反代 / 证书规范（实现待定）

> **V3.0 只要求公网反代/证书可用**；1Panel 或 OpenResty/Nginx 均可，具体实现 P8 前确定。V3 应用不手工维护反代配置，只提供上游地址与必需的头/端口约定。

| 项 | 规范 |
|---|---|
| 边缘位置 | 新机器；新二级域名解析到新机器公网 IP；HTTPS 在新机器终止 |
| 监听 | 443（HTTPS），80 跳转 443 |
| 上游 | 反代到本机 V3 `http://127.0.0.1:80` 或容器内网地址；不跨机器回源 |
| 网络 | 新机器只开 443 公网 + 本机/内网 80；老机器仅用于 bootstrap/运维访问 |
| 域名 | 待定；上线前配站点 + DNS |
| 证书 | 1Panel 或 OpenResty/Let's Encrypt；启用 HSTS（确认后） |
| 转发头 | `Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto https` |
| Gin | `TrustedProxies` 只填反代出口地址；`server.public_base_url` 设为新二级域名 |
| WebSocket | `proxy_http_version 1.1`、`Upgrade`/`Connection` 头；读超时 ≥300s |
| 上传 | `client_max_body_size` 按附件需求（建议 50–100MB） |
| 超时 | `proxy_connect_timeout 10s`、`proxy_read_timeout 300s` |
| 安全头 | `X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`、CSP（按前端） |
| 限流 | 登录/写接口在反代层加 `limit_req`；应用层保留限流 |
| 管理入口 | `/api/v1/auth/admin/*`、`/api/v1/ops/*`、`/api/v1/migrate/*` 加 IP 白名单/内网限制 |
| 健康检查 | `/api/v1/health/health` 仅内网/回源可访问；调试端口 29092 不暴露公网 |
| 管理面板 | 若用 1Panel，面板**不对公网**；仅内网/VPN/IP 白名单 |
| 日志 | 反代 access/error log 接入监控；保留真实 IP |
| CORS | 只允许正式前端域名；不使用 `*` 携带凭证 |

> **端口边界**：新机器公网只开 443；V3 容器内 80 仅本机/内网访问；29090/29092、管理面板不对公网。老机器继续服务老域名/V2，不受影响。

### 3.5 L1/L2 维护切换（手动模式）

> **自动 fallback 默认关闭**（`database.failover_mode=manual`）。切换由系统内“开始维护/结束维护”显式触发，避免脑裂。

| 节点 | 角色 | 网络 |
|---|---|---|
| L1 | 主机，运行生产 V3 + PG/Redis | 内网；对云/公网暴露 |
| L2 | 备份机，维护时启动 V3 + PG/Redis | 与 L1 内网；对云/公网暴露 |
| ACloud | 云服务器，RustFS 固定在此 | 与 L1/L2 走公网 |

**服务归属（决定是否随维护切换）：**

| 归属 | 服务 | 维护时 |
|---|---|---|
| L1 | V3 主入口、PostgreSQL、Redis、L1 上的 Browser/外部工具 | **切到 L2** |
| L2 | 维护期间的 V3、PostgreSQL、Redis | 维护时启用 |
| ACloud | RustFS、n8n、OpenClaw、LangBot、MCP、TTS 等云端能力 | **固定不切** |

> 判断规则：URL/连接指向 **L1** 的，维护时切到 L2；指向 **ACloud** 的保持不变。

**三种维护模式：**

| 模式 | 流程 | 反向同步 | 适用 |
|---|---|---|---|
| `shutdown` 停机 | 直接停止 L1，不启用 L2 | 不需要 | 短维护、可接受停机 |
| `migrate_return` 迁移并迁回 | L1→L2 同步确认 → L2 运行 → L2→L1 反向同步确认 → 回 L1 | **必须** | 计划维护、最终回 L1 |
| `migrate_no_return` 迁移不迁回 | L1→L2 同步确认 → L2 长期运行，L1 停用/重建 | 不需要 | L1 退役/迁移到 L2 |

**流程：**

1. **开始维护**（L1 上点击）
   - 选择模式；
   - `migrate_*` 需勾选“L1→L2 已同步”；
   - `shutdown` 直接停止 L1，不进 L2；
   - 记录模式、操作人、时间、同步确认。
2. **维护期间**
   - `migrate_*`：在 L2 启动 V3（`APP_PROFILE=backup`），连接 L2 的 PG/Redis；
   - L1 绑定的 URL 切到 L2；ACloud 固定 URL 不变；
   - 云服务器访问 L2 走公网。
3. **结束维护**
   - `shutdown`：直接结束，启动 L1；
   - `migrate_return`：先完成 **L2→L1 反向同步** 并确认，再停 L2、启动 L1（`APP_PROFILE=host`）；
   - `migrate_no_return`：不反向同步，L2 继续作为主；L1 保持停用/进入退役流程。

**状态机：**

```
normal(L1)
  ├─ start(shutdown) → shutdown
  │     └─ end() → normal(L1)
  ├─ start(migrate_return, sync_confirmed) → maintenance_l2
  │     └─ end(sync_back_confirmed) → normal(L1)
  └─ start(migrate_no_return, sync_confirmed) → maintenance_l2_permanent
        └─ end() → normal(L2)  // L1 退役
```

**API（规划）：**

```
GET  /api/v1/ops/maintenance
POST /api/v1/ops/maintenance/start   {mode: shutdown|migrate_return|migrate_no_return, sync_confirmed, note}
POST /api/v1/ops/maintenance/end     {sync_back_confirmed, note}
```

- 权限：`ops.maintenance.read/start/end`（管理员）。
- 状态持久化：`zzz_ops_maintenance`（P2 建表）+ 审计。
- 前端：系统维护页，显示模式、当前节点、活动目标、同步确认、反向同步、URL 归属提示。
- **不自动启动 L2 进程**：由运维在 L2 启动 V3；系统负责状态、校验、审计与提示。
- **不做运行中热切连接**：L1/L2 各自由自己的 V3 进程连接本机服务。

---

## 4. 关键设计决策（D1–D34）

| # | 议题 | 决策 | 说明 |
|---|---|---|---|
| D1 | 数据库策略 | **PG-only** | 已确认；V3 运行时无 MySQL |
| D2 | 驱动 | `pgx/v5` + `pgx/v5/stdlib` | 新代码用 `pgxpool`；现有 `*sql.DB` 仓库用 stdlib |
| D3 | 占位符 | 逐语句改 `$n` | 不做运行时 `?` 改写；避免 jsonb `?` 歧义 |
| D4 | Schema 起点 | V3.0 基线：基座/公共/PM（约 35）+ PM 扩展 13 + 运维/配置 3 | 业务表在 V3.1+ 按模块新增迁移 |
| D5 | 类型映射 | 见 §5.1 | identity/boolean/timestamptz/jsonb/numeric |
| D6 | GORM | **保留** + PG 驱动 + 生产关闭 AutoMigrate | 详见 §4.1 |
| D7 | JSON | `jsonb` + GIN | 替代 `JSON_CONTAINS`/`JSON_EXTRACT` |
| D8 | 时间 | `timestamptz` + UTC | 连接参数 `TimeZone=UTC` |
| D9 | 数据迁移 | V3.0 只 bootstrap 基座/公共/PM；业务模块按模块迁移 | 不做全量大迁移 |
| D10 | 老机器 | V3.0 永不下线；全部模块迁移并稳定后评估 | 已确认 |
| D11 | 测试 | 真实 PG 集成测试；SQLite 仅纯逻辑 | 方言问题必须由 PG 暴露 |
| D12 | 项目管控 | **完整规划**：里程碑/任务/依赖/日志/风险/发布/基线/迭代/评论/检查项/标签/资产 | 详见 §8 |
| D13 | API 版本 | 保持 `/api/v1` | 前端后续重构不影响 API 契约 |
| D14 | V3.0 构建范围 | 业务模块不注册、不进入 V3 二进制 | 代码保留在 V2 分支，后续按模块移植 |
| D15 | 项目管理归属 | V3 为 PM 唯一事实源；V2 PM 冻结 | 避免双写冲突 |
| D16 | 认证归属 | **V3 为唯一用户/RBAC 事实源**；bootstrap 一次性复制；之后 V3 管理员维护 | V2 冻结后不再作为身份源 |
| D17 | 公网暴露 | **反代/证书**：443 终止 → 新机器 V3:80；实现方式待定（1Panel 边缘或 OpenResty） | 域名待定；详见 §3.4 |
| D18 | 开发方式 | **AI 主执行 + 人工评审** | 详见 §4.2 与 V3 SOP |
| D19 | 公网用户 | V3.0 管理员创建用户并授权；后续再完善注册/审批/验证 | 默认无自助注册 |
| D20 | V2 状态 | **全面冻结**：只运行 + 安全/数据修复，不新增功能 | V3 是唯一开发线 |
| D21 | 上线日期 | **2026-12-15 核心上线**；PM-07 12/31 前补齐 | 已确认 |
| D22 | 公网边缘 | 反代/证书必需；1Panel 或 OpenResty 均可 | V3 只依赖 80 + 转发头 + HTTPS |
| D23 | 权限模型 | 管理员创建用户并分配角色/权限；敏感操作二次确认 + 审计 | 复用现有 RBAC + 扩展资源×动作 |
| D24 | 1Panel 接入 | **暂停，不进入 V3.0** | 后续单独立项 |
| D25 | 1Panel 多实例/Key | 暂停；后续按“实例表 + Key 加密入库”设计 | 不阻塞 V3.0 |
| D26 | 新/老域名 | 新二级域名 → 新机器 V3；老二级域名 → 老机器 V2 | 并行期两套入口 |
| D27 | 1Panel 数据/MCP | 暂停；后续按 1Panel-skills 只读模块评估 | 不阻塞 V3.0 |
| D28 | 多主机管理 | V3.0 做主机台账 + SSH 连通/状态；完整运维 V3.1 | 云服务器/主机/备份机 |
| D29 | 主机凭据 | 加密入库（AES-GCM），只回提示；SSH 指纹校验 | 禁止明文/日志 |
| D30 | DB 迁移/备份 | **V3.1 低优先级**：主机间迁移 + 冷备份；等容器迁移后做 | 不进 12.15 |
| D31 | 外部服务 | **V3.2 低优先级**：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS | 已有能力保留 |
| D32 | 模块精简 | **s0 改名 health**；删除 s1/s2 占位模块 | 保留探活能力，减少占位模块 |
| D33 | 配置中心 | 模块声明 Schema；默认值/校验/优先级/热更新/审计/脱敏 + 管理 API | 运行参数 DB 动态化；密钥走 Secret |
| D34 | L1/L2 维护 | **手动维护模式**：开始/结束维护显式点击；L1→L2 人工同步确认；切过 L2 则 L2→L1 反向同步确认；RustFS 固定在 ACloud | 默认关闭自动 fallback |

### 4.1 GORM 保留方案（详细）

**结论：保留 GORM，但把它降级为“CRUD 映射工具”，不再承担 Schema 管理。**

#### 4.1.1 连接方式

```go
// internal/services/auth/store.go（示例）
import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func NewPostgresUserStore(sqlDB *sql.DB) (*PostgresUserStore, error) {
    db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
        Logger: gormlogger.Default.LogMode(gormlogger.Warn),
        // 关闭默认复数化以外的隐式行为，保持现有表名
        NamingStrategy: schema.NamingStrategy{SingularTable: false},
    })
    if err != nil {
        return nil, err
    }
    return &PostgresUserStore{db: db}, nil
}
```

- `sqlDB` 来自 `internal/database` 的 `Main()`（pgx stdlib），不单独建连接池。
- 不在 GORM 里开第二个数据库连接；GORM 与手写 SQL 共享同一个 `*sql.DB`/事务。
- 需要事务时用 `db.Transaction(func(tx *gorm.DB) error { ... })`，或从 `*sql.DB` 开 `*sql.Tx` 后 `postgres.New(postgres.Config{Conn: tx})`。

#### 4.1.2 AutoMigrate 策略

| 环境 | AutoMigrate | 说明 |
|---|---|---|
| local | 可选（`database.auto_migrate=true`） | 仅本地快速开发；仍需最终跑迁移 |
| test | 关闭 | 测试库由迁移/测试基座建表 |
| staging / prod | **强制关闭** | 启动时校验 `_migrations` 版本；不一致即 fail-fast |

启动校验建议：

```go
// 伪代码
if cfg.Database.AutoMigrate {
    if cfg.Server.Mode != "debug" {
        return fmt.Errorf("auto_migrate 仅允许 debug 模式")
    }
}
if err := migrate.EnsureCurrent(ctx, db, expectedVersion); err != nil {
    return fmt.Errorf("schema 版本不匹配: %w", err)
}
```

#### 4.1.3 模型标签规范

| 场景 | 推荐写法 | 禁止 |
|---|---|---|
| 主键 | `gorm:"primaryKey;autoIncrement"` + `int64` | `type:bigint(20) unsigned` |
| 布尔 | `bool` + `gorm:"not null;default:false"` | `tinyint(1)` |
| 时间 | `time.Time` + `gorm:"type:timestamptz;default:now()"` | `datetime` |
| JSON | `datatypes.JSON`（`gorm.io/datatypes`）或 `json.RawMessage` + `gorm:"type:jsonb"` | `type:json` |
| 金额 | `decimal.Decimal` 或 `string` + `gorm:"type:numeric(18,4)"` | `float64` |
| 软删除 | `gorm.DeletedAt`（标准模型）或自定义 `deleted bool` | 混用两套语义 |
| 表名 | 显式 `TableName()`，保持现有表名 | 依赖隐式复数化导致改名 |
| 索引 | 迁移里建；模型只做查询 | 依赖 AutoMigrate 建索引 |

#### 4.1.4 GORM 使用边界

- **适合**：auth 用户/RBAC/API Key、scheduler 任务/日志、简单 CRUD、分页查询。
- **不适合**：ia 的复杂统计/窗口/日期查询、goodser 的批量库存/组合包、复杂 JSONB 查询；这些后续迁移时优先用 `pgx`/手写 SQL。
- **批量写入**：`CreateInBatches`；UPSERT 用 `clause.OnConflict{DoUpdates: clause.AssignmentColumns(...)}`。
- **日志**：接入 zerolog；慢查询阈值 200ms；生产只记录 warn/error。
- **错误**：开启 `TranslateError`，把 PG 唯一键/外键错误映射为业务错误；不要靠字符串匹配。
- **测试**：GORM Store 必须跑 PG 集成测试；SQLite 不再作为验收环境。
- **迁移一致性**：新增/修改模型时，必须同步迁移文件；CI 增加“模型表/列 vs 迁移 Schema”检查（可用 information_schema 对比）。

#### 4.1.5 需要改造的 GORM Store

| Store | 文件 | 改造 |
|---|---|---|
| User | `internal/services/auth/store.go` | `gorm.io/driver/mysql` → `postgres`；`AutoMigrate` 移除 |
| RBAC | `internal/services/auth/rbac.go` | 同上；默认权限 seed 改 `ON CONFLICT DO NOTHING` |
| API Key | `internal/services/auth/apikey.go` | 同上 |
| Scheduler | `internal/services/scheduler/store.go` | 同上；`gorm.Expr("NOW()")` 保留或改 `time.Now()` |

### 4.2 AI 开发工作流（摘要）

> 完整规范见 [V3 SOP](../projects/go-backend-core/v3/SOP.md)。这里只列主线。

```
需求(DM) → 人工确认验收标准 → 任务契约(上下文/约束/验证命令)
   ↓
AI 规划(只读探索 + 方案) → 人工确认方案（高风险任务）
   ↓
AI 实现(小步提交) → AI 自测(build/test/lint) → 子代理评审(SQL/安全/回归)
   ↓
人工评审 + 合并 → AI 更新任务日志/文档/资产 → DM 状态流转
```

**AI 任务契约必须包含：**

1. 目标与非目标（明确不做什么）。
2. 必读上下文：`AGENTS.md`、模块文档、V3 SOP、相关迁移/仓库文件。
3. 验收标准：可执行的测试/命令/接口断言。
4. 约束：PG 方言、迁移、权限、兼容 `/api/v1`、不得改业务模块。
5. 交付物：代码、测试、迁移、文档、DM 任务日志。
6. 回滚/风险：涉及数据/迁移时必须写。

**AI 硬性规则：**

- 不直接提交 `main`/`feature-V3` 主干；每个任务在 `v3/<task-code>` 分支。
- 不执行破坏性数据库操作（DROP/TRUNCATE/无备份迁移）除非人工书面确认。
- 不读取/输出密钥；不修改 `.env` 真实值。
- 不跳过测试；失败必须报告，不得伪造通过。
- 不擅自扩大范围；发现范围外问题登记为新任务。
- 提交信息带任务编号：`feat(v3): ... (V3-P3-04)`。
- 合并前必须有人工 review；AI 不自动合并。

**AI 上下文包建议目录：**

```
aiPrompt/
└── v3/
    ├── context.md          # 项目地图/关键约束/命令
    ├── pg-rules.md         # PG 方言/迁移/GORM 规则
    ├── task-template.md    # 任务契约模板
    ├── review-checklist.md # 评审清单
    └── migration-rules.md  # 数据迁移/校验规则
```

### 4.3 仍需确认的落地细节

1. **V3 域名**：上线前确定；DNS 与证书由谁管理。
2. **TLS 证书来源**：1Panel/Let's Encrypt/现有证书；是否需要 HSTS。
3. **公网用户开通方式**：管理员创建用户，还是“申请 → 管理员审批”；是否需要邮箱/手机号验证。
4. **V2 业务模块在并行期的访问入口**：继续用老域名，还是也经 1Panel？两套登录如何向用户说明。
5. **12.15 上线批次**：项目管理高级前端（风险矩阵/发布日历/甘特/报表）是否必须 12.15 前完成，还是 12/31 前补齐。
6. **1Panel v2**：新机器版本、API/MCP 文档、Token 权限；数据源/MCP 工具清单；本机端口与鉴权方式。
7. **管理入口限制**：`auth/admin`、`ops`、`migrate` 是否只允许内网/IP 白名单。
8. **业务模块迁移顺序**：是否按 goodser → ia → login3rd → shopchecker。

---

## 5. SQL 方言迁移手册（V3.0 范围）

### 5.1 DDL 类型映射

| MySQL | PostgreSQL | 说明 |
|---|---|---|
| `BIGINT AUTO_INCREMENT PRIMARY KEY` | `BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY` | 保留显式插入能力；迁移后 `setval` |
| `INT AUTO_INCREMENT` | `INTEGER GENERATED BY DEFAULT AS IDENTITY` | 同上 |
| `TINYINT(1)` / `BOOLEAN` | `BOOLEAN` | 禁止 `= 1` |
| `TINYINT` / `SMALLINT` | `SMALLINT` | 状态码等 |
| `INT` / `BIGINT` | `INTEGER` / `BIGINT` | 计数用 `BIGINT` |
| `DECIMAL(p,s)` | `NUMERIC(p,s)` | 金额禁止 `float` |
| `DOUBLE` / `FLOAT` | `DOUBLE PRECISION` / `REAL` | 行情/比率 |
| `VARCHAR(n)` | `VARCHAR(n)` | 保持长度约束 |
| `TEXT` / `LONGTEXT` | `TEXT` | 需要默认值时用 `DEFAULT ''` |
| `DATETIME` / `TIMESTAMP` | `TIMESTAMPTZ` | 统一 UTC；默认 `now()` |
| `DATE` | `DATE` | 到期日、交易日期 |
| `JSON` | `JSONB` | 支持索引与操作符 |
| `ENUM(...)` | `VARCHAR(n) + CHECK` | 便于演进 |
| `BLOB` | `BYTEA` | 文件/哈希 |
| `UNSIGNED` | `CHECK (col >= 0)` | PG 无 unsigned |
| `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4` | 删除 | PG 库级编码 |
| `ON UPDATE CURRENT_TIMESTAMP` | 触发器 `set_updated_at()` 或应用层赋值 | 统一触发器 + 应用层兜底 |
| `KEY idx (...)` / `UNIQUE KEY` | 独立 `CREATE INDEX` / `CREATE UNIQUE INDEX` | 便于命名与部分索引 |
| `FULLTEXT` | `tsvector` + GIN 或 `pg_trgm` | V3.0 暂不迁移 |

### 5.2 DML/查询映射

| 场景 | MySQL 写法 | PostgreSQL 写法 |
|---|---|---|
| 占位符 | `WHERE id = ?` | `WHERE id = $1` |
| 批量插入 | `VALUES (?,?),(?,?)` | `VALUES ($1,$2),($3,$4)` |
| UPSERT | `ON DUPLICATE KEY UPDATE c=VALUES(c)` | `ON CONFLICT (k) DO UPDATE SET c=EXCLUDED.c` |
| 忽略重复 | `INSERT IGNORE INTO ...` | `INSERT INTO ... ON CONFLICT DO NOTHING` |
| 替换整行 | `REPLACE INTO t (...) VALUES (...)` | `INSERT ... ON CONFLICT (pk) DO UPDATE SET ...` |
| JSON 包含 | `JSON_CONTAINS(tags, ?)` | `tags @> $1::jsonb` |
| JSON 取标量 | `JSON_UNQUOTE(JSON_EXTRACT(col, '$.k'))` | `col ->> 'k'` |
| 当前日期 | `CURDATE()` | `CURRENT_DATE` |
| 当前时间 | `NOW()` | `now()` |
| 日期加 N 天 | `DATE_ADD(d, INTERVAL ? DAY)` | `d + make_interval(days => $1)` |
| 日期差 | `DATEDIFF(a, b)` | `(a::date - b::date)` |
| 日期格式化 | `DATE_FORMAT(d, '%Y-%m-%d')` | `to_char(d, 'YYYY-MM-DD')` |
| 取年份 | `YEAR(d)` | `EXTRACT(YEAR FROM d)` |
| 字符串拼接 | `CONCAT(a,b)` | `a || b` 或 `concat(a,b)` |
| 左补零 | `LPAD(id, 10, '0')` | `lpad(id::text, 10, '0')` |
| 最新一行 | `MAX(CONCAT(date, LPAD(id,10,'0')))` 自连接 | `DISTINCT ON (k) ... ORDER BY k, date DESC, id DESC` |
| 大小写不敏感 | `LIKE '%x%'` | `ILIKE '%x%'` |
| NULL 排序 | `ORDER BY c`（NULL 靠前） | 显式 `NULLS FIRST/LAST` |
| 当前库 | `SELECT DATABASE()` | `SELECT current_database()` / `current_schema()` |
| 表列表 | `SHOW TABLES` | `SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='public'` |
| 列信息 | `SHOW COLUMNS FROM t` | `information_schema.columns`（列名小写） |
| 标识符 | `` `col` `` | `"col"` 或全小写不加引号 |
| 分页 | `LIMIT ? OFFSET ?` | `LIMIT $1 OFFSET $2` |
| 严格分组 | `GROUP BY` 宽松 | 所有非聚合列必须出现在 `GROUP BY` |
| 数组/IN | `IN (?)` 动态拼接 | `= ANY($1)` 或动态 `$n` |

### 5.3 V3.0 逐文件改造清单

| 优先级 | 文件/范围 | 主要问题 | 预估 |
|---|---|---|---:|
| P0 | `internal/config/config.go`、`config.yaml`、`.env.example` | MySQL → PG 配置、`sslmode/search_path/timezone` | 1d |
| P0 | `internal/database/manager.go`、`internal/database/postgres` | PG 主库、`Main()`/`PG()`、健康检查、热加载 | 2d |
| P0 | `internal/modules/{auth,integration,migrate,ops,health,servicenav,webhook}` | `database.DB.MySQL["main"]` 全部替换 | 2d |
| P0 | `internal/services/scheduler/module.go` + `store.go` | GORM MySQL → PG、AutoMigrate → 迁移 | 1d |
| P1 | `zzz/developmentManager/*_repository.go` | `SELECT DATABASE()`、`information_schema`、UPSERT、分页 | 2d |
| P1 | `internal/modules/webhook/repository.go`、`servicenav/repository.go` | `JSON_CONTAINS`、分页 | 1d |
| P1 | `internal/modules/integration/*`、`internal/services/outbox/mysql.go` | 外盒/动作日志 UPSERT、JSONB、时间 | 1d |
| P2 | `internal/services/auth/{store,rbac,apikey}.go` | GORM MySQL → PG；AutoMigrate → 迁移 | 2.5d |
| P2 | `internal/services/fileref/mysql.go`、`internal/modules/ops/service.go` | 自建 DDL、`DATABASE()`、反引号、扫描 SQL | 1.5d |
| P2 | `internal/services/audit/mysql.go`、`feature/store.go` | DDL、`REPLACE INTO`、PG Store | 1d |
| P2 | `internal/services/migrate/{adapter,schema,dumper,backup}.go` | MySQLAdapter 默认、dump 方言、`SHOW` | 1.5d |
| P3 | 基座/公共/PM 全仓 `?` 占位符 | codemod + 人工复核 | 1.5d |
| 后续 | `zzz/goodser`、`zzz/ia`、`zzz/login3rd`、`zzz/shopchecker` | 业务模块方言改造 | V3.1–V3.4 |

> V3.0 **不修改**业务模块的 SQL；它们留在 V2。后续模块迁移时按同一手册执行，并补充该模块的表/查询清单。

### 5.4 占位符改造规则

1. 只改 **SQL 字符串常量** 中的 `?`，不改 Go 代码里的 `?`。
2. 每条 SQL 语句独立编号，从 `$1` 开始；动态 `IN` 子句必须序号连续。
3. 禁止在 SQL 中使用 jsonb 的 `?` 存在运算符；需要判断键是否存在时用 `jsonb_exists(col, $1)`。
4. `fmt.Sprintf` 拼接 SQL 时，`$n` 不能跨拼接段重复；建议先构造 `args`，再统一生成占位符。
5. codemod 产出 diff 后必须逐文件 review；每个仓库至少配一条 PG 集成测试。

### 5.5 PostgreSQL 特有陷阱

| 陷阱 | 表现 | 处理 |
|---|---|---|
| 标识符折叠 | 未加引号的 `CamelCase` 变成小写 | 表/列全小写；动态标识符统一 `Quote()` |
| `?` 运算符 | jsonb `?` 与占位符冲突 | 禁用 `?` 运算符 |
| 时区 | `CURRENT_DATE` 依赖会话时区 | DSN 设 `TimeZone=UTC` |
| 布尔扫描 | MySQL `tinyint` 返回 int，PG 返回 bool | 模型字段用 `bool` |
| jsonb 扫描 | `database/sql` 返回 `[]byte` | `json.RawMessage`/`[]byte` + `json.Unmarshal` |
| `COUNT(*)` | 返回 `int64` | 扫描到 `int64` |
| `LIMIT $1` | 参数类型需为整数 | 传 `int` |
| 严格 GROUP BY | 非聚合列必须分组 | 改写查询或窗口函数 |
| NULL 排序 | 与 MySQL 相反 | 显式 `NULLS FIRST/LAST` |
| 大小写搜索 | `LIKE` 区分大小写 | `ILIKE`/`citext` |
| 序列 | 导入数据后序列不前进 | 每表 `setval` |
| 连接池 | PG 连接数更敏感 | 下调 `max_open`；预留 pgbouncer |
| 锁 | `ALTER TABLE` 默认强锁 | 大表用 `CONCURRENTLY`/分步迁移 |

---

## 6. Schema 与迁移体系（V3.0）

### 6.1 目录与版本

```
migrations/
├── mysql/                     # V2 归档，只读；老机器继续使用
│   └── 001–017_*.sql
└── postgres/
    ├── 0001_v3_baseline.sql   # V3.0 基座/公共/PM 核心表（约 35 张）
    ├── 0002_v3_pm_full.sql    # 完整项目管理扩展（约 13 张）
    ├── 0003_v3_ops_config.sql # 主机台账 + 配置中心（3 张）
    ├── 0004_*.sql             # 后续变更
    └── README.md              # V3.0 bootstrap 与模块迁移说明
```

> 实际文件按落地顺序编号：`001_init.sql`（`_migrations`）、`002_system_config.sql`（配置中心），后续基线/台账为 `003+`；上面为规划集合。

- **0001 baseline**：V3.0 必需的基座/公共/PM 核心表，约 35 张：
  - `_migrations` 1
  - auth：users/roles/permissions/role_permissions/user_roles/api_keys 6
  - scheduler：scheduler_tasks/scheduler_task_logs 2
  - ops/fileref：file_references/scan_targets 2
  - audit：audit_log 1
  - feature：feature_flags 1
  - integration：action_logs/connectors/outbox 3
  - webhook：configs/delivery_logs 2
  - servicenav：item/type 2
  - developmentManager 核心：15
- **0002 PM full**：约 13 张扩展表（见 §8.3）。
- **0003 ops/config**：`zzz_ops_host`、`system_config`、`system_config_history`（3 张）。
- **1Panel 相关表不在 V3.0**：多实例/密钥入库后续单独立项。
- 业务模块表（goodser/ia/login3rd/shopchecker）不在 V3.0 基线；V3.1+ 按模块新增迁移。
- V2 的 `_migrations` 记录不迁移；V3 以 baseline 为新起点（版本 1），bootstrap 工具负责把 V2 数据装入 V3。

### 6.2 迁移执行规则

| 规则 | 说明 |
|---|---|
| 单一事务 | 每个迁移文件默认一个事务；`CREATE INDEX CONCURRENTLY` 单独标注 |
| 幂等 | 使用 `IF NOT EXISTS` / `IF EXISTS`；seed 用 `ON CONFLICT DO NOTHING` |
| checksum | 已发布文件不可修改；修改必须新增迁移 |
| 回滚 | 每个文件配 down 段；破坏性变更先备份 |
| 生产禁用 AutoMigrate | `database.auto_migrate=false`；启动校验 `_migrations` 版本 |
| 迁移前检查 | 连接、版本、dirty、锁、磁盘、备份；失败即停 |
| 迁移后检查 | `ANALYZE`、序列、外键、行数、关键查询 |

### 6.3 表与索引约定

- 表名全小写、`snake_case`；基座表无前缀，模块表 `zzz_<module>_`。
- 主键统一 `id BIGINT GENERATED BY DEFAULT AS IDENTITY`。
- 审计字段统一：`created_at timestamptz not null default now()`、`updated_at timestamptz not null default now()`、`created_by/updated_by varchar(64)`、`deleted boolean not null default false`、`deleted_at timestamptz`。
- 外键命名 `fk_<table>_<ref>`；唯一索引 `uk_<table>_<cols>`；普通索引 `idx_<table>_<cols>`。
- 软删除表使用部分索引：`CREATE INDEX ... WHERE deleted = false`。
- JSONB 按查询模式建 GIN：`USING gin (tags jsonb_path_ops)`。
- 金额 `numeric(18,4)`；比率 `numeric(18,8)`；禁止 `float` 参与金额计算。

### 6.4 迁移模块改造

| 现状 | V3 |
|---|---|
| `migrate` 模块优先取 MySQL | 优先取 PG；MySQLAdapter 归档到 `internal/services/migrate/legacy` |
| `SHOW TABLES`/`SHOW COLUMNS` | PGAdapter 用 `pg_catalog`/`information_schema` |
| dump 输出 MySQL DDL | 输出 PG DDL；`formatSQLValue` 处理 `timestamptz/jsonb/boolean` |
| `_migrations` 建表带 `ENGINE` | 去掉 `ENGINE`；字段用 `timestamptz` |
| `transfer` 支持 MySQL→PG | 保留为 bootstrap 工具底层能力；不在运行时暴露 |
| backup 用 JSON dump | 保留 JSON dump；新增 `pg_dump`/`pg_restore` 脚本 |

### 6.5 数据字典

- V3.0 的约 51 张表必须登记到 developmentManager `db_table/db_column`。
- 字典扫描改为 PG：`current_schema()`、`information_schema.columns` 小写列名、`data_type` 映射。
- 每张表补 `purpose`，每个字段补 `meaning`；V3 新增表必须同迁移提交。
- 业务模块表在 V3.1+ 迁移时登记，不提前塞入 V3.0。

---

## 7. V3.0 数据 bootstrap 与业务模块后续迁移

### 7.1 V3.0 bootstrap 范围

只迁移 V3.0 需要的数据，**不迁移业务模块数据**。

| 分组 | 表 |
|---|---|
| 认证/权限 | `users`、`roles`、`permissions`、`role_permissions`、`user_roles`、`api_keys` |
| 调度 | `scheduler_tasks`、`scheduler_task_logs`（仅 active 任务 + 最近日志，可选） |
| 运维/文件 | `file_references`、`scan_targets` |
| 审计/开关 | `audit_log`（可选）、`feature_flags` |
| 集成 | `zzz_integration_connectors`、`zzz_integration_outbox`、`zzz_integration_action_logs`（近期） |
| Webhook/导航 | `zzz_webhook_configs`、`zzz_webhook_delivery_logs`（近期）、`zzz_servicenav_item`、`zzz_servicenav_type` |
| 项目管理 | 15 张 `zzz_development_manager_*` |
| 设置 | `data/settings.json`（文件复制） |

> 不迁移：`zzz_goodser_*`、`zzz_ia_*`、`zzz_login3rd_*`、`zzz_shopchecker_*`。

### 7.2 工具设计

新增 `cmd/v3migrate`（仅 V3.0 bootstrap 使用）：

```
v3migrate plan      # 输出允许迁移的表、行数、依赖顺序、预估大小
v3migrate dump      # 从 V2 MySQL 导出 NDJSON/CSV（按表分片）
v3migrate load      # 按依赖顺序批量写入 V3 PG（COPY）
v3migrate verify    # 行数/主键/FK/JSON/时间/金额/抽样/序列
v3migrate report    # 生成 Markdown/JSON 报告
```

- 读取 V2 MySQL 用 `database/sql` + `go-sql-driver/mysql`（仅工具依赖，可独立构建）。
- 写入 V3 PG 用 `pgx` `CopyFrom`，每批 1,000–10,000 行。
- 表顺序按外键拓扑排序。
- 类型转换显式完成：`tinyint(1)→bool`、`datetime→timestamptz`、`json→jsonb`、`decimal→numeric`。
- 迁移后 `setval` + `ANALYZE`。
- 工具必须有 allowlist；默认拒绝业务模块表。

### 7.3 校验

| 校验项 | 方法 | 通过标准 |
|---|---|---|
| 行数 | 每表 `COUNT(*)` 对比 | 完全一致 |
| 主键 | `MIN/MAX/COUNT(DISTINCT id)` | 一致，无重复 |
| 外键 | 孤儿行查询 | 0 孤儿 |
| JSON | `jsonb_typeof` / 抽样反序列化 | 全部合法 |
| 时间 | 范围、时区、NULL | 与 V2 语义一致（UTC） |
| 金额 | `SUM`/`MIN`/`MAX` 对比 | 误差 0（numeric） |
| 业务抽样 | auth/RBAC/PM/各平台配置 5–10 条关键查询 | 结果一致 |
| 序列 | `nextval` 与 `max(id)` | 大于最大值 |

### 7.4 双环境并行 Runbook（V3.0）

| 时间 | 动作 | 回滚点 |
|---|---|---|
| T-7d | V3 新机器就绪；PG/Redis/S3/Chrome 可用；V3 部署演练 | 无 |
| T-3d | V2 侧冻结 developmentManager 写入；通知 PM 用户 | 可恢复 V2 PM |
| T-2d | 第一次 bootstrap 演练；校验报告 | 清空 V3 PG 重来 |
| T-1d | 第二次演练；确认窗口；准备 V2 快照 | 清空 V3 PG 重来 |
| T-0 | 停 V2 PM 写入；执行最终 dump/load/verify；启动 V3 | V3 可清空重来 |
| T+1h | V3 健康检查 + PM 核心 API 冒烟；通知用户切到 V3 PM | 停 V3，恢复 V2 PM |
| T+1d | 对比 PM 数据；确认无差异 | 保留 V2 快照 |
| T+7d | V3 稳定；V2 PM 永久只读/归档 | 结束 |

> V3.0 只切换“项目管理”使用入口，不切换业务模块。V2 业务继续正常运行。

### 7.5 业务模块后续迁移（V3.1+ 通用 SOP）

每个业务模块迁移必须走以下步骤（详见 V3 SOP）：

1. **立项**：在 DM 建里程碑（V3.x）与需求；明确新增/修改需求。
2. **代码移植**：从 V2 分支复制模块代码到 V3；按 PG 手册改造 SQL/GORM。
3. **Schema**：为模块新增迁移；数据字典登记。
4. **测试**：PG 集成测试 + 业务回归 + 性能基线。
5. **数据迁移**：冻结 V2 模块写入 → 快照 → 迁移 → 校验 → 增量（如需要）。
6. **路由切换**：前端/网关把该模块 API 切到 V3；V2 模块停用。
7. **观察**：7 天稳定；保留 V2 数据快照。
8. **下线**：所有模块迁移完成且 V3 稳定后，才评估老机器下线。

### 7.6 1Panel 接入（V3.0 暂停，后续待定）

> **状态（2026-11-18）：1Panel API/数据源/MCP/多实例/密钥入库暂停，不进入 V3.0 WBS。**
> 以下内容保留为后续评估资料；V3.0 只要求公网反代/证书可用（1Panel 或 OpenResty 均可）。

**原评估结论：1Panel v2 在新机器，V3 直接使用；老机器保留自己的 1Panel 服务老域名/V2。**

1. **边缘（12.15 必需）**：新机器 1Panel v2 管理新二级域名站点、证书、HTTPS、反向代理，回源本机 V3:80。
2. **数据来源（12.15 最小可用）**：Docker/数据库/系统状态只读接入。
3. **MCP 支持（12.15 最小可用）**：1Panel MCP/工具注册与只读调用；受限操作后补。

#### 7.6.0 边缘职责（必需）

| 项 | 1Panel v2 负责 | V3 负责 |
|---|---|---|
| 域名/站点 | 新二级域名站点、绑定、DNS 确认 | 提供 `server.public_base_url` |
| 证书 | 申请/续期/部署 HTTPS 证书 | 不管理证书 |
| 反代 | 443 → 本机 V3:80 | 容器内监听 80 |
| 转发头 | `Host/X-Real-IP/X-Forwarded-For/X-Forwarded-Proto` | 配置 `TrustedProxies`、读取真实 IP |
| 限流/安全头 | 反代层限流、安全头（可配） | 应用层限流、CORS、RBAC |
| 日志 | 反代 access/error log | 应用日志、审计 |
| 1Panel 面板 | 仅内网/VPN/IP 白名单 | 不暴露、不代理 |

> **网络前提**：新二级域名指向新机器；1Panel 与 V3 同机；V3 通过本机/内网访问 1Panel。老机器与 V3 同局域网，仅用于 bootstrap/迁移/运维，不做生产反代。

#### 7.6.1 为什么 API 适配不是核心

| 原因 | 说明 |
|---|---|
| 定位不同 | 1Panel 是运维面板，不是多租户业务 API；权限模型与 V3 RBAC 不一致 |
| API 稳定性 | 版本升级可能改接口；需要适配层隔离，不能散落调用 |
| 安全边界 | 1Panel 权限很高；V3 公网暴露后，必须最小权限、网络隔离、审计 |
| 职责冲突 | V3 已有 migrate/数据字典/监控；数据库 Schema 必须由 V3 迁移唯一管理 |
| 许可证/分发 | 需确认 1Panel 许可证与二次分发边界；建议只通过 HTTP API 集成，不复制其代码 |
| 上线风险 | 12.15 窗口紧，仅保留数据/MCP 最小只读；完整 API/受限操作后补 |

#### 7.6.2 可接入能力与边界

| 能力 | 建议 | 边界 |
|---|---|---|
| Docker/Compose | 只读列表/状态/日志；受限重启/重部署 | 仅管理员；二次确认 + 审计；不管理 V3 自身容器 |
| 数据库管理 | 只读状态/连接数/慢查询；触发备份 | **不允许 1Panel 改 V3 Schema**；迁移仍由 V3 migrate 管理 |
| MCP 管理 | **12.15 最小可用**：1Panel MCP/工具注册 + 只读调用 | 工具白名单；不与 V3 integration 的 MCP 端点重复；明确谁做网关 |
| 系统监控 | 宿主机 CPU/内存/磁盘/网络只读 | 应用指标仍用 Prometheus/Grafana；不替代现有监控 |
| 应用商店/证书 | **边缘必需**：证书申请/续期/部署；应用商店可选 | V3 不手工改证书/反代；1Panel 面板不公网 |

#### 7.6.2.1 API 认证与只读范围（已核对官方文档）

**版本/地址：**

- 1Panel 社区版 **2.5.1**；API 前缀 `/api/v2`。
- 开发机：`10.144.144.10:58331`（仅开发/联调；生产用新机器本机/内网地址）。
- Swagger：`{host}:{port}/1panel/swagger/index.html`（需登录；开发机 Swagger 未登录返回 401）。

**鉴权（官方文档）：**

| 项 | 说明 |
|---|---|
| 安全入口 | `EternallyAscend`；面板路径 `https://gtp.hailong.site/EternallyAscend`；API 仍走 `/api/v2`（带/不带入口前缀均测过） |
| Header | `1Panel-Token`、`1Panel-Timestamp`（秒级 Unix 时间戳） |
| 推荐签名 | `HMAC-SHA256(API-Key, "1panel:" + timestamp)` |
| 兼容签名 | `MD5("1panel" + API-Key + timestamp)`（旧版，后续可能移除） |
| API Key | 面板“用户信息/API 接口”创建并启用；配置 IP 白名单、有效期 |
| 时间 | 客户端与 1Panel 必须 NTP 同步，否则签名失败 |
| 示例 | `GET /api/v2/dashboard/base/os`（只读） |

**只读能力（参考官方 1Panel-skills 模块）：**

| 模块 | 只读内容 | V3 用途 |
|---|---|---|
| `monitoring` | 节点状态、仪表盘指标、Top 进程、监控历史、GPU 历史 | 系统监控面板 |
| `websites` | 网站列表/详情、Nginx 配置、域名、HTTPS、证书、日志 | 边缘状态/证书检查 |
| `apps` | 应用市场、已安装应用、服务、端口/连接 | 运行环境概览 |
| `containers` | 容器列表/状态/Inspect/资源统计/日志 | Docker 只读 |
| `logs` | 操作日志、登录日志、系统日志文件 | 审计辅助 |
| `cronjobs` | 定时任务列表/详情/执行记录 | 计划任务概览 |
| `task-center` | 任务中心记录/执行中数量 | 异步任务状态 |
| `nodes` | 节点列表/状态（部分可能需 Pro/XPack） | 多节点概览 |

> **只读边界**：V3.0 只调用查询类接口（GET，或 1Panel-skills 的只读 search POST）；不调用创建/更新/删除/重启/执行脚本类接口。写操作 12.15 后单独评审。

#### 7.6.3 适配器设计

```
internal/services/onepanel/
├── client.go       # HTTP client：base_url/api_key/timeout/TLS/retry/breaker
├── sign.go         # 1Panel-Token/Timestamp：HMAC-SHA256 + MD5 兼容
├── models.go       # Dashboard/Container/Website/Cert/DB/Monitor DTO
├── readonly.go     # 只读查询：monitoring/containers/websites/apps/logs/cronjobs
├── mcp.go          # 1Panel MCP/工具注册（如果版本提供；否则 adapter 降级）
└── client_test.go  # mock server + 签名/只读契约测试

internal/modules/ops/
└── onepanel_handler.go  # /api/v1/ops/onepanel/*（管理员）
```

配置（示例）：

```yaml
onepanel:
  enabled: true
  base_url: "http://127.0.0.1:<port>"   # 生产：新机器本机；开发：http://10.144.144.10:58331
  api_key: ""               # .env/secret，禁止入库/入日志
  auth_mode: "hmac-sha256"  # hmac-sha256（推荐）| md5（兼容旧版）
  tls_verify: false         # 本机 HTTP；跨机时启用 TLS
  timeout: 10s
  ip_whitelist: "127.0.0.1,10.144.144.0/24"  # 仅文档；实际在 1Panel 配置
  features:
    docker: true
    database: true
    mcp: true
    monitor: true
```

权限：

| 资源 | 动作 | 说明 |
|---|---|---|
| `ops.onepanel` | read | 查看 Docker/DB/MCP/监控 |
| `ops.onepanel.docker` | operate | 重启/重部署（二次确认） |
| `ops.onepanel.database` | backup | 触发备份（不改 Schema） |
| `ops.onepanel.mcp` | manage | MCP 服务登记/启停 |

#### 7.6.4 实施阶段

| 阶段 | 内容 | 时间 |
|---|---|---|
| 边缘配置 | 新机器 1Panel v2 建新二级域名站点、证书、反代到本机 V3:80 | W3–W4（P8-06，必需） |
| P0 调研 | 确认 1Panel v2 版本/API/Token 权限/MCP 语义 | W1（0.5d） |
| 数据/MCP 最小可用 | 系统/Docker/DB 状态只读 + MCP 工具注册/只读调用 | W3–W4（P8-07，1.0d） |
| 受限操作 | 重启容器、触发备份、MCP 启停 | 12.15 后 2–3d |
| 评估 | 是否保留/扩展；安全复盘 | 按使用情况 |

#### 7.6.5 安全要求

- 1Panel 面板只开内网/专线，不对公网；V3 通过本机/内网地址调用。
- 1Panel 边缘仅暴露 443；1Panel 管理端口不暴露公网。
- V3 公网应用访问本机 1Panel 时必须走**严格 allowlist 适配器**，禁止原始 URL 代理，防 SSRF/越权。
- 使用专用最小权限 Token；禁止 root/面板管理员账号。
- Token 存 Secret/环境变量；日志脱敏。
- 所有写操作二次确认 + 审计 + 限流 + 超时/熔断。
- 1Panel 不可用时，V3 核心功能不受影响（降级）。
- 不把 1Panel 作为身份源、数据库 Schema 源或唯一监控源。

#### 7.6.6 多实例与 API Key 入库

**表：`zzz_ops_onepanel_instance`（V3.0 新增，迁移 `0003_v3_onepanel.sql`）**

| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| name | varchar(64) UK | 实例名（如 `gtp-cloud`、`new-prod`） |
| base_url | varchar(255) | 面板地址（如 `https://gtp.hailong.site`） |
| api_key_ciphertext | text | AES-GCM 密文；禁止明文 |
| api_key_hint | varchar(16) | 末 4 位提示，仅展示 |
| auth_mode | varchar(16) | `hmac-sha256` / `md5` |
| tls_verify | boolean | 是否校验 TLS（生产 true） |
| environment | varchar(16) | dev/staging/prod |
| tags | jsonb | 标签/分组 |
| enabled | boolean | 是否启用 |
| last_status | varchar(16) | ok/error/unknown |
| last_checked_at | timestamptz | 最近探测 |
| last_error | text | 最近错误（脱敏） |
| created_by / updated_by | varchar(64) | 审计 |
| deleted / created_at / updated_at | — | 审计 |

**加密：**

- 使用 `internal/services/crypto`（AES-GCM）加密 API Key；主密钥来自 `APP_CRYPTO_MASTER_KEY_HEX`/Secret。
- 写入后只返回 `api_key_hint`；列表/详情永不返回明文/密文。
- 日志、审计、错误信息全部脱敏。
- 更新 Key 时重新加密；支持后续密钥轮换。

**API（管理员）：**

```
GET|POST        /api/v1/ops/onepanel/instances
GET|PUT|DELETE  /api/v1/ops/onepanel/instances/:id
POST            /api/v1/ops/onepanel/instances/:id/test
GET             /api/v1/ops/onepanel/instances/:id/dashboard
GET             /api/v1/ops/onepanel/instances/:id/containers
GET             /api/v1/ops/onepanel/instances/:id/websites
GET             /api/v1/ops/onepanel/instances/:id/apps
GET             /api/v1/ops/onepanel/instances/:id/logs
GET             /api/v1/ops/onepanel/instances/:id/cronjobs
GET             /api/v1/ops/onepanel/instances/:id/tasks
GET             /api/v1/ops/onepanel/instances/:id/nodes
```

- `instance_id` 显式传入；不设默认实例，避免误操作。
- 权限：`ops.onepanel.read`、`ops.onepanel.manage`、`ops.onepanel.test`。
- 所有调用审计；测试连接不返回 Key。

**1Panel-skills 接入策略：**

| 方案 | 说明 | 建议 |
|---|---|---|
| A. Go 适配器复刻 1Panel-skills 只读模块 | 按模块/动作映射到 `/api/v2`，不引入 Node | **首选**；易做 RBAC/审计/多实例 |
| B. 独立运行 1Panel-skills CLI/sidecar | Node 18+，V3 通过 HTTP/CLI 调用 | 需要原样使用 skill 时；不放进 V3 主进程 |
| C. OpenClaw/MCP 外部技能 | 把 1Panel-skills 注册为外部 MCP/skill | 已有 OpenClaw 时可用；多实例 Key 仍需 V3 管理 |

- 1Panel-skills README 使用 MD5 签名；官方 API 文档推荐 HMAC-SHA256。V3 两种都支持，默认 HMAC。
- 许可证：GitHub README 标 MIT，仓库元数据标 GPL-3.0；**在确认前不复制其代码**，只按公开 API/模块契约实现，或独立进程调用。
- 只读动作先接；写动作（create/update/delete/restart）不进入 V3.0。

**当前联调状态（2026-11-18）：**

| 项 | 结果 |
|---|---|
| 地址 | `https://gtp.hailong.site`；安全入口 `EternallyAscend` |
| 路径 | `/api/v2/dashboard/base/os` 与 `/EternallyAscend/api/v2/dashboard/base/os` 均试过 |
| 签名 | HMAC-SHA256 与 MD5 均试过 |
| 结果 | `401 {"message":"API 接口禁止访问"}` |
| 无鉴权 | `401 用户未登录: 当前会话已过期` |
| 入口验证 | `/EternallyAscend` 可访问并 301 到自身，说明入口存在 |
| 网络线索 | 响应头 `Server: cloudflare`，API 经 Cloudflare 回源；1Panel 看到的可能是 Cloudflare 回源 IP |
| 已排除 | 安全入口路径、HMAC/MD5、秒/毫秒时间戳、浏览器 UA、原始 Key 直接当 Token |
| 判断 | API 接口未启用 / Key 未启用或过期 / IP 白名单未放行（尤其 Cloudflare 回源 IP） |
| 下一步 | 1Panel 用户菜单 → API 接口 → 启用 + 确认 Key 启用/有效期 + 白名单临时 `0.0.0.0/0`（或加 Cloudflare 段/直连源站）后复测 `/api/v2/dashboard/base/os` |

---

### 7.7 多主机管理（V3.0 最小）

**目标**：登记云服务器、主机、备份机，查看 SSH 连通与基础状态；完整运维/批量操作后续。

**表：`zzz_ops_host`（V3.0 新增）**

| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| name | varchar(64) UK | 主机名 |
| role | varchar(16) | `cloud` 云服务器 / `host` 主机 / `backup` 备份机 |
| environment | varchar(16) | prod/staging/dev |
| address | varchar(255) | IP/域名 |
| ssh_port | int | SSH 端口，默认 22 |
| username | varchar(64) | SSH 用户 |
| auth_type | varchar(16) | password/key |
| credential_ciphertext | text | 密码/私钥密文（AES-GCM） |
| credential_hint | varchar(32) | 提示（末 4 位/指纹） |
| os / tags / notes | — | 系统、标签、备注 |
| status | varchar(16) | unknown/online/offline/error |
| last_checked_at / last_error | — | 最近探测 |
| deleted / created_by / updated_by / timestamps | — | 审计 |

**API（管理员）：**

```
GET|POST        /api/v1/ops/hosts
GET|PUT|DELETE  /api/v1/ops/hosts/:id
POST            /api/v1/ops/hosts/:id/test        # SSH 连通
GET             /api/v1/ops/hosts/:id/status      # 在线/负载/磁盘（只读）
```

**V3.0 边界：**

- 只做台账 + SSH 连通/状态；不做任意命令执行、文件分发、批量重启。
- 凭据加密入库；响应只返回提示；审计所有连接。
- SSH 主机指纹校验；命令白名单；超时/重试/熔断。
- 权限：`ops.host.read`、`ops.host.manage`、`ops.host.connect`。
- 前端：V3.0 先列表/详情；批量运维 V3.1。

### 7.8 数据库迁移/备份（V3.1，低优先级）

> **前置**：等对应容器迁移到新机器后再做；不进入 12.15。

**能力：**

- 主机间迁移：PostgreSQL `pg_dump/pg_restore`、`pg_basebackup`；源/目标主机 + 数据库实例选择。
- 冷备份：停服务/只读 → 复制数据目录/快照 → 校验 → 启动；支持本地/远端存储。
- 备份策略：全量/增量（按需）、保留天数、加密、校验和、恢复演练。
- 任务编排：scheduler + ops host + storage + crypto + audit。

**表（V3.1）：** `zzz_ops_backup_policy`、`zzz_ops_backup_job`、`zzz_ops_backup_artifact`、`zzz_ops_migration_job`。

**API（V3.1）：**

```
GET|POST        /api/v1/ops/backups/policies
POST            /api/v1/ops/backups/policies/:id/run
GET             /api/v1/ops/backups/jobs
POST            /api/v1/ops/backups/jobs/:id/restore
GET|POST        /api/v1/ops/migrations
GET             /api/v1/ops/migrations/:id
```

**原则：** 先只读/演练；恢复/覆盖操作二次确认；生产迁移必须人工审批；保留回滚点。

### 7.9 外部服务（V3.2，低优先级）

| 服务 | 现状 | V3 计划 | 优先级 |
|---|---|---|---|
| Ollama / Embedding | `services/ai` 已有 Ollama/OpenAI | 保留；补 Embedding/模型管理 | 低 |
| Browser | chromedp/browserless 已有 | 保留；连接池/健康检查 | 低 |
| n8n | settings 已有地址 | 保留；动作/回调对接 | 低 |
| OpenClaw | settings/connector 已有 | 保留；连接器完善 | 低 |
| LangBot | settings 已有地址 | 保留；消息渠道对接 | 低 |
| MCP | integration 已有 MCP 端点 | 保留；工具注册/鉴权 | 低 |
| TTS | 暂无 | 新增适配（按需） | 低 |

> 这些能力不进入 V3.0 关键路径；已有实现保留，新增按 V3.2 排期。

### 7.10 配置中心（V3.0 架构约束）

**目标：能配置的项目/能力尽量做成配置项，减少写死和二次开发。**

#### 7.10.1 配置分类与优先级

| 类别 | 示例 | 存储/优先级 | 热更新 |
|---|---|---|---|
| 启动/安全 | DB DSN、JWT secret、master key、监听端口 | Secret/ENV > config.yaml > 默认值；**不入 DB 明文** | 否 |
| 运行时业务 | 限流、保留天数、分页上限、开关、超时、外部 URL | DB > config.yaml > 默认值 | 是 |
| 模块能力 | AI/Browser/n8n/OpenClaw/LangBot/MCP/TTS、主机/备份 | DB > config.yaml > 默认值 | 是（能力启停/参数） |
| 只读展示 | 版本、构建信息、只读环境信息 | 代码/构建注入 | 否 |

- 环境变量可 `env_locked` 锁定某项，防止被 DB 覆盖。
- 敏感项只允许 Secret/ENV 或加密存储；接口只回 `***`/末 4 位。
- 配置变更先校验，再持久化，再发布 `config.changed`；应用失败自动回滚。

#### 7.10.2 配置 Schema

每个模块声明 `ConfigSpec`：

```go
type Spec struct {
    Key         string        // 唯一键，如 "scheduler.worker_concurrency"
    Module      string        // 所属模块
    Type        string        // string/int/bool/duration/json/secret
    Default     any
    Enum        []string
    Min, Max    *int
    Sensitive   bool
    Reloadable  bool
    EnvLocked   bool
    Description string
    Group       string        // 前端分组
    Validate    func(any) error
}
```

- 模块在 `init()`/构造时注册；启动时汇总为 Schema。
- 配置中心按 Schema 渲染表单、校验、生成文档。
- 新增配置必须同时提供：默认值、校验、说明、是否敏感、是否可热更新。

#### 7.10.3 数据模型

| 表 | 用途 |
|---|---|
| `system_config` | 当前生效配置：`key UK, value jsonb, version, checksum, updated_by, updated_at` |
| `system_config_history` | 变更历史/回滚：`id, key, old_value, new_value, version, operator, note, created_at` |

> 敏感值只存密文/Secret 引用；历史记录同样脱敏。
> **env 边界**：业务账号与外部密钥（JD、1Panel 等）不写 env，统一写入 `system_config`；env 只保留基础设施/启动配置。

#### 7.10.4 管理 API

```
GET  /api/v1/settings/config/schema          # 全部配置 Schema
GET  /api/v1/settings/config                 # 当前值（脱敏）
PUT  /api/v1/settings/config                 # 批量更新（校验+热更新）
POST /api/v1/settings/config/validate        # 只校验
POST /api/v1/settings/config/reset           # 重置为默认
GET  /api/v1/settings/config/history         # 变更历史
POST /api/v1/settings/config/rollback/:ver   # 回滚
```

- 权限：`settings.config.read`、`settings.config.write`。
- 所有变更审计；敏感项不回显。
- 前端：设置页按 Schema 分组渲染；V3.0 先核心配置，表单可后补。

#### 7.10.5 V3.0 覆盖范围

| 模块/能力 | 配置项（示例） |
|---|---|
| server/log | 端口、模式、日志级别/格式/轮转、超时 |
| database/redis | 连接池、超时、重试；DSN 走 Secret |
| auth/RBAC | JWT 过期、刷新、登录限流/锁定、密码策略、API Key 策略 |
| security | CORS、限流、上传大小、管理入口白名单 |
| monitor/tracing | 采样率、指标开关、健康检查间隔 |
| scheduler | 并发、超时、重试、保留天数 |
| integration/webhook | 幂等 TTL、限流、重试/退避、签名、保留天数 |
| AI/Browser | Provider/模型/超时、Browser 地址/并发（低优先级能力参数） |
| host/backup | SSH 超时、并发、备份保留、存储目标（备份 V3.1） |
| 项目管理 | 分页上限、生成引擎、auto_commit 开关、导入限制 |

#### 7.10.6 验收

- 核心配置项 100% 有 Schema、默认值、说明、校验。
- 修改可热更新项无需重启；失败自动回滚。
- 敏感项不出现在 API/日志/历史明文。
- 配置优先级、锁定、回滚、审计有测试。
- 前端可按 Schema 自动渲染表单。

---

## 8. 项目管理：完整追踪 V3

### 8.1 目标

把 developmentManager 从“项目台账 + 需求”升级为**完整项目管控平台**，支撑 V3.0 及后续业务模块迁移：

```
项目
 ├── 里程碑（V3.0 / V3.1 ...）
 │    ├── 需求（RQM-V3-xxx）
 │    │    └── 任务（V3-Px-yy）
 │    │         ├── 依赖
 │    │         ├── 日志/工时
 │    │         ├── 检查项
 │    │         ├── 评论
 │    │         ├── 标签
 │    │         └── 资产
 │    ├── 迭代（Sprint）
 │    ├── 风险
 │    ├── 发布
 │    └── 基线（计划快照）
 └── 资产/生成任务
```

### 8.2 现有能力与扩展

| 能力 | 现状 | V3 |
|---|---|---|
| 项目台账 | ✅ | 保持 |
| 需求/版本/状态 | ✅ | 增加里程碑/迭代关联 |
| 资产 | ✅ | 增加任务资产关联 |
| 部署/字典/生成 | ✅ | 保持 |
| 里程碑 | ❌ | 新增 |
| 任务/WBS | ❌ | 新增（父子任务） |
| 任务依赖 | ❌ | 新增（FS/SS/FF/SF + 环检测） |
| 任务日志/工时 | ❌ | 新增 |
| 风险 | ❌ | 新增（概率/影响/缓解/负责人） |
| 发布 | ❌ | 新增（版本/时间/变更/回滚点） |
| 基线 | ❌ | 新增（计划快照 JSONB + 对比） |
| 迭代/Sprint | ❌ | 新增 |
| 评论/检查项/标签 | ❌ | 新增 |
| 路线图/看板/燃尽 | ❌ | 新增 API + 前端 |

### 8.3 数据模型（迁移 `0002_v3_pm_full.sql`）

> 所有表前缀 `zzz_development_manager_`；含审计字段与软删除。

#### 8.3.1 核心表

**milestone — 里程碑/版本**

| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| project_id | bigint FK | 所属项目 |
| version | varchar(32) | 版本号，如 `V3.0` |
| name | varchar(128) | 名称 |
| goal | text | 目标与范围 |
| status | varchar(16) | planned/active/blocked/done/cancelled |
| start_date / target_date | date | 计划起止 |
| released_at | timestamptz | 实际发布 |
| owner | varchar(64) | 负责人 |
| deleted / created_at / updated_at | — | 审计 |
| UK(project_id, version) | — | 项目内版本唯一 |

**task — 任务/WBS**

| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| project_id | bigint FK | 项目 |
| milestone_id | bigint FK | 里程碑 |
| iteration_id | bigint FK NULL | 迭代 |
| requirement_id | bigint FK NULL | 需求 |
| parent_id | bigint FK NULL | 父任务 |
| code | varchar(32) | 任务编号 |
| title | varchar(255) | 标题 |
| description | text | 说明/验收标准 |
| type | varchar(16) | refactor/feature/test/docs/ops/migration/risk |
| status | varchar(16) | todo/doing/blocked/review/done/cancelled |
| priority | varchar(4) | P0–P3 |
| estimate_hours / actual_hours | numeric(6,1) | 工时 |
| owner | varchar(64) | 负责人 |
| start_date / due_date | date | 计划 |
| started_at / done_at | timestamptz | 实际 |
| order_no | int | 排序 |
| deleted / created_at / updated_at | — | 审计 |
| UK(project_id, code) | — | 编号唯一 |

**task_dependency — 依赖**

`id, task_id FK, depends_on_task_id FK, dep_type(FS/SS/FF/SF), lag_days int, created_at`
`UK(task_id, depends_on_task_id)`；禁止自依赖；服务层环检测。

**task_log — 日志/工时**

`id, task_id FK, from_status, to_status, hours numeric(6,1), note text, operator varchar(64), created_at`

#### 8.3.2 完整管控表

**risk — 风险**

`id, project_id, milestone_id NULL, task_id NULL, title, description, probability(low/medium/high), impact(low/medium/high), level(low/medium/high/critical), mitigation text, owner, status(open/mitigating/closed/accepted), due_date, deleted, audit`

**release — 发布**

`id, project_id, milestone_id NULL, version, name, release_type(major/minor/patch/hotfix), status(planned/deploying/deployed/rolled_back), released_at, change_summary text, rollback_plan text, deployed_by, deleted, audit`

**baseline — 计划基线**

`id, project_id, milestone_id NULL, name, snapshot jsonb, note, created_by, created_at`
`snapshot` 保存当时的里程碑/任务/预估/日期，用于计划 vs 实际对比。

**iteration — 迭代/Sprint**

`id, project_id, milestone_id NULL, name, goal, start_date, end_date, status(planned/active/closed), capacity_hours numeric(8,1), deleted, audit`

**comment — 评论**

`id, project_id, target_type(requirement/task/risk/release), target_id, content text, operator, created_at`
> 也可拆分为 requirement_comment/task_comment；先用统一表，服务层校验 target_type。

**task_checklist — 检查项**

`id, task_id FK, title, done boolean, order_no, created_at, updated_at`

**label — 标签**

`id, project_id, name, color varchar(16), UK(project_id,name)`

**task_label — 任务标签**

`id, task_id FK, label_id FK, UK(task_id,label_id)`

**task_asset — 任务资产**

`id, task_id FK, asset_id FK, relation_type(input/output/evidence), created_at`
> 复用现有 `project_asset`，不重复存文件。

### 8.4 API 扩展

```
# 里程碑
GET|POST        /projects/:id/milestones
GET|PUT|DELETE  /milestones/:id
GET             /milestones/:id/progress

# 任务
GET|POST        /projects/:id/tasks
GET|PUT|DELETE  /tasks/:id
POST            /tasks/:id/status
POST            /tasks/:id/logs
GET             /tasks/:id/logs
POST            /tasks/:id/dependencies
DELETE          /tasks/:id/dependencies/:dep_id
GET|POST        /tasks/:id/checklist
PUT|DELETE      /tasks/:id/checklist/:item_id
GET|POST        /tasks/:id/comments

# 风险/发布/基线/迭代
GET|POST        /projects/:id/risks
GET|PUT|DELETE  /risks/:id
GET|POST        /projects/:id/releases
GET|PUT|DELETE  /releases/:id
GET|POST        /projects/:id/baselines
GET             /baselines/:id/diff
GET|POST        /projects/:id/iterations
GET|PUT|DELETE  /iterations/:id

# 标签/资产
GET|POST        /projects/:id/labels
POST|DELETE     /tasks/:id/labels
GET|POST        /tasks/:id/assets

# 视图/报表/导入导出
GET             /projects/:id/roadmap
GET             /projects/:id/board
GET             /projects/:id/burndown?from=&to=
GET             /projects/:id/gantt
GET             /projects/:id/report/weekly
POST            /projects/:id/import
GET             /projects/:id/export
```

### 8.5 权限扩展

| 资源 | 动作 | 说明 |
|---|---|---|
| `dm.milestone` | read/create/update/delete | 里程碑 |
| `dm.task` | read/create/update/delete/assign/import | 任务、依赖、日志、检查项、评论 |
| `dm.risk` | read/create/update/delete | 风险 |
| `dm.release` | read/create/update/delete | 发布 |
| `dm.baseline` | read/create/delete | 基线 |
| `dm.iteration` | read/create/update/delete | 迭代 |

默认角色：`editor` read/create/update/assign；`viewer` read；`admin` 全部；`import` 单独授权。

### 8.6 前端页面

1. **路线图**：里程碑时间轴 + 阶段进度 + 风险/逾期。
2. **任务看板**：todo/doing/review/done；拖拽改状态；按负责人/优先级/标签筛选。
3. **任务详情**：描述/验收/依赖/日志/检查项/评论/标签/资产。
4. **甘特/日历**：里程碑/任务/迭代时间条。
5. **风险登记**：概率×影响矩阵、缓解计划、负责人。
6. **发布日历**：版本/发布时间/回滚点。
7. **报表**：里程碑完成率、需求完成率、燃尽、工时、阻塞时长。
8. **导入向导**：V3 路线图 Markdown/JSON 预览 + 幂等导入。

### 8.7 V3 台账初始化

- 项目：`go-backend-core`。
- 里程碑：`V3.0`（基座 + 公共 + 项目管理），后续 `V3.1` goodser、`V3.2` ia、`V3.3` login3rd、`V3.4` shopchecker。
- 需求：V3.0 16 条 + 后续 4 条（见路线图）。
- 任务：V3.0 61 个（编号 `V3-P0-01` … `V3-P8-11`、`V3-P9-05`），含依赖、预估、验收。
- 资产：本规划、V3 路线图、V3 SOP、迁移报告、V3 数据字典、发布说明。
- 生成任务：每阶段可用 `kind=plan/doc`；`kind=code` 只产出分支草案。

### 8.8 运转机制

| 节奏 | 动作 | 产出 |
|---|---|---|
| 每日 | 任务状态/工时/阻塞更新 | 看板、任务日志 |
| 每周一 | 周计划：任务、依赖、风险 | 周计划 |
| 每周五 | 周复盘：完成率、逾期、阻塞、下周计划 | 周报 |
| 每阶段末 | 验收：DoD、测试、文档、迁移校验 | 阶段验收记录 |
| 发布前 | 里程碑评审：Exit Criteria 逐条勾选 | 发布检查表 |

**任务 DoD：**

1. 代码 + 单元测试 + `golangci-lint` 通过。
2. SQL 变更配 PG 集成测试或 golden SQL 测试。
3. 表结构变更配迁移 + 数据字典 + 文档。
4. API 变更配 Swagger + 模块文档。
5. 提交引用任务编号。
6. 人工评审通过并在任务日志留痕。

### 8.9 指标

| 指标 | 计算 | 目标 |
|---|---|---|
| 里程碑进度 | `sum(done.estimate)/sum(estimate)` | 按计划 |
| 需求完成率 | done/total | 100% |
| 阻塞任务数/时长 | status=blocked + 日志 | 0 个 > 2 天 |
| 逾期任务 | due_date < today 且未 done | 0 |
| 方言清零 | grep 门禁计数 | 0 |
| 集成测试覆盖 | 有 PG 测试的仓库/总仓库 | ≥ 80% |
| p95 延迟 | 关键列表 API | ≤ V2 × 1.2 |
| 数据校验通过率 | 校验项通过/总项 | 100% |
| AI 返工率 | 评审打回任务/总任务 | ≤ 15% |

---

## 9. WBS 与排期（V3.0 → 2026-12-15）

> 任务级明细见 [V3 路线图](../projects/go-backend-core/v3/README.md)。
> V3.0 共 **61 个任务、约 66.5 人日**；其中 **12.15 核心上线 60 个任务、65.5 人日**，PM 高级前端 1.0 人日放 12.15 后。
> 4 周窗口 + AI 并行执行；人工评审、数据迁移演练、公网安全评审是关键路径。

### 9.1 12.15 上线批次

| 批次 | 范围 | 说明 |
|---|---|---|
| **12.15 核心上线（已确认）** | P0–P9；PM-01–06、PM-08；配置中心；公网反代/证书/HTTPS；多主机台账/SSH；维护模式；公网认证/权限硬化 | 基座 + 公共 + PM + 配置中心 + 多主机 + 维护 + 公网 |
| **12.15 后补齐（12/31 前）** | PM-07 高级前端（风险矩阵/发布日历/甘特/报表）；V3.1 准备 | 不阻塞 12.15 核心上线 |
| **后续待定** | 1Panel API/数据/MCP/多实例/密钥入库 | 单独立项，不排期 |
| **后续里程碑** | V3.1 goodser、V3.2 ia、V3.3 login3rd、V3.4 shopchecker | 按模块迁移 |

> 如果 12/15 前强行塞入 PM-07 与 1Panel，会挤压迁移演练/公网安全评审。建议按上表分批，保证上线稳定。

### 9.2 阶段汇总

| 阶段 | 内容 | 任务数 | 人日 | 依赖 | 验收标准 |
|---|---|---:|---:|---|---|
| **V3-P0 规划与基线** | 冻结 V2、确认范围、建分支/看板 | 3 | 2.5 | — | 决策/范围签字；V2 快照 |
| **V3-P1 数据层与配置中心** | 配置、Manager、`Main()/PG()`、健康检查、Compose、配置 Schema/热更新/管理 API | 6 | 7.0 | P0 | V3 可连 PG；配置中心可用 |
| **V3-P2 Schema 基线** | PG baseline、类型/索引/触发器、GORM 表迁入、迁移器、ops/config 表 | 6 | 6.5 | P1 | 空库建约 51 表；迁移可重复 |
| **V3-P3 方言改造** | 占位符、UPSERT、JSON、日期、元数据（基座/公共/PM） | 6 | 8.0 | P2 | 方言门禁清零；PG 测试通过 |
| **V3-P4 基础/公共模块** | auth/scheduler/outbox/fileref/ops/audit/feature/migrate + 公网认证硬化 | 6 | 7.0 | P3 | 基础模块 PG 冒烟全绿；公网登录安全 |
| **V3-PM 项目管控** | 完整模型/API/权限/核心前端/导入（PM-07 后补） | 7 | 9.0 | P2 | PM 核心/高级 API 可用；核心页面可用 |
| **V3-SOP 规范与 AI** | V3 SOP、AI 上下文包、试点任务 | 3 | 3.0 | P0 | SOP 可执行；试点通过 |
| **V3-P6 数据 bootstrap** | 工具、加载、校验、演练/报告 | 4 | 4.0 | P2/P3 | 校验 100%；窗口内完成 |
| **V3-P7 测试/CI** | PG 测试基座、集成测试、迁移测试、CI | 5 | 5.5 | P3 | CI 全绿；覆盖 ≥ 80% |
| **V3-P8 部署/文档** | 新机器、公网反代/证书、多主机台账/SSH、维护模式、备份、监控、文档、UAT | 9 | 9.0 | P4/P6 | 公网 HTTPS 可用；主机台账/维护模式可用；部署演练通过 |
| **V3-P9 上线/并行** | 新机器上线、观察、发布、公网安全评审、V3.1+ 准备 | 5 | 4.0 | 全部 | 7 天稳定；V2 冻结；老机器不下线 |
| **合计（12.15 批次）** |  | **60** | **65.5** |  |  |
| **后补** | PM-07 高级前端 | 1 | 1.0 |  | 12/31 前 |
| **总计** |  | **61** | **66.5** |  |  |

### 9.3 4 周排期（11/18 → 12/15）

| 周 | 日期 | 主线 | 并行 | 里程碑 |
|---|---|---|---|---|
| W1 | 11/18–11/24 | P0 + SOP-01/02 | P1 配置/Manager/配置中心；P2 baseline 启动；PM-01 | 范围/SOP/数据层/配置中心 |
| W2 | 11/25–12/01 | P2 baseline + GORM 表 | P3 codemod/基座 SQL；PM-02/03；auth bootstrap | Schema 基线 |
| W3 | 12/02–12/08 | P3 收尾 + P4 | P6 bootstrap；P7 测试/CI；PM-04/05/06；多主机台账/SSH；反代/证书准备 | 方言/公共/PM/主机 |
| W4 | 12/09–12/15 | P8 部署 + P9 上线 | UAT；公网安全评审；PM-08 导入；缓冲 | 12.15 上线 |
| 后补 | 12/16–12/31 | PM-07 高级前端 | V3.1 准备 | 12/31 补齐 |

> **4 周非常紧**：如果 P2/P3 延期，优先砍 PM-07，不砍迁移演练、PG 测试、公网安全评审。每天站会跟踪关键路径；每周五复盘。

### 9.4 关键路径

```
P0 → P1 → P2 → P3 → P4 → P6 → P8 → P9
                  └── PM-01..06/08（并行）
                  └── P7（并行，但 P7-02 依赖 P3-06）
```

- 12.15 前必须完成：P0–P4、P6、P7、P8、P9、PM-01–06/08、配置中心、公网反代/证书、多主机台账、公网认证硬化。
- 可后补：PM-07 高级前端、V3.1 迁移准备。
- 暂停：1Panel API/数据/MCP/多实例/密钥入库（后续单独立项）。
- 不可压缩：数据迁移演练、PG 集成测试、公网安全评审、回滚演练。

### 9.5 未来里程碑（不占 V3.0）

| 里程碑 | 内容 | 优先级 | 前置 |
|---|---|---|---|
| V3.1 | 多主机完整运维 + 数据库迁移/备份（主机间、冷备份） | 低 | 容器迁移到新机器 |
| V3.2 | 外部服务：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS | 低 | V3.1 稳定 |
| V3.3 | goodser 迁移（含目标识别 / 相似商品检查） | 中 | V3.0 稳定 |
| V3.4 | ia 迁移 | 中 | V3.3 稳定 |
| V3.5 | login3rd 迁移 | 中 | V3.4 稳定 |
| V3.6 | shopchecker 迁移 | 中 | V3.5 稳定 |
| V3.7 | 老机器下线评估 | 高 | 全部迁移 + 稳定 30 天 |

每个业务模块都要走 V3 SOP 的“模块接入流程”，并可能包含业务需求修改/新增。

### 9.6 1Panel 接入（暂停，后续待定）

- 1Panel API/数据源/MCP/多实例/密钥入库**不进入 V3.0**。
- V3.0 只要求公网反代/证书可用；1Panel 可作为边缘实现之一，也可用 OpenResty。
- 后续单独立项时再启用 API 联调、多实例与密钥入库设计（见 §7.6 评估资料）。

---

## 10. 测试与验收

### 10.1 测试分层

| 层级 | 范围 | 工具 | 门禁 |
|---|---|---|---|
| 单元测试 | 纯逻辑、解析、状态机 | `go test -short` | 必须通过 |
| 仓库集成测试 | 每个 repository 的 CRUD/分页/JSON/upsert | 真实 PG | 必须通过 |
| 迁移测试 | 空库 up、重复执行、down、checksum | PG | 必须通过 |
| 数据 bootstrap 测试 | V2 选定表 → V3 load → verify | MySQL + PG | 必须通过 |
| API 冒烟 | 基座/公共/PM 核心路由 | `httptest`/脚本 | 必须通过 |
| 性能测试 | 列表/搜索/统计 | `EXPLAIN` + 基准 | p95 达标 |
| 安全测试 | SQL 注入、权限、密钥 | 人工 + 扫描 | 无高危 |
| AI 交付检查 | 任务契约、测试证据、范围 | SOP 清单 | 人工 review |

### 10.2 验收清单

- [ ] `go build ./...`、`go test -short ./...`、`golangci-lint run` 全绿。
- [ ] `go test -tags=integration ./...` 在 PG 上全绿。
- [ ] `migrations/postgres/0001` 在空库执行成功；重复执行幂等。
- [ ] 约 35 张基础表 + 13 张 PM 扩展表 + 3 张运维/配置表全部存在；数据字典一致。
- [ ] V2 bootstrap 校验报告 100% 通过。
- [ ] 基座/公共/PM 核心 API 冒烟通过。
- [ ] 业务模块未注册、未进入 V3 二进制；V2 老机器业务正常。
- [ ] GORM 在 PG 上工作；生产 `auto_migrate=false`。
- [ ] 项目管理完整功能可用（里程碑/任务/依赖/风险/发布/基线/迭代/看板/报表）。
- [ ] 配置中心可用：核心配置有 Schema/默认值/校验/热更新/审计/脱敏；管理 API 可用。
- [ ] 多主机台账可用：云服务器/主机/备份机登记、SSH 连通/状态、权限/审计。
- [ ] AI SOP 试点任务通过；任务台账完整。
- [ ] 新机器连续运行 7 天无 P0/P1；老机器未下线。
- [ ] `/api/v1` 契约未破坏。
- [ ] 公网反代可用：新二级域名 443/HTTPS、证书续期、反代本机 V3:80；HTTP 跳转 HTTPS；WebSocket/上传正常。
- [ ] 1Panel API/数据/MCP 接入未进入 V3.0；不阻塞上线。
- [ ] 公网用户由管理员创建/授权；默认无自助注册；登录限流/锁定/审计生效；后续注册/审批有任务。
- [ ] 调试端口 29092、MySQL/管理接口不暴露公网；管理接口有 IP 白名单/内网限制。
- [ ] V2 冻结生效：无新增功能；V3 为唯一开发线。
- [ ] 12.15 批次完成；后补项（PM-07）有负责人和日期。
- [ ] 1Panel 接入暂停已记录，后续单独立项。
- [ ] DB 迁移/备份与外部服务明确为 V3.1/V3.2 低优先级。

### 10.3 性能目标

| 场景 | 目标 |
|---|---|
| 列表 API p95 | ≤ V2 × 1.2 |
| 详情 API p95 | ≤ 200ms |
| 统计/报表 p95 | ≤ 1s |
| bootstrap 窗口 | ≤ 60 分钟（含校验） |
| 连接池 | 不超过 PG `max_connections` 的 70% |
| 1Panel/反代额外延迟 | ≤ 20ms（内网回源） |
| 公网登录接口 | 限流生效；无暴力破解风险 |

---

## 11. 风险与缓解

| # | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R1 | V3 与 V2 并行期数据归属混乱 | 中 | 高 | 明确 PM/用户/RBAC 在 V3 唯一事实源；V2 冻结；路由独立 |
| R2 | 公网用户/权限迁移遗漏或越权 | 中 | 高 | bootstrap 校验；管理员审批；RBAC 测试；最小权限；审计 |
| R3 | SQL 方言漏改，运行时才报错 | 高 | 高 | codemod + review + PG 集成测试 + grep 门禁 |
| R4 | GORM 与迁移 Schema 漂移 | 中 | 中 | 关闭 AutoMigrate；模型/Schema 对比测试 |
| R5 | 序列未重置导致主键冲突 | 高 | 高 | 自动 `setval` + 校验项 |
| R6 | JSONB/时间/布尔语义差异 | 中 | 中 | 显式映射 + 抽样校验 |
| R7 | 项目管理扩展范围过大拖慢上线 | 中 | 中 | 分 PM-A/B/C 三个阶段，但模型一次规划完整 |
| R8 | AI 生成代码质量/越界 | 中 | 高 | 任务契约 + 子代理评审 + 人工合并 + 范围门禁 |
| R9 | 业务模块迁移时 V2/V3 数据不一致 | 中 | 高 | 按模块冻结 + 快照 + 增量 + 校验 + 回滚 |
| R10 | 老机器长期不下线带来运维双份成本 | 高 | 中 | 设定模块迁移节奏；老机器只维护不新增功能 |
| R11 | 性能回退 | 中 | 中 | 索引评审 + `EXPLAIN` + p95 门禁 |
| R12 | 文档/环境变量遗漏 | 中 | 低 | 发布检查表 + grep + 部署演练 |
| R13 | **12.15 窗口过紧** | 高 | 高 | 分“必上线/后补齐”；砍 PM-07，不砍测试/迁移/安全；每日站会 |
| R14 | 公网暴露导致攻击面扩大 | 中 | 高 | 反代限流/安全头/HTTPS；管理接口白名单；登录锁定；依赖扫描 |
| R15 | 反代/证书配置错误导致不可用/真实 IP 丢失 | 中 | 高 | 边缘配置清单 + 预发演练 + `TrustedProxies` + 日志校验 |
| R16 | 1Panel 接入暂停导致后续返工 | 中 | 低 | 明确不在 V3.0；只保留通用反代接口；后续单独立项 |
| R17 | V2 冻结后仍有功能诉求 | 高 | 中 | 需求转 V3 里程碑；V2 仅安全修复；PO 统一口径 |
| R18 | 新机器反代单点故障 | 中 | 高 | 反代健康检查；证书/配置备份；备用入口预案 |
| R19 | 老机器下线被延迟 | 高 | 中 | 老机器保留老域名/V2；V3 独立边缘；全部模块迁移后评估下线 |
| R20 | 配置热更新错误/优先级混乱 | 中 | 高 | Schema/校验/回滚/审计；敏感项 Secret；灰度+失败回滚 |
| R21 | 主机凭据/SSH 越权 | 中 | 高 | AES-GCM 加密、指纹校验、命令白名单、最小权限、审计 |

---

## 12. 交付物

| 类别 | 交付物 |
|---|---|
| 代码 | PG 数据层、配置中心、基座/公共/PM 方言改造、GORM PG Store、多主机台账、`cmd/v3migrate` |
| Schema | `migrations/postgres/0001_v3_baseline.sql`、`0002_v3_pm_full.sql`、后续迁移 |
| 测试 | PG 集成测试、迁移测试、bootstrap 校验、性能报告 |
| 部署 | 新机器 Compose、`.env.example`、公网反代/证书配置清单（站点/证书/反代本机/限流/安全头）、备份/恢复、监控面板 |
| 文档 | 本规划、V3 路线图、V3 SOP、V3 数据字典、部署/迁移/并行 Runbook、公网安全清单、发布说明 |
| 台账 | developmentManager 中 V3.0 里程碑/16 需求/61 任务/依赖/风险/发布/基线/迭代/资产 |
| 报告 | bootstrap 校验报告、回滚演练报告、AI 试点报告、公网安全评审、上线复盘 |

---

## 13. 待确认问题

| # | 问题 | 建议 |
|---|---|---|
| Q1 | V3 域名是什么？DNS 是否指向新机器？ | 上线前确定；反代/证书实现待定 |
| Q2 | 公网用户先创建：由谁创建/分配角色？后续是否做注册/审批？ | V3 管理员创建；后续迭代 |
| Q3 | V2 业务模块并行期用什么入口？ | 继续老域名；V3 新域名；向用户说明两套入口 |
| Q4 | 12.15 核心上线已确认；PM-07 12/31 补齐是否确认？ | 是 |
| Q5 | 公网反代/证书用 1Panel 边缘还是 OpenResty？ | P8 前确定；不影响 P0/P1 |
| Q6 | 管理接口是否限制内网/IP 白名单？ | 建议是 |
| Q7 | AI 试点任务选哪个？ | V3-P1-02（database.Manager） |
| Q8 | 业务模块迁移顺序？ | goodser → ia → login3rd → shopchecker |

> 以上确认后，P0 结束，进入 P1。未经确认不进入开发（遵循 `docs/module-sop.md` 四段式 SOP）。

---

## 14. 参考

- [现有架构](../architecture.md)
- [技术选型](../tech-stack.md)
- [模块设计](../module-design.md)
- [数据模型](../data-model.md)
- [部署方案](../deployment.md)
- [模块 SOP](../module-sop.md)
- [V2 项目开发管理规划](feature-V2-project-dev-management.md)
- [V3 路线图与任务台账](../projects/go-backend-core/v3/README.md)
- [V3 SOP](../projects/go-backend-core/v3/SOP.md)
