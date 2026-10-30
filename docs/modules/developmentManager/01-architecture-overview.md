# developmentManager 架构总览

## 模块定位

对"项目开发"全生命周期做可视化管理的**台账型业务模块**：项目（基座/模板/模块）登记与关联、部署资产、数据表字典、需求与生成流程。它自身也是受管对象（`developmentmanager` 项目关联 `go-backend-core` 基座）。

## 分层架构

```
zzz/developmentManager/
├── module.go        # modules.Module 实现：Init 装配 / RegisterRoutes 挂路由
├── handler.go       # Gin 处理器 + swag 注解 + 审计 + 错误映射
├── service.go       # 业务编排：校验、删除二次确认、级联编排
├── repository.go    # raw SQL 数据访问（database/sql，db 标签）
├── model.go         # 实体、常量、请求结构、分页响应
├── errors.go        # 模块错误码 46001+（DevelopmentManagerError）
├── permissions.go   # 权限矩阵（28 点）+ 幂等播种 + 默认角色
├── seed.go          # 台账 seed（项目/关联/资产，后台重试）
└── *_test.go        # 纯逻辑单测（校验/错误映射/权限矩阵/seed 一致性）
```

依赖方向：`handler → service → repository → *sql.DB`，无反向依赖。

## 依赖关系

| 依赖 | 用途 | 获取方式 |
|---|---|---|
| `database.DB.MySQL["main"]` | MySQL 连接池（`*sql.DB`） | Init 时直接取 |
| `internal/modules/auth` | 登录中间件 + RBAC 服务 | `modules.Get("auth")` → `AuthMiddleware()` / `RBAC()` |
| `internal/middleware` | `RequirePermission(rbac, resource, action)` | 逐路由挂载 |
| `internal/services/audit` | 写操作审计（包级 Record*） | handler 内调用，失败不阻断 |
| `internal/services/scheduler`（P5+） | 生成任务调度 | 计划中 |

**注册顺序**：`main.go` 中注册于全部 zzz 模块之后（依赖 auth 先 Init）。

## 权限模型

- 资源 × 动作矩阵（规划 §4.1）：`dm.project / dm.relation / dm.deployment / dm.dictionary / dm.requirement / dm.generation / dm.asset`，共 28 个权限点，`Init` 幂等播种（名称 = `resource.action`）。
- **角色动态可增**：admin 经 `/api/v1/auth/admin/roles` 与 `roles/:name/permissions` 管理；`editor`（read/create/update + 需求审批）、`viewer`（read）仅在不存在时播种，不覆盖既有配置。
- 敏感双保险：`dm.generation:autocommit` 权限 + 请求体 `auto_commit=1`（P5）。
- 元数据接口 `GET /permissions/meta` 输出本模块权限清单，供管理端勾选分配。

## 与规划阶段的对应

P1（本阶段）：项目台账 + 关联 + 资产索引 + 权限/seed。
P2 部署资产（container/resource_alloc/volume_mapping/access_point/whitelist×2/backup）→ P3 数据字典 → P4 需求 → P5 生成 → P6 前端（Go-Frontend-Core）→ P7 收尾 → P8 历史模块迁移。
