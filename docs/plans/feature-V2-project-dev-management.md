# Feature-V2 规划：项目开发管理（developmentManager）

> 分支：`feature-V2`（基于 `feature-goodser` @ 22b5c23）
> 状态：**已核对完毕（全部决策见 §0），前缀规范已落档 `docs/data-model.md`**
> 关联仓库：`Go-Backend-Core`（后端基座，本仓库）、`Go-Frontend-Core`（NKUI 前端基座）、`NKUI`（前端模板）

## 0. 已确认决策（2026-10-29 第一轮核对）

| # | 议题 | 决策 |
|---|---|---|
| Q1 | 模块命名 | **`developmentManager`**：目录 `zzz/developmentManager`，Go 包名小写 `developmentmanager`，路由前缀见 Q1a |
| Q2 | 建表方式 | **统一走 `migrations/` 迁移**（由 migrate 模块管理版本/checksum，DDL 方言见 Q6）；**并将以前模块的 `initTables()` 陆续改造为规范迁移**（列入 P8 见 §6） |
| Q3 | 生成引擎 | 第一版**模板/手工生成为主线**；**AI 作为可选项接入末尾**，`engine=ai\|template\|manual` 可选，内建 AI 不可用时任务仍能用其他方式完成（自动降级模板引擎） |
| Q4 | 表前缀 | **`zzz_development_manager_`**，与现有 `zzz_ia_*`、`zzz_shopchecker_*`、`zzz_goodser_*` 规范一致，不使用 `pdm` |
| Q1a | 命名细节 | 目录 `zzz/developmentManager`（驼峰），Go 包名小写 `developmentmanager`，**路由前缀全小写 `/api/v1/developmentmanager`** |
| Q5 | 代码生成边界 | 产物为 patch/分支草案；**生成请求提供 `auto_commit` 开关**，允许配置是否自动提交（默认关闭；开启时提交到特性分支，仍不落主干） |
| Q6 | 数据库 | **仅 MySQL**：以前的模块全部是 mysql、没有 pg；本模块沿用现状，**DDL 只写 `migrations/mysql/NNN_*.sql`（MySQL 方言）**，连接池用 `database.DB.MySQL["main"]`；pg 支持不在本期范围（基座的 `postgres.nas` 属 NAS 用途，与业务模块无关），日后确有需要再补 pg 方言 |
| Q7 | 前端范围 | **先制作 Web**（Go-Frontend-Core）；小程序/移动端**后续等排期**，复用同一 API |
| Q8 | 现有文档与 Pages | **只记录索引、文件不动**（对 GitHub Pages 100% 兼容，旧链接不失效）；新增文档直接写入 `docs/projects/<key>/` |
| Q9 | 权限 | 现有 rbac 的 read/write/admin 三点**不够**，扩展为**资源 × 动作权限矩阵**（草案见 §4.1，已按草案执行） |
| Q9a | 角色与权限管理 | **允许动态增加角色与权限，并允许 admin 分配**：直接复用基座 auth 模块现成的 `/api/v1/auth/admin/*`（权限 CRUD、角色 CRUD、`roles/:name/permissions` 配置、`users/:id/roles` 分配），**不另造管理接口**；§4.1 的 admin/editor/viewer 与权限点仅作**初始幂等播种**，可增删改 |
| — | 前缀规范落档 | 已写入 `docs/data-model.md` §命名规范（基座独立库、基座表无前缀、模块表带模块前缀 `zzz_*`） |
| Q10 | 部署建模 | 采用 **A 行式**，但**端口、反代、访问地址合并为 `access_point` 一张表**（绑定链，行粒度 = 端口 × 地址，仍可 SQL 过滤）；资源分配保持**独立 `resource_alloc` 表**（1:1）；白名单建模见 Q11 |
| Q11 | 白名单建模 | 白名单**升格为独立可复用实体**：`whitelist` 组表 + `whitelist_member` 成员行表（`container_id` FK，容器改名自动跟随）；`access_point` 只留 `whitelist_id` 组引用（**NULL = 不限制**），临时/单次名单建一次性组（**只留组引用，无内嵌 JSON 列**）。**允许白名单为空**：接入点可不引用组，组可 0 成员。部署 7 表，全模块 **15 张表** |

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
| `backend_module` 后端模块 | 绑定一个后端基座 | zzz/ia、zzz/login3rd、**developmentManager（本模块）** |
| `frontend_module` 前端模块 | 绑定一个前端模板 **且** 绑定一个后端模块 | Go-Frontend-Core 下的 developmentManager 管理端页面 |

