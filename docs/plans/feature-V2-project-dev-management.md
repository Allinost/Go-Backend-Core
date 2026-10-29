# Feature-V2 规划：项目开发管理（PDM, Project Development Management）

> 分支：`feature-V2`（基于 `feature-goodser` @ 22b5c23）
> 状态：**草稿，待核对订正后启动开发**
> 关联仓库：`Go-Backend-Core`（后端基座，本仓库）、`Go-Frontend-Core`（NKUI 前端基座）、`NKUI`（前端模板）

---

## 1. 目标与定位

在 Go-Backend-Core 基座上新增一个业务模块，实现对"项目开发"全生命周期的可视化管理，覆盖：

1. **项目台账**：基座 / 模板 / 模块三类项目的登记、关联、归类管理；
2. **部署资产**：容器、端口、反代、内外网、代理网访问地址、资源分配、卷映射、备份情况；
3. **数据字典**：项目数据表的用途、字段含义；
4. **需求管理**：可视化的需求增 / 改 / 更新 / 删（含版本留痕），并可按流程生成开发方案、代码、文档；
5. **归类管理**：需求、代码、文档、资产按项目归档，项目删除时软删除并标记关联为已删除。

本模块本身也是"一个项目"的实践者：属于 `zzz/` 业务模块，关联 `Go-Backend-Core` 基座项目；前端部分属于 `Go-Frontend-Core` 的一个模块，关联 NKUI 模板 + 本后端模块。

---

## 2. 核心概念模型

### 2.1 项目类型（project.type）

| 类型 | 说明 | 示例 |
|---|---|---|
| `base` 后端基座 | 通用后端骨架项目 | Go-Backend-Core |
| `template` 前端模板 | 前端模板项目 | NKUI |
| `backend_module` 后端模块 | 绑定一个后端基座 | zzz/ia、zzz/login3rd、**pdm（本模块）** |
| `frontend_module` 前端模块 | 绑定一个前端模板 **且** 绑定一个后端模块 | Go-Frontend-Core 下的 pdm 管理端页面 |

### 2.2 关联关系（pdm_project_relation）

- `uses_base`：模块 → 基座
- `uses_template`：前端模块 → 前端模板
- `binds_backend_module`：前端模块 → 后端模块
- `related`：其他横向关联（如基座 ↔ 基座、模板 ↔ 模板）

规则：
- 基座/模板可有多个（后期扩展），故关联为**多对多 + 类型**；
- 关联全部软删除：删除项目时仅置 `deleted`，其发出与收到的关联同步标记 `deleted`，保留可恢复与审计能力；
- 删除项目前若存在"被依赖"的启用中关联，需二次确认并提示依赖方。

### 2.3 目录归类约定（需求/代码/文档/资产）

仓库内按项目归档，DB 中登记路径索引（`pdm_project_asset`）：

```
docs/
├── projects/
│   ├── go-backend-core/          # 基座项目自身的文档
│   │   ├── requirements/
│   │   ├── architecture.md ...
│   ├── nkui/                     # 前端模板项目
│   ├── pdm/                      # 本模块（zzz/pdm 后端部分）
│   │   ├── requirements/
│   │   ├── design/
│   │   └── api/
│   └── go-frontend-core/
└── modules/<module>/...          # 现有 docs/modules/* 迁移映射为项目内文档
```

- 代码类资产记录 `repo + branch + path`，不复制代码；
- 文件类资产（附件、截图、导出包）走 `services/fileref` + `services/storage`；
- 现有 `docs/modules/*` 在 P1 阶段做一次映射迁移（不删除旧目录，只登记索引，避免破坏现有 Pages）。

---

## 3. 数据模型（DDL 草案）

> 表前缀 `pdm_`；建表时同步写明**表用途、每个字段的含义**（本模块自身满足 1.2 的规范要求）。
> 建表方式待确认（见 §8-Q2）：倾向沿用 `zzz/*` 的 `initTables()` 幂等 DDL。

### 3.1 项目与关联

