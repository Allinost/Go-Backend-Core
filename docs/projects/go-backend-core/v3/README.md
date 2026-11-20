# Go-Backend-Core V3 路线图与任务台账

> 用途：V3（PostgreSQL 基座 + 项目化管控）的唯一任务级事实源。
> 人类阅读看本文件；机器导入看文末「导入映射」。
> 主规划：[docs/plans/V3-postgres-refactor-2026-11-18.md](../../../plans/V3-postgres-refactor-2026-11-18.md)
> V3 SOP：[SOP.md](SOP.md)
> 状态：**V3.0 范围/上线窗口已确认（2026-12-15），待 P0 启动** · 更新日期：2026-11-18

---

## 1. 项目与里程碑

| 项 | 值 |
|---|---|
| 项目 | `go-backend-core`（Go-Backend-Core 后端基座） |
| 当前里程碑 | `V3.0` |
| 名称 | PostgreSQL 基座、公共能力与项目化管控 |
| 目标 | 新机器 PG-only；公网反代/证书；完成基座/公共/项目管理；业务模块留在 V2，后续逐步迁移 |
| 状态 | `planned`（P0 启动中） |
| 上线日期 | **2026-12-15 核心上线**（已确认；PM 高级前端 12/31 前补齐） |
| 预估 | V3.0：61 任务 / 66.5 人日；12.15 批次 60 任务 / 65.5 人日；AI 主执行 + 人工评审 |
| 关键路径 | P0 → P1 → P2 → P3 → P4 → P6 → P8 → P9 |
| 并行流 | V3-PM（完整项目管理）、V3-SOP（AI 开发规范） |
| 后续里程碑 | V3.1 goodser · V3.2 ia · V3.3 login3rd · V3.4 shopchecker |

### 1.1 V3.0 范围

**包含：**

- 基座模块：health/auth/migrate/ops/settings/integration/webhook/servicenav/scheduler/net/crypto；删除 s1/s2。
- 公共能力：auth/audit/cache/config/eventbus/feature/fileref/migrate/monitor/outbox/scheduler/storage/tracing/ws 等。
- 项目管理：developmentManager + 完整项目管理扩展。
- 新机器部署、PG、Redis、S3、Chrome、监控、备份。
- 公网反代/证书/HTTPS（1Panel 或 OpenResty，实现待定）；容器内 80；公网用户由管理员创建并授权；默认无自助注册。
- V3 SOP 与 AI 开发工作流。
- 新二级域名 → 新机器反代 → 本机 V3:80；老二级域名 → 老机器 → V2，两套入口独立。
- 新旧机器同局域网，可互访，仅用于 bootstrap/迁移/运维，不做生产反代。

**不包含：**

- `goodser / ia / login3rd / shopchecker` 的代码、数据、路由切换。
- V2 → V3 全量数据迁移。
- 老机器下线。
- 前端整体重构。
- **1Panel API/数据/MCP/多实例/密钥入库（暂停，后续单独立项）**。
- 公网自助注册（默认关闭）。

---

## 2. 需求清单（V3.0 16 条）