### 2.2 关联关系（zzz_development_manager_project_relation）

- `uses_base`：模块 → 基座
- `uses_template`：前端模块 → 前端模板
- `binds_backend_module`：前端模块 → 后端模块
- `related`：其他横向关联（如基座 ↔ 基座、模板 ↔ 模板）

规则：
- 基座/模板可有多个（后期扩展），故关联为**多对多 + 类型**；
- 关联全部软删除：删除项目时仅置 `deleted`，其发出与收到的关联同步标记 `deleted`，保留可恢复与审计能力；
- 删除项目前若存在"被依赖"的启用中关联，需二次确认并提示依赖方。

### 2.3 目录归类约定（需求/代码/文档/资产）

仓库内按项目归档，DB 中登记路径索引（`zzz_development_manager_project_asset`）：

```
docs/
├── projects/
│   ├── go-backend-core/          # 基座项目自身的文档
│   │   ├── requirements/
│   │   ├── architecture.md ...
│   ├── nkui/                     # 前端模板项目
│   ├── development-manager/      # 本模块（zzz/developmentManager 后端部分）
│   │   ├── requirements/
│   │   ├── design/
│   │   └── api/
│   └── go-frontend-core/
└── modules/<module>/...          # 现有 docs/modules/* 迁移映射为项目内文档
```

- 代码类资产记录 `repo + branch + path`，不复制代码；
- 文件类资产（附件、截图、导出包）走 `services/fileref` + `services/storage`；
- 现有 `docs/modules/*` **只登记索引、文件不动**（§0-Q8，对 GitHub Pages 100% 兼容）；新文档一律写入 `docs/projects/<key>/`。

---

## 3. 数据模型（DDL 草案）

> 表前缀 **`zzz_development_manager_`**（对齐现有 `zzz_ia_*` 规范，见 §0-Q4；规范已落档 `docs/data-model.md`）；
> 建表时同步写明**表用途、每个字段的含义**（本模块自身满足 1.2 的规范要求）。
> 建表方式：**统一走 `migrations/mysql/NNN_*.sql`，MySQL 方言**（§0-Q2、§0-Q6：现状所有模块均为 MySQL、无 pg），由 migrate 模块统一执行。本节各表即迁移文件的内容草案。

### 3.1 项目与关联

**zzz_development_manager_project — 项目台账（登记一个受管项目的基本信息与仓库位置）**
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

**zzz_development_manager_project_relation — 项目关联（记录项目间基座/模板/模块绑定关系）**
| 字段 | 含义 |
|---|---|
| id, from_project_id, to_project_id | 关联两端（均为 zzz_development_manager_project.id） |
| relation_type | uses_base / uses_template / binds_backend_module / related |
| note | 关联说明 |
| deleted, deleted_at | 软删除标记（删除项目时同步置位） |
| UK(from,to,type) | 防重复关联 |

### 3.2 部署资产（1.2）（定稿：行式建模 + 网络四合一 + 白名单组复用，§0-Q10/Q11）

> 原则：**端口、反代、访问地址是一条绑定链**，合并为 `access_point` 一张表（行粒度 = 端口 × 访问地址），一行看全"容器端口 → 反代 → 暴露地址"，同时保留 SQL 过滤能力；**白名单升格为可复用的独立实体**（组 + 成员两表），接入点通过主键引用组；其余按行式独立建表。

**zzz_development_manager_container — 项目关联容器（一个项目可关联多个容器）**
`id, project_id, container_name(项目内唯一键), image, compose_ref(docker-compose 文件/服务名), replicas, network_mode, env_notes(json), status_note, deleted`