**pdm_project — 项目台账**
| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| project_key | varchar(64) UK | 项目唯一键（目录/接口引用用，如 `go-backend-core`） |
| name | varchar(128) | 项目名称 |
| type | varchar(32) | 项目类型：base/template/backend_module/frontend_module |
| description | text | 项目简介 |
| repo_url | varchar(255) | 代码仓库地址（可空） |
| repo_path | varchar(255) | 本地路径（可空） |
| default_branch | varchar(64) | 默认分支 |
| tech_stack | json | 技术栈摘要（语言/框架/DB），仅作展示 |
| status | varchar(16) | active/archived |
| deleted | tinyint(1) | 软删除标记 |
| deleted_at / created_by / updated_by / created_at / updated_at | — | 审计字段 |

**pdm_project_relation — 项目关联**
| 字段 | 含义 |
|---|---|
| id, from_project_id, to_project_id | 关联两端（均为 pdm_project.id） |
| relation_type | uses_base / uses_template / binds_backend_module / related |
| note | 关联说明 |
| deleted, deleted_at | 软删除标记（删除项目时同步置位） |
| UK(from,to,type) | 防重复关联 |

### 3.2 部署资产（1.2）

**pdm_container — 项目关联容器（一个项目可多个）**
`id, project_id, container_name, image, compose_ref(docker-compose 文件/服务名), replicas, network_mode, env_notes(json), status_note, deleted`

**pdm_container_port — 端口**
`id, container_id, listen_port(容器内端口), host_port(宿主端口，可空=不映射), protocol(tcp/udp), purpose(用途), exposure(none/intranet/extranet)`

**pdm_reverse_proxy — 反向代理**
`id, project_id, container_id(可空), proxy_type(nginx/caddy/traefik/other), domain, upstream_url, listen_port, tls(0/1), notes`

**pdm_proxy_access — 代理网访问地址（留空即不可达，可多个）**
`id, project_id, container_id(可空), name, network_zone(intranet/extranet/proxy_net), url(可空 → 不可达), reachable(tinyint，与 url 一致性由服务层保证), notes`
> 语义：`url IS NULL` ⇒ 该地址不可达，前端显示"不可达"。

**pdm_proxy_access_allow — 代理地址允许访问的容器（多对多）**
`id, access_id, container_id, note` — 满足"可能有多个容器允许访问"。

**pdm_resource_alloc — 资源分配（按容器）**
`id, container_id, cpu_limit, cpu_shares, mem_limit, mem_reservation, gpu_note, storage_quota, notes`

**pdm_volume_mapping — 卷映射**
`id, container_id, host_path, container_path, mode(rw/ro), purpose(用途), in_backup(是否纳入备份)`

**pdm_backup — 备份情况**
`id, project_id, container_id(可空), volume_id(可空), backup_type(full/db/volume), strategy_cron, destination, retention_days, last_run_at, last_status(success/failed/running), last_error, restore_note, enabled`

### 3.3 数据表字典（1.2 末项）

**pdm_db_table — 数据表登记**
`id, project_id, db_type(mysql/postgres), table_schema, table_name, purpose(表用途), status(active/deprecated/dropped), module_name(归属模块), created_note`

**pdm_db_column — 字段字典**
`id, table_id, column_name, data_type, nullable, default_value, meaning(字段含义), example, is_pk, is_index, note`

> P3 提供两种录入方式：手动维护 + "扫描库结构导入初稿"（读 `information_schema`，人工补 `meaning/purpose`）。

### 3.4 需求管理（1.5）

**pdm_requirement — 需求**
`id, project_id, code(RQM-xxxx 项目内编号), title, content(markdown), type(feature/change/fix/ops), priority(P0-P3), status(draft/reviewing/approved/planned/developing/done/closed), source(人工/AI/客户), owner, deleted, timestamps`

**pdm_requirement_version — 需求修订与删除留痕**
`id, requirement_id, version_no, title, content, status, change_note(本次变更说明), operator, created_at`
> 增=初始版本；改=新增版本；删=软删除 + 生成"已删除"快照版本，保证可回溯。