| 编号 | 需求 | 优先级 | 覆盖任务 | 验收标准 |
|---|---|---|---|---|
| RQM-V3-001 | PostgreSQL 单一运行时 | P0 | P1、P8、P9 | V3 只连 PG；无 MySQL 主库 |
| RQM-V3-002 | Schema 基线与迁移治理 | P0 | P2 | 空库建约 51 张表（基础/PM/主机/配置）；迁移可重复/可回滚 |
| RQM-V3-003 | 基座/公共/PM 方言改造 | P0 | P3 | MySQL 方言 grep 清零；PG 测试通过 |
| RQM-V3-004 | GORM PG 策略 | P0 | P3、P4 | PG 驱动工作；生产关闭 AutoMigrate |
| RQM-V3-005 | 项目管理核心 | P0 | PM-01–04 | 里程碑/任务/依赖/日志/路线图/看板可用 |
| RQM-V3-006 | 项目管理完整能力 | P1 | PM-05–08 | 风险/发布/基线/迭代/评论/检查项/标签/资产/报表/导入 |
| RQM-V3-007 | V3 SOP 与 AI 开发 | P0 | SOP-01–03 | SOP 可执行；AI 试点通过 |
| RQM-V3-008 | V3.0 数据 bootstrap | P0 | P6 | 基座/公共/PM 数据校验 100% |
| RQM-V3-009 | PG 集成测试与 CI | P0 | P7 | CI 含真实 PG；仓库覆盖 ≥ 80% |
| RQM-V3-010 | 新机器部署与监控 | P1 | P8 | 新机器稳定；备份/告警可用 |
| RQM-V3-011 | 双环境并行与老机器保留 | P0 | P9 | V3 稳定 7 天；V2 冻结但正常运行；不下线 |
| RQM-V3-012 | 文档与 V3 数据字典 | P1 | P2-06、P8-03 | 架构/选型/数据模型/部署/模块文档同步 |
| RQM-V3-013 | 公网访问与反代/证书 | P0 | P4-06、P8-06、P9-05 | 新二级域名 443/HTTPS/证书/反代；容器 80；管理员创建用户；默认无自助注册；管理接口受限 |
| RQM-V3-014 | 多主机管理（最小） | P0 | P8-09、P8-10 | 云服务器/主机/备份机台账、SSH 连通/状态、权限/审计 |
| RQM-V3-015 | 配置中心 | P0 | P1-06 | 核心配置有 Schema/默认值/校验/热更新/审计/脱敏/管理 API |
| RQM-V3-016 | L1/L2 手动维护切换 | P0 | P8-11 | 停机/迁移可迁回/迁移不迁回；同步/反向同步确认；L1 切换、ACloud 固定 |

---

## 3. 任务台账（V3.0：58 个；12.15 批次 57 个）

> 字段：编号 · 任务 · 类型 · 优先级 · 预估人日 · 依赖 · 状态 · 验收/备注
> 状态：`todo` 待办 · `doing` 进行中 · `blocked` 阻塞 · `review` 待验收 · `done` 完成 · `cancelled` 取消。

### V3-P0 规划与基线（3 个，2.5 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P0-01 | 冻结 V2 基线：tag、MySQL 快照、行数与数据字典 | ops | P0 | 1.0 | — | todo | 产出 V2 快照与行数清单 |
| V3-P0-02 | 确认 V3.0 范围、并行期认证/数据归属/路由 | ops | P0 | 1.0 | P0-01 | doing | 决策表签字；本文件定稿 |
| V3-P0-03 | 建 `feature-V3`、CI 基线、看板与周报机制 | ops | P1 | 0.5 | P0-02 | todo | 分支/看板/周报模板就绪 |


### V3-P1 数据层与配置中心（6 个，7 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P1-01 | 配置 PG 主库：`sslmode/search_path/timezone/池` + `.env.example` | refactor | P0 | 1.0 | P0-03 | done | `database.postgres.main` 可加载 |
| V3-P1-02 | `database.Manager`：PG 主库、`Main()/PG()`、健康检查、热加载 | refactor | P0 | 1.5 | P1-01 | done | 模块不再直接取 MySQL |
| V3-P1-03 | pgx stdlib `*sql.DB` 封装、事务助手、错误映射 | refactor | P1 | 1.0 | P1-02 | done | 现有仓库可复用 `*sql.DB` |
| V3-P1-04 | 启动 fail-fast、s0→health 改名、s1/s2 删除、移除 MySQL fallback | refactor | P0 | 1.0 | P1-02 | done | health 路由可用；PG 不可用即退出；MySQL 兼容保留给未迁移模块 |
| V3-P1-05 | Compose/.env/config：postgres 服务、移除 V3 mysql | ops | P1 | 0.5 | P1-01 | blocked | 待业务模块移出/认证迁 PG 后切 V3 Compose |
| V3-P1-06 | 配置中心：Schema/默认值/校验/优先级/热更新/审计/脱敏/管理 API | feature | P0 | 2.0 | P1-01 | doing | 框架/内存/API 已落地；PG 持久化待 P2 建表 |