**zzz_development_manager_resource_alloc — 资源分配（容器 1:1，独立表）**
`id, container_id UK, cpu_limit, cpu_shares, mem_limit, mem_reservation, gpu_note, storage_quota, notes`

**zzz_development_manager_volume_mapping — 卷映射（记录宿主目录与容器目录挂载关系）**
`id, container_id, host_path, container_path, mode(rw/ro), purpose(用途), in_backup(是否纳入备份), deleted`

**zzz_development_manager_access_point — 网络接入点（端口 + 反代 + 代理访问地址 + 容器白名单 四合一）**
| 段 | 字段 | 含义 |
|---|---|---|
| 宿主 | `id, project_id, container_id, remark, deleted` + 审计字段 | 归属项目与宿主容器 |
| 端口 | `listen_port, host_port(空=不映射), protocol(tcp/udp), purpose, exposure(none/intranet/extranet)` | 端口映射与内外网暴露 |
| 反代 | `proxy_type(nginx/caddy/traefik/other), domain, upstream_url, tls` | **可空 = 无反代直连** |
| 地址 | `network_zone(intranet/extranet/proxy_net), url, reachable, notes` | **`url IS NULL` ⇒ 不可达**，前端置灰 |
| 白名单 | `whitelist_id FK → whitelist（可空）` | **引用常用白名单组；`NULL` = 该地址不设白名单（不限制）**；**允许为空**（可不引用组）；引用空组 = 当前无任何容器放行（登记语义）；临时名单建一次性组（存储路径唯一，§0-Q11） |

- 行粒度 = **端口 × 访问地址**：同一端口有内网+外网两个地址 → 2 行；多端口共用同一反代域名 → 多行按 `domain` 归组；地址未分配 → 照样 1 行且 `url = NULL`；
- 典型查询仍是 SQL：`url IS NULL` 查不可达、`exposure='extranet'` 查外网暴露、按 `domain` 分组查反代、`whitelist_id` 关联查"哪些地址用了某白名单组"。

**zzz_development_manager_whitelist — 常用白名单组（可复用、可命名，被多条接入点引用）**
`id, project_id, name(项目内唯一组名，如 "ia-market-allow"), note, deleted, 审计字段`
> **允许白名单为空**：组可以 0 成员（先建组占名、后补成员），服务层不做"组必须有成员"校验。

**zzz_development_manager_whitelist_member — 白名单成员（组 → 允许访问的容器，行式 + FK）**
`id, whitelist_id, container_id FK → container, note`
> 容器改名自动跟随（FK）；容器软删除时服务层提示"仍被白名单组 X 引用"。

**zzz_development_manager_backup — 备份情况（记录备份策略与最近执行结果）**
`id, project_id, container_id(可空), volume_id(可空), backup_type(full/db/volume), strategy_cron, destination, retention_days, last_run_at, last_status(success/failed/running), last_error, restore_note, enabled`

### 3.3 数据表字典（1.2 末项）

**zzz_development_manager_db_table — 数据表登记（记录项目数据表及其用途）**
`id, project_id, db_type(mysql/postgres), table_schema, table_name, purpose(表用途), status(active/deprecated/dropped), module_name(归属模块), created_note`

**zzz_development_manager_db_column — 字段字典（记录表内字段的含义）**
`id, table_id, column_name, data_type, nullable, default_value, meaning(字段含义), example, is_pk, is_index, note`

> P3 提供两种录入方式：手动维护 + "扫描库结构导入初稿"（读 `information_schema`，人工补 `meaning/purpose`）。

### 3.4 需求管理（1.5）

**zzz_development_manager_requirement — 需求（项目级需求登记与状态流转）**
`id, project_id, code(RQM-xxxx 项目内编号), title, content(markdown), type(feature/change/fix/ops), priority(P0-P3), status(draft/reviewing/approved/planned/developing/done/closed), source(人工/AI/客户), owner, deleted, timestamps`