**pdm_generation — 生成任务（方案/代码/文档）**
`id, requirement_id, project_id, kind(plan/code/doc), engine(ai/template/manual), status(pending/running/success/failed), input_snapshot(json), output_ref(文档路径/分支+commit/PR 链接), error, operator, started_at, finished_at`

**pdm_project_asset — 归类资产索引**
`id, project_id, category(requirement/code/doc/asset), title, uri(仓库相对路径或 fileref id), format, version, size, checksum, note, deleted`

---

## 4. 后端模块设计（`zzz/pdm`）

严格沿用 `zzz/*` 分层模式：

```
zzz/pdm/
├── module.go        # 实现 modules.Module，Name()="pdm"，Init 中 initTables() + 注册调度任务
├── handler.go       # Gin handlers + swag 注解（路由挂载 /api/v1/pdm/*）
├── service.go       # 业务编排（软删除级联、关联校验、生成流程）
├── repository.go    # 数据访问
├── model.go         # 上述实体
├── errors.go        # 模块错误
├── generation/      # 生成流程：plan/code/doc 三种生成器（P5）
│   ├── generator.go
│   ├── plan.go      # 方案生成（模板 + 可选 AI）
│   ├── doc.go       # 文档生成（写入 docs/projects/<key>/）
│   └── code.go      # 代码生成（产出分支/patch 草案，不直接落主干）
├── scan/            # 数据字典扫描导入（P3，读 information_schema）
└── seed/            # 初始数据：把现有项目（基座/模板/zzz 模块）登记进系统
```

要点：
- **注册**：`cmd/server/main.go` 增加 `modules.Register(&pdm.Module{})`；
- **DB**：`database.DB.MySQL["main"]`（与现有 zzz 模块一致；Pg 适配见 §8-Q6）；
- **权限**：复用 `middleware` 的 auth/rbac；新增权限点 `pdm:read / pdm:write / pdm:admin`（删除、生成任务需要 admin）；
- **审计**：写操作接 `services/audit`；
- **调度**：备份状态刷新、生成任务重试用 `modules.Get("scheduler")` 注册；
- **生成引擎**：优先 `services/ai`（OpenAI/Ollama Provider 已具备），不可用时降级为模板生成；
- **关联项目读取**：只读登记 Go-Frontend-Core / NKUI，不扫描其文件系统（跨仓库内容通过资产索引 uri 记录）。

### API 概要（前缀 `/api/v1/pdm`）

```
GET|POST        /projects                      列表(筛选 type/status/deleted)、新建
GET|PUT|DELETE  /projects/:id                  详情、更新、软删除（级联标记关联）
GET|POST        /projects/:id/relations        关联列表、新增关联
GET|POST        /projects/:id/containers       容器
POST            /containers/:id/ports|proxy-access|resources|volumes
GET|POST        /projects/:id/proxy-access     代理访问地址（含 allow 容器）
GET|POST        /projects/:id/backups          备份情况
GET|POST        /projects/:id/tables           数据表字典（含 columns 子资源）
POST            /tables/:id/scan-import        从库扫描导入初稿
GET|POST|PUT    /projects/:id/requirements     需求 CRUD（PUT 自动产生 version）
DELETE          /requirements/:id              软删除 + 快照版本
GET             /requirements/:id/versions     修订历史
POST            /requirements/:id/generations  触发生成 plan/code/doc
GET             /generations/:id               任务状态与产物引用
GET|POST        /projects/:id/assets           归类资产索引
GET             /stats                         仪表盘统计
```

---

## 5. 前端规划（Go-Frontend-Core 模块）

在 `Go-Frontend-Core`（NKUI 模板基座）中新增 `pdm` 管理模块，绑定 NKUI 模板 + 后端 `pdm` 模块（本模块即 1.3 中"前端模块绑定模板与后端模块"的首个实例）：

- **项目列表页**：按类型分组的卡片/表格，支持筛选、软删除恢复；
- **项目详情页 Tab**：
  1. 概览（基本信息、关联项目关系图）
  2. 部署（容器 / 端口 / 反代 / 代理访问（不可达置灰） / 资源 / 卷 / 备份）
  3. 数据字典（表 → 字段级联表格，表用途、字段含义列）
  4. 需求（看板 + 列表，增改更新删、版本历史抽屉、状态流转）
  5. 生成（触发方案/代码/文档生成，任务进度与产物链接）
  6. 资产（文档/代码/附件索引）
