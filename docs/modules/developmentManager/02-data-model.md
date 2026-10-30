# developmentManager 数据模型

> 表前缀规范见 [docs/data-model.md](../../../data-model.md)：模块表统一 `zzz_development_manager_` 前缀，建表统一走 migrations 迁移（模块不 initTables）。本模块规划共 **15 张表**，P1 已建 3 张。

## 总览

| # | 表 | 阶段 | 用途 |
|---|---|---|---|
| 1 | `zzz_development_manager_project` | P1 | 项目台账（基座/模板/模块） |
| 2 | `zzz_development_manager_project_relation` | P1 | 项目关联（基座/模板/模块绑定） |
| 3 | `zzz_development_manager_project_asset` | P1 | 归类资产索引（需求/代码/文档/附件路径索引） |
| 4 | `zzz_development_manager_container` | P2 | 项目关联容器 |
| 5 | `zzz_development_manager_resource_alloc` | P2 | 资源分配（容器 1:1） |
| 6 | `zzz_development_manager_volume_mapping` | P2 | 卷映射 |
| 7 | `zzz_development_manager_access_point` | P2 | 网络接入点（端口+反代+地址+白名单引用 四合一） |
| 8 | `zzz_development_manager_whitelist` | P2 | 常用白名单组（可复用，允许空组） |
| 9 | `zzz_development_manager_whitelist_member` | P2 | 白名单成员（FK → container） |
| 10 | `zzz_development_manager_backup` | P2 | 备份策略与最近执行结果 |
| 11 | `zzz_development_manager_db_table` | P3 | 数据表登记 |
| 12 | `zzz_development_manager_db_column` | P3 | 字段字典 |
| 13 | `zzz_development_manager_requirement` | P4 | 需求登记与状态流转 |
| 14 | `zzz_development_manager_requirement_version` | P4 | 需求版本快照（增改删全留痕） |
| 15 | `zzz_development_manager_generation` | P5 | 生成任务（plan/code/doc） |

## P1 已建表（migrations/mysql/002_development_manager.sql）

### zzz_development_manager_project — 项目台账

登记一个受管项目的基本信息与仓库位置。字段与规划 §3.1 一致：

| 字段 | 类型 | 含义 |
|---|---|---|
| id | bigint PK | 主键 |
| project_key | varchar(64) UK | 项目唯一键（目录/接口引用用，如 `go-backend-core`；格式：小写字母/数字/中划线，2-64 位） |
| name | varchar(128) | 项目名称（2-128 字符） |
| type | varchar(32) | 项目类型：`base` / `template` / `backend_module` / `frontend_module` |
| description | text | 项目简介 |
| repo_url | varchar(255) | 代码仓库地址（可空） |
| repo_path | varchar(255) | 仓库本地路径（可空，相对仓库根目录） |
| default_branch | varchar(64) | 默认分支（缺省 `main`） |
| tech_stack | json | 技术栈摘要数组（语言/框架/DB），仅作展示 |
| status | varchar(16) | `active` / `archived` |
| deleted / deleted_at | — | 软删除标记与时间 |
| created_by / updated_by / created_at / updated_at | — | 审计字段 |

### zzz_development_manager_project_relation — 项目关联

记录项目间基座/模板/模块绑定关系（多对多）：

| 字段 | 含义 |
|---|---|
| id, from_project_id, to_project_id | 关联两端（均为 project.id） |
| relation_type | `uses_base`（模块→基座）/ `uses_template`（前端→模板）/ `binds_backend_module`（前端→后端模块）/ `related`（其他） |
| note | 关联说明 |
| deleted / deleted_at | 软删除标记（删除任一端项目时事务内同步置位） |
| created_by / created_at / updated_at | 审计字段 |
| UK(from, to, type) | 防重复关联 |

### zzz_development_manager_project_asset — 归类资产索引

需求/代码/文档/资产按项目归档的**路径索引，文件本体不入库**：

| 字段 | 含义 |
|---|---|
| project_id | 所属项目 |
| category | `requirement` / `code` / `doc` / `asset` |
| title, uri | 标题与定位（仓库相对路径或 fileref ID） |
| format / version / size / checksum | 文件元信息（可空/缺省 0） |
| note / deleted / deleted_at / created_at / updated_at | 备注与审计字段 |

## 软删除与级联（P1 语义）

- 所有删除均为**软删除**（`deleted=1` + `deleted_at`），列表接口默认只查未删除，`deleted=1` 参数可查已删除项。
- **删除项目**（`DELETE /projects/:id`）在一个事务（`DeleteProjectTx`）内完成：
  1. 项目本体置 `deleted=1`；
  2. 级联标记该项目**发出与收到的全部未删除关联**（from 或 to 命中）；
  3. 级联标记该项目全部资产索引。
- **删除保护**：存在启用中的被依赖关联（其他项目 `uses_base` 等指向它）时默认拒绝（错误码 46106），返回被依赖清单；请求加 `force=1` 强制删除。

## P2~P5 规划表要点（详见规划 §3.2~§3.4）

- **access_point 四合一**：端口（listen/host_port/protocol/exposure）+ 反代（proxy_type/domain/upstream_url/tls，可空=直连）+ 访问地址（url **NULL=不可达**）+ `whitelist_id`（**NULL=不限制**）；行粒度 = 端口 × 访问地址。
- **whitelist 组复用**：组 + 成员两表，组**允许 0 成员**；成员 FK → container，容器改名自动跟随。
- **requirement_version**：增=初始版本、改=新增版本、删=软删除+"已删除"快照，全部可回溯。
- **generation**：`engine=ai|template|manual`，AI 不可用自动降级；`auto_commit` 默认 0，仅对 `kind=code` 生效。

## Go 模型（model.go）

- 实体：`Project` / `ProjectRelation`（含两端 key、name JOIN 展示）/ `ProjectAsset`；`TechStack` 为 JSON 列，扫描后解析为 `[]string`。
- 请求：`ListProjectsReq`（page/page_size/type/status/keyword/deleted）、`CreateProjectReq`（= `UpdateProjectReq` 全量更新）、`CreateRelationReq`、`CreateAssetReq` 等，分页统一 `Normalize()`（page≥1，page_size 1-100 缺省 20）。
- 响应：`PaginatedResp[T]`（items/has_more/total，与现有 zzz 模块一致）、`StatsResp`（仪表盘统计）。
- 常量：`ProjectType*` / `ProjectStatus*` / `Relation*` / `AssetCategory*` + 对应 `Valid*` 合法集合（单测校验数量与内容）。
