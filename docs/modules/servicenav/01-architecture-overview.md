# 服务导航模块架构概览

## 1. 模块定位

服务导航模块（`servicenav`）是一个**业务管理模块**，用于统一记录、维护内网/外网可访问的服务条目清单。它面向运维场景：把散落在各处的手工记录（服务名、访问地址、内网端口、EasyTier 端口、账号与密码提示、备注）集中到一处，支持按名称模糊搜索、按类型筛选、排序与分页，方便快速找到某个服务的访问方式和凭据信息。

### 1.1 解决的问题

| 问题 | 方案 |
|------|------|
| 服务访问信息散落在聊天记录/笔记里，难找难维护 | 集中维护在 `services_nav_item` 表，提供统一管理 API |
| 服务类型随意、口径不一 | 独立 `services_nav_type` 表管理类型，条目引用已注册类型 |
| 内网不开放的服务无法标注端口 | `intranet_port` 支持 NULL 表达「不支持内网访问」 |
| 密码记在明文笔记里容易泄露 | 明确字段语义为「密码提示」，仅辅助回忆，不存真实密码 |

### 1.2 实体关系

```
services_nav_type (服务类型)             services_nav_item (服务条目)
┌──────────────────────┐               ┌──────────────────────────────────┐
│ id PK                │               │ id PK                            │
│ name UNIQUE          │◄──引用───     │ name UNIQUE                      │
│ remark               │   types[]     │ types JSON (类型名称数组)         │
│ created_at           │   (JSON数组)   │ address / intranet_port(NULL)    │
│ updated_at           │               │ easytier_port / account          │
└──────────────────────┘               │ password_hint / remark          │
                                       │ created_at / updated_at          │
                                       └──────────────────────────────────┘
```

### 1.3 命名空间

| 层面 | 策略 | 示例 |
|------|------|------|
| 代码目录 | `internal/modules/servicenav/` | `internal/modules/servicenav/service.go` |
| 表名 | `services_nav_{表名}` | `services_nav_item`, `services_nav_type` |
| 路由前缀 | `/api/v1/servicenav` | `/api/v1/servicenav/services` |
| 模块名 | `servicenav` | `modules.Register(&servicenav.Module{})` |

---

## 2. 核心能力

### 2.1 服务条目管理

- 服务条目的 CRUD（名称、类型、访问地址、内网端口、EasyTier 端口、账号、密码提示、备注）
- 列表查询：按服务名**模糊搜索**（`LIKE %kw%`）、按类型**筛选**（`JSON_CONTAINS`）、**排序**（name/created_at，asc/desc）、**分页**（page/page_size，默认 name 升序）
- 名称唯一校验，内网端口支持显式置 NULL

### 2.2 服务类型管理

- 类型的 CRUD（名称 + 备注），名称唯一
- 删除保护：类型被服务条目引用时禁止删除（`ErrTypeInUse`）
- 创建/更新条目时校验类型必须已注册（`ErrInvalidTypeRef`）

### 2.3 端口语义

| 字段 | 含义 | NULL 语义 |
|------|------|-----------|
| `intranet_port` | 内网端口 | NULL = 不支持内网访问 |
| `easytier_port` | EasyTier 虚拟组网端口 | 无特殊语义 |

---

## 3. 依赖关系

```
servicenav Module
  ├── internal/database            # MySQL Pool (database.DB.MySQL["main"])
  ├── internal/pkg/response        # 统一响应格式
  ├── internal/pkg/errors          # 错误码体系
  ├── internal/pkg/logger          # 结构化日志
  └── github.com/google/uuid       # 主键生成
```

不依赖 Redis、不依赖其他业务模块，纯 MySQL 单机即可运行。