- **需求编辑器**：Markdown 编辑 + 版本对比（基于 versions）。
- 端：先 Web；移动端/小程序复用同一 API，P6 不做（见 §8-Q7）。

---

## 6. 实施阶段（每个阶段可独立验收）

| 阶段 | 内容 | 验收标准 |
|---|---|---|
| **P0 规划核对** | 本文件 + 用户订正 | 用户确认规划（本阶段） |
| **P1 骨架 + 项目台账** | `zzz/pdm` 模块骨架、pdm_project/relation/asset 表、CRUD、软删除级联、seed 登记现有项目、`main.go` 注册 | 接口可建/删项目与关联，删除后关联带 deleted 标记 |
| **P2 部署资产** | container/port/proxy/proxy_access(+allow)/resource/volume/backup 七组实体与 API | 一个项目可完整录入 1.2 所列全部部署信息 |
| **P3 数据字典** | db_table/db_column + 扫描导入 + 手工维护 | 表用途、字段含义可录入并展示 |
| **P4 需求管理** | requirement/version、状态流转、软删除快照 | 增改更新删 + 版本历史全部可回溯 |
| **P5 生成流程** | generation 任务、plan/doc 模板生成 → 接入 services/ai；code 产出分支草案 | 由需求一键生成方案与文档，失败可重试 |
| **P6 前端模块** | Go-Frontend-Core 中 pdm 模块（Web） | 全流程可视化操作走通 |
| **P7 收尾** | Swagger(`make swagger`)、docs/projects 归档、测试、发布说明 | 通过 golangci-lint + 单测，文档齐全 |

依赖关系：P1 → P2/P3/P4（可并行）→ P5 → P6 → P7。

---

## 7. 风险与原则

- **不动现有基座行为**：仅新增 `zzz/pdm` 与 `main.go` 一行注册，不改既有模块；
- **软删除贯穿**：所有实体默认软删除，关联同步标记，避免误删破坏台账；
- **生成不直落主干**：代码生成产出分支/patch 草案，人工评审后合并；
- **数据字典渐进**：先扫描导入结构，含义字段由人补全，不追求自动推断；
- **前端仓库独立**：Go-Frontend-Core 单独分支与提交，不与本仓库混提。

---

## 8. 待确认问题（请核对订正）

- **Q1 模块命名**：后端模块名定为 `pdm`（路由 `/api/v1/pdm`，目录 `zzz/pdm`）？还是用 `projdev` / `devmgmt`？
- **Q2 建表方式**：沿用 `zzz/*` 的 `initTables()` 幂等 DDL（与现有一致），还是纳入 `migrations/mysql|postgres/NNN_*.sql` 统一迁移（本模块表多，倾向后者或两者结合）？
- **Q3 生成引擎**：P5 是否直接接 `services/ai`（OpenAI/Ollama），还是第一版只做"模板化生成"（人工填参数 → 产出方案/文档骨架），AI 后置？
- **Q4 代码生成边界**：`kind=code` 的产物形态——(a) 仅生成方案级代码骨架文档、(b) 产出 patch 文件、(c) 自动开分支提交？建议 (b) 起步。
- **Q5 前端范围**：P6 只做 Web 端（Go-Frontend-Core），移动/小程序后续再用同一 API 接入，是否同意？
- **Q6 数据库**：pdm 表默认建在 `mysql.main`（与现有 zzz 一致）；若要同时支持 pg，需明确适配策略。
- **Q7 代理访问语义**：`url` 留空 = 不可达，"允许多个容器访问"= 每条访问地址挂一组允许的容器 —— 是否与预期一致？
- **Q8 现有文档迁移**：是否将 `docs/modules/*` 登记进 `pdm_project_asset`（只建索引不移动文件），还是物理迁移到 `docs/projects/<key>/`？
- **Q9 权限粒度**：是否复用现有 rbac 中间件，仅新增 `pdm:read/write/admin` 三个权限点？