**zzz_development_manager_requirement_version — 需求修订与删除留痕（每次增改删都产生版本快照）**
`id, requirement_id, version_no, title, content, status, change_note(本次变更说明), operator, created_at`
> 增=初始版本；改=新增版本；删=软删除 + 生成"已删除"快照版本，保证可回溯。

**zzz_development_manager_generation — 生成任务（按需求生成方案/代码/文档的任务记录）**
`id, requirement_id, project_id, kind(plan/code/doc), engine(ai/template/manual), auto_commit(tinyint 是否自动提交产物，默认 0 关；仅对 kind=code 生效), status(pending/running/success/failed), input_snapshot(json), output_ref(文档路径/分支+commit/PR 链接), error, operator, started_at, finished_at`

**zzz_development_manager_project_asset — 归类资产索引（需求/代码/文档/资产按项目归档的索引）**
`id, project_id, category(requirement/code/doc/asset), title, uri(仓库相对路径或 fileref id), format, version, size, checksum, note, deleted`

---

## 4. 后端模块设计（`zzz/developmentManager`）

严格沿用 `zzz/*` 分层模式：

```
zzz/developmentManager/            # 包名 developmentmanager；路由前缀 /api/v1/developmentmanager（全小写）
├── module.go        # 实现 modules.Module，Name()="developmentmanager"，Init 中初始化 repo/服务 + 注册调度任务
├── handler.go       # Gin handlers + swag 注解（路由挂载 /api/v1/developmentmanager/*）
├── service.go       # 业务编排（软删除级联、关联校验、生成流程）
├── repository.go    # 数据访问
├── model.go         # 上述实体
├── errors.go        # 模块错误
├── generation/      # 生成流程：plan/code/doc 三种生成器（P5）
│   ├── generator.go # 统一入口：engine 选择 + AI 不可用时降级 template
│   ├── plan.go      # 方案生成（模板先行）
│   ├── doc.go       # 文档生成（写入 docs/projects/<key>/）
│   └── code.go      # 代码生成（产出 patch/分支草案，不直接落主干）
├── scan/            # 数据字典扫描导入（P3，读 information_schema）
└── seed/            # 初始数据：把现有项目（基座/模板/zzz 模块）登记进系统
```

要点：
- **注册**：`cmd/server/main.go` 增加 `modules.Register(&developmentmanager.Module{})`；
- **建表**：模块自身**不建表**，DDL 落在 `migrations/mysql/NNN_*.sql`（MySQL 方言，§0-Q6），由 migrate 模块统一执行（§0-Q2）；
- **DB**：`database.DB.MySQL["main"]`，与现有 zzz 模块一致，SQL 按 MySQL 方言编写（§0-Q6，仅 MySQL）；
- **权限**：复用 `middleware.RequirePermission(rbac, resource, action)`（资源 × 动作），三点式 read/write/admin 不够用，扩展为**权限矩阵**（见 §4.1）；
- **审计**：写操作接 `services/audit`；
- **调度**：备份状态刷新、生成任务重试用 `modules.Get("scheduler")` 注册；
- **生成引擎**：主线为模板/手工生成；`engine=ai` 时走 `services/ai`（OpenAI/Ollama Provider 已具备），AI 不可用则任务自动降级为模板引擎完成（§0-Q3）；
- **关联项目读取**：只读登记 Go-Frontend-Core / NKUI，不扫描其文件系统（跨仓库内容通过资产索引 uri 记录）。

### 4.1 权限矩阵（§0-Q9/Q9a：资源 × 动作扩展；角色与权限**动态可增、admin 可分配**）

挂载方式：`middleware.RequirePermission(rbac, resource, action)`，action 统一用 `read / create / update / delete`，特例动作单列。