### V3-P2 Schema 基线（6 个，6.5 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P2-01 | PG baseline `004` + PM `005`：约 35 张基座/公共/PM 核心表 + 13 张 PM 扩展；`002` 配置中心 2 表、`003` 主机/维护 2 表已完成（identity/boolean/timestamptz/jsonb/numeric） | migration | P0 | 2.0 | P1-03 | done | dev 已建 52 表、49 个外键 |
| V3-P2-02 | 索引/约束/FK/部分索引/GIN 评审与补全 | migration | P1 | 1.5 | P2-01 | todo | 关键查询有计划可用索引 |
| V3-P2-03 | GORM 表迁入迁移（auth/scheduler/fileref/audit/feature） | migration | P0 | 1.0 | P2-01 | todo | 生产禁用 AutoMigrate |
| V3-P2-04 | 迁移器 PG 默认：`_migrations` 基线、checksum、MySQL 归档 | refactor | P0 | 1.0 | P2-01 | todo | `migrate` 模块默认走 PG |
| V3-P2-05 | `updated_at` 触发器 + 公共函数 + 应用层兜底 | migration | P1 | 0.5 | P2-01 | todo | 更新时间不依赖 MySQL 语法 |
| V3-P2-06 | 数据字典 PG 化与 V3.0 表登记 | docs | P1 | 0.5 | P2-01 | todo | 表用途/字段含义完整 |

### V3-P3 方言改造（6 个，8 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P3-01 | codemod：SQL 字符串 `?`→`$n`，处理引号/注释/运算符 + 测试 | refactor | P0 | 1.5 | P2-04 | doing | 暂采用 `dbutil.Rebind` 运行期转换，全量 codemod 待补 |
| V3-P3-02 | 通用查询 helper：动态 IN、JSONB、ILIKE、分页、Quote | refactor | P0 | 0.5 | P3-01 | done | `internal/pkg/dbutil` 已落地并测试 |
| V3-P3-03 | 基座模块 SQL：webhook/servicenav/integration/outbox/ops | refactor | P0 | 2.0 | P3-01 | done | PG CRUD/JSONB/ILIKE 已实测 |
| V3-P3-04 | developmentManager 仓库：`DATABASE()`/info_schema/upsert/分页 | refactor | P0 | 1.5 | P3-01 | done | 台账/部署/字典/需求/生成全切 PG；字典扫描与 upsert 已实测 |
| V3-P3-05 | Store 改造：auth/scheduler/audit/feature/fileref/migrate | refactor | P0 | 1.5 | P3-01 | done | auth/scheduler/audit/feature/fileref/migrate 均已切 PG |
| V3-P3-06 | 方言清零门禁 + golden SQL 测试 + 全仓 grep | test | P0 | 1.0 | P3-03..05 | done | `TestV3DialectGate` 已落地，V3 模块方言计数为 0 |

### V3-P4 基础/公共模块（6 个，7 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P4-01 | auth User/RBAC/ApiKey：PG 驱动/迁移/默认权限 | refactor | P0 | 2.0 | P3-05 | done | HTTP 登录/RBAC/API Key 已验证；自助注册关闭属 P4-06 |
| V3-P4-02 | scheduler PG Store：tasks/logs/`NOW()`/AutoMigrate 关闭 | refactor | P0 | 1.0 | P3-05 | done | PG 持久化/JSONB/日志已验证 |
| V3-P4-03 | outbox + integration action log：upsert/jsonb/时间 | refactor | P1 | 1.0 | P3-03 | done | PG 外盒/连接器/动作日志已实测 |
| V3-P4-04 | fileref + ops：DDL→迁移、`current_database()`、扫描、引号 | refactor | P1 | 1.0 | P3-03 | done | 引用/扫描目标 PG 已实测 |
| V3-P4-05 | audit + feature + migrate：PG Store/`ON CONFLICT`/dumper | refactor | P1 | 1.0 | P3-05 | done | 审计/开关 PG 已实测；迁移器 PG 默认 |
| V3-P4-06 | 公网认证/权限硬化：管理员创建用户并授权、限流/锁定/审计、关闭自助注册 | refactor | P0 | 1.0 | P4-01 | done | 注册开关默认关、管理员建用户、登录失败锁定已实现；全局限流已启用 |

