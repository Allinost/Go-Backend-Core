# developmentManager 模块结构

> 代码位置：`zzz/developmentManager`（Go 包名 `developmentmanager`，与目录名大小写差异符合现有惯例）。

## 文件组织（P1 现状）

| 文件 | 职责 |
|---|---|
| `module.go` | `modules.Module` 实现：`Name()="developmentmanager"`、`Init`（取 MySQL 连接 → 解析 auth 模块 → 幂等播种权限 → 构建 svc/handler → 后台启动台账 seed）、`RegisterRoutes`（登录中间件 + 逐路由 `RequirePermission`）、`GetService()` 供其他模块调用 |
| `handler.go` | Gin 处理器 + swag 注解（`@Router /developmentmanager/...`、`@Security BearerAuth`）；`actorName/actorID` 从 auth 中间件注入的上下文取操作人；`handleError` 统一模块错误 → HTTP 状态映射；`recordAudit` 审计（失败不阻断）；`parseID` 路径参数校验 |
| `service.go` | 业务编排：`validateProjectReq`（key 正则/名称长度/类型/状态归一化）、创建前唯一键查重、更新时"键未变跳过查重"、删除二次确认（被依赖清单 + force）、关联创建五重校验（发起方存在/类型合法/非自关联/目标存在未删除/不重复） |
| `repository.go` | raw SQL（`database/sql`）：三实体 CRUD、JOIN 关联两端项目 key/名称、`LIMIT n+1` 探测 has_more、`DeleteProjectTx` 事务级联软删除、`Stats` 分组统计 |
| `model.go` | 实体、枚举常量与 `Valid*` 集合、请求结构（含分页 `Normalize/Offset`）、`PaginatedResp[T]`、`StatsResp` |
| `errors.go` | 错误码常量（46001+）、中文消息表、`DevelopmentManagerError{Code,Message,Detail}` 与 `WithDetail` |
| `permissions.go` | `PermissionSpecs` 28 个权限点（resource×action+中文说明，`init()` 拼 name）、`SeedPermissions` 幂等播种、`ensureRole` 默认角色（editor/viewer 仅不存在时创建，不覆盖） |
| `seed.go` | 台账 seed：8 个现有项目（基座/NKUI/前端基座/5 个 zzz 模块）+ 7 条关联 + 3 条资产索引，全部幂等；`seedWithRetry` 迁移未应用时每 5 秒重试至 60 次 |
| `deployment_model.go` | P2 部署资产实体（container/resource_alloc/volume_mapping/access_point/whitelist/whitelist_member/backup）、枚举合法集合、请求结构 |
| `deployment_repository.go` | P2 数据访问：7 实体 CRUD、接入点 LEFT JOIN 白名单组名、白名单成员 JOIN 容器名、容器删除事务级联、资源分配 `ON DUPLICATE KEY UPDATE` |
| `deployment_service.go` | P2 业务校验：端口/协议/暴露/反代/区域/挂载模式/备份类型、跨项目引用、白名单成员与容器引用约束、`url=NULL` 不可达归一化 |
| `deployment_handler.go` | P2 HTTP 处理器（29 条路由）+ swag 注解 + 审计 |
| `deployment_test.go` | P2 校验逻辑、枚举集合、错误码映射单测 |
| `service_test.go` | 校验逻辑、错误码→状态映射、枚举集合数量 |
| `permissions_test.go` | 权限命名规范/数量（28）/去重；seed 数据一致性（关联两端与资产归属均已登记、类型合法） |

规划中 P3/P5 将新增子包 `scan/`（数据字典扫描导入）与 `generation/`（plan/code/doc 生成器 + AI 降级），届时按规划 §4 目录拆分。

## 生命周期

1. **注册**：`cmd/server/main.go` 中 `modules.Register(&developmentmanager.Module{})`，注册于全部 zzz 模块之后（依赖 auth 模块先 Init）。
2. **Init**：
   - 取 `database.DB.MySQL["main"]` 连接池（`*sql.DB`），不可用则启动失败；
   - `modules.Get("auth")` 取 `*auth.Module` → `AuthMiddleware()`、`RBAC()`（auth 模块为此新增 `RBAC()` 访问器）；
   - `SeedPermissions` 幂等播种 28 个权限点与 editor/viewer 角色（失败则启动失败，保证接口不裸奔）；
   - `go seedWithRetry()` 后台幂等登记台账数据（迁移未应用不致命）。
3. **RegisterRoutes**：路由组 `r.Use(m.authMW)`，每条路由挂 `middleware.RequirePermission(rbac, resource, action)`。
4. **Close**：无额外资源需要清理。

## 迁移与部署注意

- DDL 在 `migrations/mysql/002_development_manager.sql`（P1 三表）与 `003_development_manager_deployment.sql`（P2 七表），按序应用，需手动触发：`POST /api/v1/migrate/schema/migrate`。
- 迁移按英文分号拆分执行：迁移 SQL 的注释与字符串内**不得出现英文分号**。
- 权限播种与路由注册不依赖迁移（无表也能启动）；台账 seed 依赖迁移，未应用时后台重试并在日志中提示。
- Swagger 已随模块生成：`swag init -g cmd/server/docs.go -o api/swagger --parseDependency --parseInternal`。

## 已知限制 / 待确认

- `dm.project:restore` 权限点已播种，但项目恢复接口（`undelete`）规划在收尾阶段实现，暂未提供。
- 部署资产的列表接口不分页（单项目内条目量级小），接入点支持按暴露范围/网络区域筛选；如需分页在 P7 统一补齐。
- 项目类型已扩展为 5 类（2026-10-30 用户确认新增 `frontend_base` 前端基座），`go-frontend-core` 登记为此类型。
- 历史 zzz 模块的 `initTables` 建表方式未改造（P8 统一迁移到 migrations）。
