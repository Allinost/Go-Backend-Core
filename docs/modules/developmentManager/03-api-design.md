# developmentManager API 设计

> 路由前缀：`/api/v1/developmentmanager`（全小写）。**全部路由**挂 auth 登录中间件 + 逐路由 `RequirePermission`（资源 × 动作，规划 §4.1）。响应统一走 `internal/pkg/response`（`code/message/data`）。

## P1 已实现路由

### 项目台账（`dm.project`）

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/projects` | dm.project:read | 列表。query：`page`、`page_size`（缺省 20，最大 100）、`type`、`status`、`keyword`（匹配名称/唯一键/简介）、`deleted`（默认只查未删除） |
| POST | `/projects` | dm.project:create | 新建。body：`project_key`（必填，`^[a-z0-9][a-z0-9-]{1,63}$`）、`name`（必填，2-128 字符）、`type`（必填，五选一：base/template/backend_module/frontend_module/frontend_base）、`description`、`repo_url`、`repo_path`、`default_branch`（缺省 main）、`tech_stack`（字符串数组）、`status`（缺省 active） |
| GET | `/projects/:id` | dm.project:read | 详情（含已删除项，`deleted` 字段区分） |
| PUT | `/projects/:id` | dm.project:update | 全量更新（body 同创建；唯一键未变时跳过查重） |
| DELETE | `/projects/:id` | dm.project:delete | 软删除 + 事务级联标记关联与资产。query：`force=1` 时忽略被依赖保护 |

### 项目关联（`dm.relation`）

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/projects/:id/relations` | dm.relation:read | 关联列表（from 或 to 命中即返回，含两端 key/名称）。query：`page`/`page_size`/`type`/`deleted` |
| POST | `/projects/:id/relations` | dm.relation:create | 新建关联。body：`to_project_id`（必填）、`relation_type`（必填，四选一）、`note`。校验：目标存在且未删除、非自关联、同两端同类型未删除关联不重复 |
| DELETE | `/projects/:id/relations/:rel_id` | dm.relation:delete | 软删除关联 |

### 归类资产索引（`dm.asset`）

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/projects/:id/assets` | dm.asset:read | 资产索引列表。query：`page`/`page_size`/`category`/`deleted` |
| POST | `/projects/:id/assets` | dm.asset:create | 登记资产。body：`category`（必填，四选一）、`title`（必填）、`uri`（必填）、`format`、`version`、`size`、`checksum`、`note` |
| DELETE | `/projects/:id/assets/:asset_id` | dm.asset:delete | 软删除资产索引 |

### 统计与权限元数据

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/stats` | dm.project:read | 仪表盘统计：项目总数/已删数/按类型/按状态、启用关联数、资产数 |
| GET | `/permissions/meta` | dm.project:read | 本模块 28 个权限点元数据（resource/action/中文说明），供 admin 权限管理页勾选分配（只读） |

> 角色与权限的增删分配**复用基座现成接口** `/api/v1/auth/admin/*`（roles/permissions/assign），本模块不另造管理 API。

## P2 已实现路由（部署资产，统一 `dm.deployment` 权限）