### V3-PM 项目管控（8 个，10 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-PM-01 | PM 核心迁移：milestone/task/task_dependency/task_log | migration | P0 | 1.0 | P2-01 | done | 表/索引/外键就绪，dev 已验证 |
| V3-PM-02 | PM 核心后端：models/repo/service/handler/routes/进度 | feature | P0 | 2.0 | PM-01、P3-04 | done | 里程碑/任务 CRUD、依赖环检测、状态流转、工时/日志、进度已实测；PM 走 PG |
| V3-PM-03 | PM 权限 + 默认角色 + 幂等导入 | feature | P1 | 1.0 | PM-02 | done | `dm.task/milestone` 权限已播种；任务按 code 幂等导入已实测 |
| V3-PM-04 | PM 前端核心：路线图/看板/任务抽屉 | feature | P0 | 2.0 | PM-02 | todo | 可视化可用 |
| V3-PM-05 | PM 高级迁移：risk/release/baseline/iteration/comment/checklist/label/task_label/task_asset | migration | P1 | 1.0 | PM-01 | done | 005 已建 9 张高级表并验证外键 |
| V3-PM-06 | PM 高级后端：风险/发布/基线/迭代/评论/检查项/标签/资产 | feature | P1 | 1.0 | PM-05 | done | CRUD/快照/关联已实测；前端待 PM-07 |
| V3-PM-07 | PM 高级前端：风险矩阵/发布日历/甘特/报表 | feature | P1 | 1.0 | PM-06、PM-04 | todo（12/31） | 页面可用；12.15 后补齐 |
| V3-PM-08 | V3 台账导入 + 周报机制 + 仪表盘 | ops | P1 | 1.0 | PM-03、PM-04 | done | 61 任务/61 依赖已入库；仪表盘/看板/路线图/周报已实现 |

### V3-SOP 规范与 AI（3 个，3 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-SOP-01 | 编写 V3 SOP：AI 流程/PG/模块接入/测试/迁移/部署 | docs | P0 | 1.0 | P0-02 | done | `SOP.md` v1.9 已覆盖 |
| V3-SOP-02 | AI 上下文包：`aiPrompt/v3/` 模板/规则/评审清单 | docs | P0 | 1.0 | SOP-01 | done | `aiPrompt/v3/` 已可用 |
| V3-SOP-03 | AI 试点：用 SOP 完成一个真实任务并复盘 | test | P0 | 1.0 | SOP-02、P1-02 | doing | 本轮 PM/bootstrap/auth 已完成试点，复盘待归档 |

### V3-P6 数据 bootstrap（4 个，4 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P6-01 | `cmd/v3migrate`：allowlist/plan/dump/load/verify/report | migration | P0 | 1.0 | P2-01 | done | plan/dump/load/verify/rehearsal/report 全部可用 |
| V3-P6-02 | 批量加载：COPY/拓扑排序/类型转换/序列重置 | migration | P0 | 1.0 | P6-01 | done | 977 行 dump、969 行导入、bool/JSON 转换、FK 重试、序列重置已验证 |
| V3-P6-03 | 校验器：行数/主键/FK/JSON/时间/金额/抽样/序列 | test | P0 | 1.0 | P6-02 | doing | 行数/FK/孤儿关联校验通过；抽样/报告待补 |
| V3-P6-04 | bootstrap 演练 + 并行切换 Runbook + 签字 | ops | P0 | 1.0 | P6-03 | done | 空 schema 演练通过，自动清理 |

