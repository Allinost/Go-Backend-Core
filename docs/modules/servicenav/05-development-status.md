# 服务导航模块开发状态

> 最后更新：2026-08-06
> 状态：**已实现**（后端完成，前端待对接）

## 1. 完成情况

| 项 | 状态 | 说明 |
|----|------|------|
| 服务条目 CRUD | ✅ | 增/删/改/查（详情），见 [03-api-design.md](03-api-design.md) |
| 服务类型 CRUD | ✅ | 类型独立管理，条目多选引用 |
| 列表查询 | ✅ | 名称模糊搜索 + 类型筛选 + 排序 + 分页 |
| 自动建表 | ✅ | `services_nav_item` / `services_nav_type`，启动时 `CREATE TABLE IF NOT EXISTS` |
| 旧表兼容 | ✅ | `ensureColumn` 自动为已存在的表补充 `protocol` 列 |
| 传输协议字段 | ✅ | `protocol` 默认 `http`，可设 `https` 等 |
| 密码提示字段 | ✅ | `password_hint`（非真实密码，明文返回） |
| 内网端口 NULL | ✅ | `intranet_port` 可 NULL，更新用 `clear_intranet_port` 显式置空 |
| 名称唯一性 | ✅ | 条目名与类型名均有唯一索引 + 业务校验 |
| 类型引用完整性 | ✅ | 创建/更新条目校验类型已注册；删除类型时被引用则拒绝 |
| Swagger 文档 | ✅ | `make swagger` 已生成，含全部 9 个接口 |
| 模块文档 | ✅ | `docs/modules/servicenav/` 01-04 + 本文件 |
| 单元测试 | ✅ | `sanitizeSort` / `Normalize` / 构造函数 |
| 注册与构建 | ✅ | `cmd/server/main.go` 注册，`go build ./...` / `go vet` / `go test` 通过 |

## 2. 代码清单

```
internal/modules/servicenav/
├── module.go              # Module 接口实现（自动建表 + ensureColumn + 路由注册）
├── handler.go             # HTTP 处理器（swagger 注释）
├── service.go             # 业务逻辑（名称唯一、类型校验、删除保护、默认协议）
├── repository.go          # 数据访问层（MySQL，JSON_CONTAINS 筛选，排序白名单）
├── model.go               # 数据模型 + DTO + 分页响应
├── errors.go              # 模块级错误码（41001-41200）
└── servicenav_test.go     # 单元测试
```

注册：`cmd/server/main.go` → `modules.Register(&servicenav.Module{})`，路由前缀 `/api/v1/servicenav`。

## 3. 数据库变更记录

### 3.1 初始建表

| 表 | 说明 |
|----|------|
| `services_nav_item` | 服务条目（name 唯一；types JSON；protocol；address；intranet_port NULL；easytier_port；account；password_hint；remark） |
| `services_nav_type` | 服务类型（name 唯一；remark） |

### 3.2 迭代变更

| 变更 | SQL | 触发 |
|------|-----|------|
| 新增 `protocol` 列 | `ALTER TABLE services_nav_item ADD COLUMN protocol VARCHAR(20) NOT NULL DEFAULT 'http' AFTER types` | 模块启动 `ensureColumn` 自动检测并执行 |

> 注意：`CREATE TABLE IF NOT EXISTS` 不会修改已存在的表，因此所有后续加列都必须通过 `ensureColumn`（基于 `information_schema.COLUMNS` 检测）兜底，而非只改建表 DDL。

## 4. 已修复的问题

| 问题 | 根因 | 修复 |
|------|------|------|
| `GET /services` 报 `Unknown column 'protocol'` (Error 1054) | 表在加字段前已存在，DDL 未生效 | `initTables` 增加 `ensureColumn` 自动补列 |
| `errors.go` 存在未使用的 `ErrInvalidSort` 死代码 | 排序采用白名单回退策略 | 移除该常量及文档中 41201 错误码 |
| gofmt 未格式化 | 手工编辑破坏对齐 | `gofmt -w` 全模块格式化 |

## 5. 已知限制与后续项

| 项 | 说明 | 建议 |
|----|------|------|
| Service 层测试缺失 | 现有单测仅覆盖纯函数（排序白名单、分页参数、构造函数）；`validateItemTypes`/名称冲突/删除保护依赖 DB | 用 sqlmock 或对 Repository 做接口化后补充 service 单测 |
| 前端未实现 | 后端 API 已就绪 | 按 03-api-design + 对接 prompt 开发页面（列表/表单/类型管理） |
| `api/proto` 为空 | 项目无 proto/gRPC 生成链路 | 如需 gRPC 再补充，当前 REST 已够用 |
| 排序字段白名单回退 | 非法 `sort_by` 静默回退 `name ASC`（而非报错） | 属有意设计（对前端更友好），如需严格校验可再调整 |
| `intranet_port`/`easytier_port` 无范围校验 | 未限制 0-65535 | 前端先校验；后端如需可在 service 层补充 |

## 6. 前端对接

前端开发提示词见对话记录（「服务导航模块前端对接 Prompt」+「协议字段追加说明」），核心要点：

- 列表页表格 + 名称搜索 + 类型下拉筛选 + 排序控件 + 分页器
- 创建/编辑弹窗：类型多选（数据源 `GET /types`）、协议下拉默认 `http`、内网端口开关控制提交 `null`
- 类型管理入口，删除被引用类型时展示 41102 提示
- 业务成败以响应 `code === 0` 判断，失败展示 `message`