| 方法 | 路径 | 权限动作 | 说明 |
|---|---|---|---|
| GET | `/projects/:id/containers` | read | 项目容器列表，`deleted=1` 查已删除 |
| POST | `/projects/:id/containers` | create | 新建容器。body：`container_name`（必填，项目内唯一）、`image`、`compose_ref`、`replicas`（缺省 1）、`network_mode`、`env_notes`（KV 对象）、`status_note` |
| GET | `/containers/:id` | read | 容器详情 |
| PUT | `/containers/:id` | update | 全量更新容器 |
| DELETE | `/containers/:id` | delete | 软删除容器，级联软删除其卷与接入点；仍被白名单组引用时 409 |
| GET | `/containers/:id/resources` | read | 资源分配（不存在时 404） |
| PUT | `/containers/:id/resources` | update | 资源分配 upsert（cpu_limit/cpu_shares/mem_limit/mem_reservation/gpu_note/storage_quota/notes） |
| GET | `/containers/:id/volumes` | read | 卷映射列表 |
| POST | `/containers/:id/volumes` | create | 新建卷映射。body：`host_path`、`container_path`（必填）、`mode`（缺省 rw）、`purpose`、`in_backup` |
| PUT | `/volumes/:id` | update | 更新卷映射 |
| DELETE | `/volumes/:id` | delete | 软删除卷映射 |
| GET | `/projects/:id/access-points` | read | 接入点列表，可按 `exposure`、`network_zone`、`deleted` 筛选 |
| POST | `/projects/:id/access-points` | create | 新建接入点。body：`listen_port`（必填）、`host_port`、`container_id`、`protocol`、`exposure`、`proxy_type`、`domain`、`upstream_url`、`tls`、`network_zone`、`url`、`whitelist_id`、`notes` |
| GET | `/access-points/:id` | read | 接入点详情（含 `whitelist_name`） |
| PUT | `/access-points/:id` | update | 全量更新接入点 |
| DELETE | `/access-points/:id` | delete | 软删除接入点 |
| GET | `/projects/:id/whitelists` | read | 白名单组列表（含 `member_count`） |
| POST | `/projects/:id/whitelists` | create | 新建白名单组（允许 0 成员） |
| GET | `/whitelists/:id` | read | 组详情（含成员数） |
| PUT | `/whitelists/:id` | update | 更新组名/说明 |
| DELETE | `/whitelists/:id` | delete | 删除组（事务内删成员）；仍被接入点引用时 409 |
| GET | `/whitelists/:id/members` | read | 成员列表（含容器名，允许空数组） |
| POST | `/whitelists/:id/members` | create | 添加成员。body：`container_id`（必填）、`note` |
| DELETE | `/whitelists/:id/members/:member_id` | delete | 移除成员（硬删除关系行） |
| GET | `/projects/:id/backups` | read | 备份记录列表 |
| POST | `/projects/:id/backups` | create | 新建备份。body：`backup_type`（必填 full/db/volume）、`container_id`、`volume_id`、`strategy_cron`、`destination`、`retention_days`、`restore_note`、`enabled`（缺省 true） |
| GET | `/backups/:id` | read | 备份详情 |
| PUT | `/backups/:id` | update | 更新备份（含 `last_status`/`last_error`/`last_run_at` 执行结果回报） |
| DELETE | `/backups/:id` | delete | 删除备份记录（硬删除） |

部署资产语义约定：
- 接入点 `url` 传空或空白 → 存 `NULL` 且 `reachable=false`；有值 → `reachable=true`（与规划"`url IS NULL` ⇒ 不可达"一致）。
- 接入点 `host_port`、`container_id`、`whitelist_id` 可不传（NULL）；`protocol` 缺省 tcp、`exposure`/`network_zone` 缺省 intranet、`proxy_type` 留空表示无反代直连。
- 跨项目引用（容器/卷/白名单组）返回 46207。

## P3 已实现路由（数据字典，统一 `dm.dictionary` 权限）

| 方法 | 路径 | 权限动作 | 说明 |
|---|---|---|---|
| GET | `/projects/:id/tables` | read | 数据表列表。query：`status`（active/deprecated/dropped）、`module_name`、`keyword`（匹配表名/用途） |
| POST | `/projects/:id/tables` | create | 登记数据表。body：`table_name`（必填，字母/数字/下划线）、`db_type`（缺省 mysql）、`table_schema`（缺省当前库）、`purpose`、`status`（缺省 active）、`module_name`、`created_note` |
| GET | `/tables/:id` | read | 表详情（含 `column_count`） |
| PUT | `/tables/:id` | update | 全量更新表登记 |
| DELETE | `/tables/:id` | delete | 删除表登记及其字段（硬删除） |
| GET | `/tables/:id/columns` | read | 字段列表（主键优先） |
| POST | `/tables/:id/columns` | create | 新增字段。body：`column_name`、`data_type`（必填）、`nullable`（缺省 true）、`default_value`（可空）、`meaning`、`example`、`is_pk`、`is_index`、`note` |
| PUT | `/columns/:id` | update | 全量更新字段 |
| DELETE | `/columns/:id` | delete | 删除字段（硬删除） |
| POST | `/tables/:id/scan-import` | **scan** | 从 `information_schema` 扫描字段并导入/更新；返回 `{table_id, scanned, created, updated}` |

扫描导入约定：
- 仅 `db_type=mysql` 支持（`postgres` 返回 46306）；
- 已有字段更新类型/可空/默认值/主键/索引，**不覆盖已填写的中文含义**；
- 库名/表名/字段名均只允许字母、数字、下划线。

## P4 已实现路由（需求管理，`dm.requirement`）

| 方法 | 路径 | 权限动作 | 说明 |
|---|---|---|---|
| GET | `/projects/:id/requirements` | read | 分页列表。query：`page`/`page_size`、`status`、`type`、`priority`、`keyword`（标题/编号/正文）、`deleted` |
| POST | `/projects/:id/requirements` | create | 新建需求。body：`title`（必填 2-200）、`content`（markdown）、`type`（缺省 feature）、`priority`（缺省 P2）、`source`（缺省 manual）、`owner`、`change_note` |
| GET | `/requirements/:id` | read | 详情（含 `latest_version`，已删除也可查） |
| PUT | `/requirements/:id` | update | 更新标题/正文/类型/优先级/来源/负责人，**不改状态**，自动追加版本快照 |
| POST | `/requirements/:id/status` | **approve** | 状态流转。body：`status`（必填）、`change_note`；非法流转返回 46403 |
| DELETE | `/requirements/:id` | delete | 软删除 + 生成 `status=deleted` 快照 |
| GET | `/requirements/:id/versions` | read | 版本历史（最新在前，含完整历史标题/正文） |