### V3-P7 测试与 CI（5 个，5.5 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P7-01 | PG 测试基座：testcontainers/CI service/schema bootstrap | test | P0 | 1.5 | P2-01 | todo | 本地/CI 均可跑 |
| V3-P7-02 | 仓库集成测试：base/webhook/servicenav/integration/auth/DM | test | P0 | 2.0 | P7-01、P3-06 | todo | 覆盖 ≥ 80% 仓库 |
| V3-P7-03 | 迁移测试：空库 up/重复/down/checksum | test | P1 | 0.5 | P2-04 | todo | 幂等/可回滚 |
| V3-P7-04 | CI：PG service、integration tag、覆盖率、方言 lint | ops | P0 | 0.5 | P7-01 | todo | PR 门禁生效 |
| V3-P7-05 | 性能基线：关键列表 EXPLAIN、p95、索引调整 | test | P1 | 1.0 | P7-02 | todo | p95 ≤ V2×1.2 |

### V3-P8 部署与文档（9 个，9.0 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P8-01 | 新机器部署：Compose/env/PG 卷/健康检查/密钥 | ops | P0 | 1.5 | P1-05、P4-05 | todo | 新机器可起 |
| V3-P8-02 | 备份恢复：`pg_dump/pg_restore`/保留策略/演练 | ops | P0 | 1.0 | P8-01 | doing | 备份/恢复脚本与 Runbook 已就绪；真实恢复演练待做 |
| V3-P8-03 | 文档：架构/技术选型/数据模型/部署/README/模块文档 | docs | P1 | 1.0 | P4-05 | doing | 部署手册已补；架构/数据模型文档待收口 |
| V3-P8-04 | 监控：PG exporter/面板/告警（连接/慢查询/复制） | ops | P1 | 0.5 | P8-01 | todo | 面板可用 |
| V3-P8-05 | UAT 脚本与检查表 | docs | P1 | 0.5 | P8-03 | done | `UAT.md` 检查表已就绪，待实际执行 |
| V3-P8-06 | 公网反代/证书：新二级域名（待定）站点/证书/HTTPS/反代本机 V3:80；转发头/限流/安全头/WS/上传 | ops | P0 | 1.0 | P1-05、P4-06 | todo | 域名就绪后配置；不阻塞 P0/P1；真实 IP 正确 |
| V3-P8-09 | 多主机台账/SSH：云服务器/主机/备份机、凭据加密、连通/状态 | feature | P0 | 1.5 | P3-02 | done | 台账/SSH 已实现；凭据不返明文 |
| V3-P8-10 | 多主机 API/权限/审计：CRUD、test、status、RBAC、审计 | feature | P0 | 0.5 | P8-09 | done | 主机台账/凭据加密/SSH 连通/状态已实测 |
| V3-P8-11 | 维护模式：`shutdown`/`migrate_return`/`migrate_no_return`、同步确认、反向同步确认、L1/ACloud URL 归属、状态/审计 | feature | P0 | 1.5 | P8-10、P1-06 | doing | 后端 API/状态机/审计已完成；前端维护页待接 |

### V3-P9 上线与并行（5 个，4.0 人日）

| 编号 | 任务 | 类型 | 优先级 | 预估 | 依赖 | 状态 | 验收/备注 |
|---|---|---|---|---:|---|---|---|
| V3-P9-01 | 新机器上线：最终 bootstrap/校验/启动/冒烟 | ops | P0 | 1.0 | P6-04、P8-05 | todo | 窗口内完成 |
| V3-P9-02 | 并行观察 7 天 + 回滚演练 + 事件记录 | ops | P0 | 1.0 | P9-01 | todo | 无 P0/P1；V2 正常 |
| V3-P9-03 | V3.0 发布：release notes/复盘/台账关闭 | docs | P0 | 1.0 | P9-02 | todo | v3.0.0 |
| V3-P9-04 | V3.1+ 准备：业务模块迁移清单/需求登记/老机器保留策略 | ops | P1 | 0.5 | P9-03 | todo | 后续里程碑就绪 |
| V3-P9-05 | 公网安全评审：暴露面/权限/限流/密钥/管理入口/日志 | test | P0 | 0.5 | P9-01、P8-06 | todo | 无高危；整改闭环 |

### 任务汇总