| 资源 resource | 动作 action | 说明 |
|---|---|---|
| `dm.project` | read / create / update / **delete** / restore | 项目台账；delete 级联标记关联，restore 恢复软删除 |
| `dm.relation` | read / create / delete | 项目关联的增删与查看 |
| `dm.deployment` | read / create / update / delete | 部署资产全域：容器、网络接入点（端口+反代+地址，白名单经组引用）、白名单组、资源、卷、备份 |
| `dm.dictionary` | read / create / update / delete / **scan** | 数据表字典；scan = 触发从库扫描导入 |
| `dm.requirement` | read / create / update / delete / **approve** | 需求；approve = 状态流转/评审通过（draft→approved 等） |
| `dm.generation` | read / execute / **autocommit** | 生成任务；execute = 触发生成；**autocommit = 允许自动提交产物（与 Q5 开关配套的独立权限）** |
| `dm.asset` | read / create / delete | 归类资产索引 |

- **权限点播种**：模块 `Init()` 时按上表批量调用 `rbac.CreatePermission(name, resource, action)` **幂等播种**（重复启动不重复插入）；
- **角色与权限动态管理（§0-Q9a）**：
  - 允许**新增角色、新增权限**，由 **admin 通过基座现成接口分配与维护**，本模块不另造管理 API：
    - 权限：`GET|POST /api/v1/auth/admin/permissions`、`PUT|DELETE .../permissions/:id`
    - 角色：`GET|POST /api/v1/auth/admin/roles`、`PUT|DELETE .../roles/:name`
    - 角色 ↔ 权限：`GET|PUT /api/v1/auth/admin/roles/:name/permissions`
    - 用户 ↔ 角色：`POST /api/v1/auth/admin/roles/assign`、`GET|PUT|DELETE .../users/:id/roles`
  - 本模块额外提供只读元数据接口 `GET /api/v1/developmentmanager/permissions/meta`，列出本模块全部权限点（resource/action/中文说明），供前端权限管理页展示与勾选分配；
- **默认角色映射（仅初始播种，可改可增）**：`admin` → 全部（含 delete/approve/autocommit/scan）；`editor` → 各资源 read/create/update + `dm.requirement:approve`；`viewer` → 各资源 read；
- 敏感动作双保险：`dm.generation:autocommit` 权限与请求体 `auto_commit` 开关**同时满足**才会自动提交。

### API 概要（前缀 `/api/v1/developmentmanager`）

```
GET|POST        /projects                      列表(筛选 type/status/deleted)、新建
GET|PUT|DELETE  /projects/:id                  详情、更新、软删除（级联标记关联）
GET|POST        /projects/:id/relations        关联列表、新增关联
GET|POST        /projects/:id/containers       容器
POST            /containers/:id/resources|volumes
GET|POST|PUT|DELETE /projects/:id/access-points 网络接入点（端口+反代+地址绑定链；whitelist_id 引用白名单组）
GET|POST|PUT|DELETE /projects/:id/whitelists   常用白名单组 + 成员（组可复用、允许空组，被多条接入点引用）
GET|POST        /projects/:id/backups          备份情况
GET|POST        /projects/:id/tables           数据表字典（含 columns 子资源）
POST            /tables/:id/scan-import        从库扫描导入初稿
GET|POST|PUT    /projects/:id/requirements     需求 CRUD（PUT 自动产生 version）
DELETE          /requirements/:id              软删除 + 快照版本
GET             /requirements/:id/versions     修订历史
POST            /requirements/:id/generations  触发生成，body={kind: plan|code|doc, engine: ai|template|manual, auto_commit: 0|1}
GET             /generations/:id               任务状态与产物引用
GET|POST        /projects/:id/assets           归类资产索引
GET             /stats                         仪表盘统计
GET             /permissions/meta              本模块权限点元数据（供 admin 分配，只读）
```

---

## 5. 前端规划（Go-Frontend-Core 模块）

在 `Go-Frontend-Core`（NKUI 模板基座）中新增 `developmentManager` 管理模块，绑定 NKUI 模板 + 后端 `developmentManager` 模块（本模块即 1.3 中"前端模块绑定模板与后端模块"的首个实例）：