需求状态机：`draft → reviewing → approved → planned → developing → done`，任意非 closed 状态可转 `closed`；`reviewing` 可退回 `draft`，`done` 可打回 `developing`。状态修改必须走 `/status`（受 `approve` 权限约束），PUT 不改变状态。

## 业务错误码（46001+ 区段）

模块错误类型 `DevelopmentManagerError{Code, Message, Detail}`；handler 按码映射 HTTP 状态（非 release 模式才透出 `Detail`）。

| 码 | HTTP | 含义 |
|---|---|---|
| 46001 | 404 | 项目不存在 |
| 46002 | 404 | 项目关联不存在 |
| 46003 | 404 | 资产不存在 |
| 46100 | 400 | 无效的项目唯一键 |
| 46101 | 400 | 无效的项目类型/状态 |
| 46102 | 409 | 项目唯一键已存在（关联重复时复用此冲突语义） |
| 46103 | 400 | 无效的关联类型 |
| 46104 | 409 | 不能将项目关联到自身 |
| 46105 | 404 | 关联目标项目不存在或已删除 |
| 46106 | 409 | 项目存在启用中的被依赖关联，需 `force=1` |
| 46107 | 400 | 无效的资产类别 |
| 46108 | 400 | 项目名称为空或长度不合法 |
| 46004 | 404 | 容器不存在 |
| 46005 | 404 | 容器资源分配不存在 |
| 46006 | 404 | 卷映射不存在 |
| 46007 | 404 | 网络接入点不存在 |
| 46008 | 404 | 白名单组不存在 |
| 46009 | 404 | 白名单成员不存在 |
| 46010 | 404 | 备份记录不存在 |
| 46200 | 400 | 容器名为空或超长 |
| 46201 | 409 | 容器名在项目内已存在 |
| 46202 | 400 | 端口非法（listen_port 1-65535 / host_port） |
| 46203 | 400 | 无效协议 |
| 46204 | 400 | 无效暴露范围 |
| 46205 | 400 | 无效反代类型 |
| 46206 | 400 | 无效网络区域 |
| 46207 | 400 | 跨项目引用 |
| 46208 | 400 | 卷映射路径为空 |
| 46209 | 400 | 无效备份类型 / 负保留天数 |
| 46210 | 400 | 无效备份状态 |
| 46211 | 409 | 白名单组名在项目内已存在 |
| 46212 | 400 | 无效挂载模式 |
| 46213 | 409 | 容器仍被白名单组引用 |
| 46214 | 409 | 白名单组仍被接入点引用 |
| 46215 | 400 | 白名单组名为空或超长 |
| 46216 | 400 | 副本数为负 |
| 46217 | 400 | 资源分配参数非法 |
| 46011 | 404 | 数据表不存在 |
| 46012 | 404 | 字段不存在 |
| 46300 | 400 | 无效数据库类型 |
| 46301 | 400 | 表名非法 |
| 46302 | 409 | 同库表重复登记 |
| 46303 | 400 | 无效表状态 |
| 46304 | 400 | 字段名/类型非法 |
| 46305 | 409 | 同表字段重复 |
| 46306 | 400 | 该数据库类型不支持扫描导入 |
| 46307 | 500 | 扫描 information_schema 失败/未找到表 |
| 46013 | 404 | 需求不存在 |
| 46014 | 404 | 需求版本不存在 |
| 46400 | 400 | 无效需求类型 |
| 46401 | 400 | 无效优先级 |
| 46402 | 400 | 无效需求状态 |
| 46403 | 409 | 非法状态流转 |
| 46404 | 400 | 需求标题为空或长度不合法 |
| 46405 | 400 | 无效需求来源 |
| 46406 | 409 | 需求已删除，不能修改 |

鉴权失败/无权限走基座标准错误（`CodeUnauth`/`CodeForbidden`）。

## 通用约定

- 分页响应：`data = { items: [], has_more: bool, total: int }`，空列表返回 `[]` 而非 `null`。
- 写操作（create/update/delete）审计写入 `services/audit`（记录失败仅告警，不阻断业务）。
- 所有 ID 为正整数路径参数，非法直接 400。

## P5 规划路由（规划 §4 API 概要）

生成 `POST /requirements/:id/generations`（body：`kind: plan|code|doc`、`engine: ai|template|manual`、`auto_commit: 0|1`）与 `GET /generations/:id`。