| 阶段 | 任务数 | 人日 | 12.15 批次 |
|---|---:|---:|---|
| P0 | 3 | 2.5 | 是 |
| P1 | 6 | 7.0 | 是 |
| P2 | 6 | 6.5 | 是 |
| P3 | 6 | 8.0 | 是 |
| P4 | 6 | 7.0 | 是 |
| PM | 8 | 10.0 | 7 个（PM-07 后补） |
| SOP | 3 | 3.0 | 是 |
| P6 | 4 | 4.0 | 是 |
| P7 | 5 | 5.5 | 是 |
| P8 | 9 | 9.0 | 是 |
| P9 | 5 | 4.0 | 是 |
| **合计** | **61** | **66.5** | **60 个 / 65.5 人日** |

### 12.15 上线批次

| 批次 | 任务 | 说明 |
|---|---|---|
| **12.15 核心上线** | P0–P9（除 PM-07）、PM-01–06、PM-08、配置中心、多主机台账/SSH、维护模式、公网反代/证书、公网认证硬化、公网安全评审 | 基座 + 公共 + PM + 配置中心 + 多主机 + 维护 + 公网 |
| **12/31 前补齐** | PM-07 高级前端（风险矩阵/发布日历/甘特/报表） | 不阻塞核心上线 |
| **后续待定** | 1Panel API/数据/MCP/多实例/密钥入库；DB 迁移/备份；外部服务 | 单独立项/低优先级 |

---

## 4. 未来里程碑（不计入 V3.0）

| 里程碑 | 内容 | 优先级 | 前置 |
|---|---|---|---|
| V3.1 | 多主机完整运维 + 数据库迁移/备份（主机间、冷备份） | 低 | 容器迁移到新机器 |
| V3.2 | 外部服务：Ollama/Embedding/Browser/n8n/OpenClaw/LangBot/MCP/TTS | 低 | V3.1 稳定 |
| V3.3 | goodser 迁移（19 表，8–10d；含目标识别 / 相似商品检查） | 中 | V3.0 稳定 |
| V3.4 | ia 迁移（13 表，8–10d） | 中 | V3.3 稳定 |
| V3.5 | login3rd 迁移（2 表，3–4d） | 中 | V3.4 稳定 |
| V3.6 | shopchecker 迁移（3 表，3–4d） | 中 | V3.5 稳定 |
| V3.7 | 老机器下线评估 | 高 | 全部模块迁移 + 稳定 30 天 |

**V3.1 数据库迁移/备份范围：**

- 主机间迁移：`pg_dump/pg_restore`、`pg_basebackup`。
- 冷备份：停服务/只读 → 复制数据目录/快照 → 校验 → 启动。
- 备份策略/任务/产物/恢复；保留、加密、校验和。
- 表：`zzz_ops_backup_policy`、`zzz_ops_backup_job`、`zzz_ops_backup_artifact`、`zzz_ops_migration_job`。

每个业务模块迁移时：

1. 在 DM 建里程碑/需求/任务（含新增/修改需求）。
2. 从 V2 分支移植代码，按 PG 手册改造。
3. 新增模块迁移 + 数据字典。
4. PG 集成测试 + 业务回归 + 性能基线。
5. 冻结 V2 模块 → 快照 → 迁移 → 校验 → 路由切换。
6. 观察 7 天；V2 模块停用但数据保留。
7. 全部模块完成且 V3 稳定后，才评估老机器下线。

---

## 5. 依赖关系（导入用）

