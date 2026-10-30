# developmentManager — 项目开发管理模块

> 代码位置：`zzz/developmentManager` ｜ 路由前缀：`/api/v1/developmentmanager`
> 规划：[docs/plans/feature-V2-project-dev-management.md](../../plans/feature-V2-project-dev-management.md)
> 状态：**P1（模块骨架 + 项目台账）、P2（部署资产）、P3（数据字典）、P4（需求管理）、P5（生成流程）、P6（前端 Web）已落地**，P7 收尾按规划推进

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
5. **seed 幂等**：启动后台登记现有项目（基座/模板/前端基座/zzz 模块）+ 关联 + 资产索引，迁移未应用时自动重试。
6. **部署建模（P2）**：端口+反代+访问地址合并 `access_point`（行粒度=端口×地址，`url=NULL` 即不可达）；白名单独立为可复用组+成员（允许空组），接入点只存 `whitelist_id`（NULL=不限制）；资源分配独立 1:1 表。容器删除级联卷与接入点，被白名单引用时需先移除成员。
7. **数据字典（P3）**：手工登记 + `information_schema` 扫描导入初稿；扫描只更新结构信息，不覆盖人工填写的字段含义。
8. **需求版本留痕（P4）**：创建/修改/状态流转/删除全部追加版本快照；状态只能经 `/status` 流转（受 `dm.requirement:approve` 约束），PUT 不改状态。
9. **生成流程（P5）**：模板生成为主线，`engine=ai` 在 Provider 不可用/失败时自动降级并记录 `engine_used`；代码产物为草案目录不落主干；`auto_commit` 需权限与开关双满足，且只允许在特性分支提交本次产物。
10. **前端（P6）**：Go-Frontend-Core 新增 `developmentManager` 模块（Web），列表页 + 六 Tab 详情页；状态流转仅展示合法目标状态，生成页支持触发/重试/产物查看。
11. **容器项目与容器管理（Q14）**：新增 `container_project` 类型管理 docker compose 部署栈；「容器管理」页面位于运维管理下，与项目开发管理同级，汇总容器项目及容器/接入点并可直达部署 Tab。
12. **历史资料统一纳管（Q16）**：把后端基座、5 个 zzz 模块、前端基座、NKUI 模板、容器栈的现有文档与历史需求统一登记进项目台账（26 条资产索引 + 21 条历史需求），seed 幂等，后续启动不重复导入。