- **项目列表页**：按类型分组的卡片/表格，支持筛选、软删除恢复；
- **项目详情页 Tab**：
  1. 概览（基本信息、关联项目关系图）
  2. 部署（容器 / 网络接入点：端口+反代+访问地址，`url` 空置灰显示"不可达"，白名单经组引用 / 白名单组管理 / 资源 / 卷 / 备份）
  3. 数据字典（表 → 字段级联表格，表用途、字段含义列）
  4. 需求（看板 + 列表，增改更新删、版本历史抽屉、状态流转）
  5. 生成（触发方案/代码/文档生成，任务进度与产物链接，engine 可选 ai/template/manual）
  6. 资产（文档/代码/附件索引）
- **需求编辑器**：Markdown 编辑 + 版本对比（基于 versions）。
- 端：**先制作 Web**（§0-Q7）；小程序/移动端后续等排期，届时复用同一 API。

---

## 6. 实施阶段（每个阶段可独立验收）

| 阶段 | 内容 | 验收标准 |
|---|---|---|
| **P0 规划核对** | 本文件 + 用户订正 | 用户确认规划（本阶段） |
| **P1 骨架 + 项目台账** | `zzz/developmentManager` 模块骨架、project/relation/asset 表迁移（migrations）、CRUD、软删除级联、seed 登记现有项目、`main.go` 注册 | 接口可建/删项目与关联，删除后关联带 deleted 标记 |
| **P2 部署资产** | container / resource_alloc / volume_mapping / **access_point（端口+反代+地址四合一）** / **whitelist + whitelist_member（可复用白名单组）** / backup 七组实体与 API | 一个项目可完整录入 1.2 所列全部部署信息，接入点一行看全绑定链，白名单组可复用 |
| **P3 数据字典** | db_table/db_column + 扫描导入 + 手工维护 | 表用途、字段含义可录入并展示 |
| **P4 需求管理** | requirement/version、状态流转、软删除快照 | 增改更新删 + 版本历史全部可回溯 |
| **P5 生成流程** | generation 任务、plan/doc 模板生成为主 → **末尾接入 services/ai 作为可选 engine（含降级）**；code 产出 patch/分支草案，**`auto_commit` 开关（默认关，受 `dm.generation:autocommit` 权限约束）** | 由需求一键生成方案与文档，AI 关闭时仍可完成，失败可重试 |
| **P6 前端模块** | Go-Frontend-Core 中 developmentManager 模块（Web） | 全流程可视化操作走通 |
| **P7 收尾** | Swagger(`make swagger`)、docs/projects 归档、测试、发布说明 | 通过 golangci-lint + 单测，文档齐全 |
| **P8 历史模块规范化**（P1 之后穿插） | 将现有 `zzz/ia`、`zzz/goodser`、`zzz/shopchecker`、`zzz/login3rd` 等的 `initTables()` DDL 迁移到 `migrations/` 统一迁移，并登记进数据字典 | 各模块启动不再自建表，迁移版本表可见全部 DDL（独立提交，不阻塞 P2-P7） |

依赖关系：P1 → P2/P3/P4（可并行）→ P5 → P6 → P7；P8 独立穿插、单独提交。

---

## 7. 风险与原则

- **不动现有基座行为**：仅新增 `zzz/developmentManager` 与 `main.go` 一行注册，既有模块改造（P8）独立提交；
- **软删除贯穿**：所有实体默认软删除，关联同步标记，避免误删破坏台账；
- **生成不直落主干**：代码生成产出分支/patch 草案，人工评审后合并；
- **AI 可选可降级**：内建 AI 不可用时生成任务仍以模板/手工方式完成，不阻塞主流程；
- **数据字典渐进**：先扫描导入结构，含义字段由人补全，不追求自动推断；
- **前端仓库独立**：Go-Frontend-Core 单独分支与提交，不与本仓库混提。

---

## 8. 确认状态

**全部决策已并入 §0（Q1–Q4、Q1a、Q5–Q9a），规划已核对完毕。** 表前缀规范已独立落档至 `docs/data-model.md`。

> 下一步：启动 **P1（模块骨架 + 项目台账）**。
