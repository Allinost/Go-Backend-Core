# 服务导航模块开发文档

## 文档目录

| 文件 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 模块定位、架构总览、核心能力、命名空间 |
| [02-data-model.md](02-data-model.md) | 数据表设计（zzz_servicenav_item / zzz_servicenav_type）、Go 模型、DTO |
| [03-api-design.md](03-api-design.md) | 服务条目 API、服务类型 API、查询筛选/排序/分页 |
| [04-module-structure.md](04-module-structure.md) | 模块文件组织、分层实现、错误码定义 |
| [05-development-status.md](05-development-status.md) | 开发状态、代码清单、DB 变更、已修复问题、已知限制 |

## 关键设计决策

1. **类型独立管理**：服务类型维护在独立的 `zzz_servicenav_type` 表，由用户自行管理（增删改），服务条目通过 `types`（JSON 数组）引用类型名称
2. **多类型支持**：一条服务条目可关联多个服务类型，列表支持按任意类型筛选（`JSON_CONTAINS`）
3. **引用完整性**：创建/更新服务条目时校验类型必须已注册；删除类型时若被条目引用则禁止删除
4. **内网端口可空**：`intranet_port` 支持 NULL（不支持内网访问），更新接口通过 `clear_intranet_port` 显式置空
5. **安全**：密码字段为「密码提示」（非真实密码）明文存储与返回；排序字段白名单防 SQL 注入
6. **名称唯一**：服务条目名与服务类型名均加唯一索引，重复时返回业务错误
