# developmentManager — 项目开发管理模块

> 代码位置：`zzz/developmentManager` ｜ 路由前缀：`/api/v1/developmentmanager`
> 规划：[docs/plans/feature-V2-project-dev-management.md](../../plans/feature-V2-project-dev-management.md)
> 状态：**P1（模块骨架 + 项目台账）已落地**，P2~P7 按规划推进

## 文档目录

| 文档 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 模块定位、分层架构、依赖关系、权限模型 |
| [02-data-model.md](02-data-model.md) | 表结构（P1 已建 3 表 + P2~P4 规划表）、Go 模型 |
| [03-api-design.md](03-api-design.md) | 路由表、请求/响应、业务错误码 |
| [04-module-structure.md](04-module-structure.md) | 文件组织、生命周期、已知限制 |

## 关键设计决策（详见规划 §0）

1. **表前缀** `zzz_development_manager_`，符合 `docs/data-model.md` 规范；DDL 走 `migrations/mysql/002_development_manager.sql` 统一迁移，模块不自建表。
2. **软删除贯穿**：项目删除 = 软删除 + 事务级联标记关联与资产；存在启用中的被依赖关联时需 `force=1`。
3. **权限 = 资源 × 动作矩阵**（28 个权限点，`dm.*`），模块 `Init` 幂等播种；角色可动态增删，由 admin 经 `/api/v1/auth/admin/*` 分配；`editor`/`viewer` 仅初始播种。
4. **鉴权**：全路由组挂 `auth` 模块登录中间件 + 逐路由 `RequirePermission`（本模块是首个使用 RBAC 的 zzz 模块）。
5. **seed 幂等**：启动后台登记现有项目（基座/模板/zzz 模块）+ 关联 + 资产索引，迁移未应用时自动重试。