| 前置 | 后置 | 说明 |
|---|---|---|
| P0-03 | P1-01 | 规划确认后才动配置 |
| P1-02 | P1-03/P1-04 | Manager 先行 |
| P1-03 | P2-01 | 有 `*sql.DB` 才能建 Schema |
| P2-01 | P2-02..P2-06、P6-01、P7-01、PM-01 | Schema 是数据/测试/迁移/管控的地基 |
| P2-04 | P3-01、P7-03 | 迁移器先支持 PG |
| P3-01 | P3-02..P3-05 | codemod 先行 |
| P3-03 | P4-03、P4-04 | 基座 SQL 改造 |
| P3-04 | PM-02 | DM 仓库改造完成才能做 PM 后端 |
| P3-05 | P4-01、P4-02、P4-05 | Store 改造 |
| P3-06 | P7-02 | 方言清零后跑仓库集成测试 |
| P4-01 | P4-06 | 认证 Store 完成后再做公网权限硬化 |
| P4-05 | P8-01、P8-03 | 基础/公共模块完成 |
| P4-06、P1-05 | P8-06 | 公网认证 + 数据层就绪后配置反代/证书 |
| P1-01 | P1-06 | 配置基础就绪后做配置中心 |
| P3-02 | P8-09 | 查询/加密 helper 就绪后做主机台账 |
| P8-09 | P8-10 | 台账/SSH 后做 API/权限/审计 |
| P8-10、P1-06 | P8-11 | 主机/配置中心完成后做维护模式 |

| PM-01 | PM-02、PM-05 | PM 核心表先行 |
| PM-02 | PM-03、PM-04 | 核心 API 先行 |
| PM-05 | PM-06 | 高级表先行 |
| PM-06 | PM-07 | 高级 API 先行 |
| PM-03、PM-04 | PM-08 | 权限/核心前端完成后导入 |
| PM-06、PM-04 | PM-07 | 高级 API + 核心前端后做高级前端（12/31） |
| SOP-01 | SOP-02 | SOP 先行 |
| SOP-02 | SOP-03 | 上下文包后试点 |
| P6-04 | P9-01 | 演练通过才上线 |
| P8-05 | P9-01 | UAT 通过才上线 |
| P8-06 | P9-05 | 公网反代/证书就绪后做公网安全评审 |
| P9-01 | P9-05 | 上线后立即安全评审 |
| P9-02 | P9-03 | 观察稳定才发布 |
| P9-03 | P9-04 | 发布后准备后续迁移 |

---

## 6. 导入映射（developmentManager）

### 6.1 里程碑

| 本文件字段 | `milestone` 字段 |
|---|---|
| 项目 `go-backend-core` | `project_key` |
| `V3.0` | `version` |
| PostgreSQL 基座、公共能力与项目化管控 | `name` |
| 目标 | `goal` |
| planned | `status` |
| 预估/开始/目标日期 | `start_date` / `target_date` |
| Tom | `owner` |

### 6.2 需求

| 本文件字段 | `requirement` 字段 |
|---|---|
| RQM-V3-00x | `code` |
| 需求 | `title` |
| 验收标准 | `content` |
| feature/refactor/ops | `type` |
| P0–P2 | `priority` |
| 待核对/已确认 | `status` |
| 本文件链接 | `source_note` |

### 6.3 任务

| 本文件字段 | `task` 字段 |
|---|---|
| V3-Px-yy | `code` |
| 任务 | `title` |
| 验收/备注 | `description` |
| 类型 | `type` |
| 优先级 | `priority` |
| 预估 | `estimate_hours = 预估 × 8` |
| 依赖 | `task_dependency`（按任务编号解析） |
| 状态 | `status` |
| 负责人 | `owner` |
| 阶段 | `milestone_id`（V3.0） |

### 6.4 导入方式

1. **P0**：在 developmentManager seed 中登记 V3.0 里程碑/需求；任务以本文件为准。
2. **PM-03**：提供 `POST /api/v1/developmentmanager/projects/:id/import`，按 `(project_id, code)` 幂等 upsert。
3. **PM-08**：`scripts/dm-import-v3.sh` 调用导入 API；周报用 `kind=doc` 生成。

JSON 结构示例：

```json
{
  "milestone": {"version": "V3.0", "name": "PostgreSQL 基座、公共能力与项目化管控", "status": "planned"},
  "requirements": [
    {"code": "RQM-V3-001", "title": "PostgreSQL 单一运行时", "priority": "P0", "type": "refactor"}
  ],
  "tasks": [
    {"code": "V3-P0-01", "title": "冻结 V2 基线", "type": "ops", "priority": "P0", "estimate_hours": 8, "depends_on": []}
  ]
}
```

---

## 7. V3.0 期间工作流

```
需求登记(RQM) → 评审(approved) → 里程碑(V3.0) → 拆任务(V3-Px-yy)
      ↓                                            ↓
  需求版本快照                               任务依赖/预估/负责人
      ↓                                            ↓
 AI 实现(doing) ──提交引用任务编号──> 测试/评审(review) ──> 完成(done)
      ↓                                            ↓
   阻塞登记(blocked)                            任务日志 + 工时
      ↓                                            ↓
 周报/燃尽/里程碑进度 <──────────────────────── 资产/文档登记
```

### 7.1 每周节奏

| 时间 | 动作 | 负责人 | 产出 |
|---|---|---|---|
| 周一 10:00 | 周计划：任务、依赖、风险 | Tom | 周计划 |
| 每日 18:00 | 更新任务状态/工时/阻塞 | AI + Tom | 看板 |
| 周五 16:00 | 周复盘：完成率、逾期、阻塞、下周计划 | Tom | 周报 |
| 阶段末 | 验收：DoD、测试、文档、迁移校验 | Tom | 阶段验收记录 |

### 7.2 任务 DoD

- [ ] 代码 + 单元测试 + `golangci-lint` 通过。
- [ ] SQL 变更配 PG 集成测试或 golden SQL 测试。
- [ ] 表结构变更配迁移 + 数据字典 + 文档。
- [ ] API 变更配 Swagger + 模块文档。
- [ ] 提交信息引用任务编号。
- [ ] 人工评审通过并在任务日志留痕。
- [ ] AI 未越界修改业务模块（V3.0 禁止）。
- [ ] 涉及公网的功能：HTTPS/限流/权限/审计检查通过。
- [ ] 涉及部署的功能：反代/证书/健康检查/回滚点确认。

### 7.3 周报模板

```markdown
## V3 周报 W{{n}}（{{date}}）

### 本周完成
- V3-Px-yy 标题（人日）

### 进行中
- V3-Px-yy 标题（进度 %，预计完成日）

### 阻塞/风险
- 任务/风险 · 影响 · 需要谁决策

### 数据
- 里程碑进度：xx%
- 需求完成：x/12
- 阻塞：x 个（最长 x 天）
- 方言门禁：x
- AI 返工：x

### 下周计划
- ...
```

---

## 8. 变更记录

| 日期 | 变更 | 作者 |
|---|---|---|
| 2026-11-18 | 初版：V3 技术规划 + 57 任务台账 | Tom / AI |
| 2026-11-18 | v0.2：改为 V3.0 基座先行；业务模块留 V2；GORM 保留；完整项目管控；AI 开发；54 任务/58.5 人日 | Tom / AI |
| 2026-11-18 | v0.3：公网 OpenResty/HTTPS；容器 80；管理员授权；V2 冻结；12.15 上线；1Panel 调研；58 任务/61.5 人日 | Tom / AI |
| 2026-11-18 | v0.4：核心上线确认；1Panel 托管反代/证书（老机器）；管理员先创建用户；PM-07 12/31 补齐 | Tom / AI |
| 2026-11-18 | v0.5：新二级域名指向新机器；新机器 1Panel v2 本地直连；数据源/MCP 最小只读；59 任务/62.5 人日 | Tom / AI |
| 2026-11-18 | v0.6：1Panel 2.5.1 API（HMAC/MD5、只读模块）落档；云服务器联调 401 待放行 | Tom / AI |
| 2026-11-18 | v0.7：多实例/API Key 加密入库；按 1Panel-skills 只读接入；60 任务/63.5 人日 | Tom / AI |
| 2026-11-18 | v0.8：1Panel 接入暂停；V3.0 回归 PG 基座+公共+PM；57 任务/61.0 人日 | Tom / AI |
| 2026-11-18 | v0.9：新增多主机管理、配置中心；DB 迁移/备份与外部服务列后续；60 任务/65.0 人日 | Tom / AI |
| 2026-11-19 | v1.0：L1/L2/ACloud 节点矩阵；手动维护模式；RustFS 固定在 ACloud；61 任务/66.5 人日 | Tom / AI |
