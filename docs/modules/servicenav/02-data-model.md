# 服务导航数据模型

## 1. 数据库

- **数据库**: 使用当前开发库（MySQL `main` 连接池）
- **表名**: `services_nav_item` / `services_nav_type`（模块启动时自动建表）
- **字符集**: `utf8mb4`
- **引擎**: InnoDB

---

## 2. 表结构

### 2.1 `services_nav_item` — 服务条目

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(100) | NOT NULL, UNIQUE | 服务名 |
| types | JSON | NOT NULL | 服务类型名称数组，如 `["web","api"]` |
| protocol | VARCHAR(20) | NOT NULL DEFAULT 'http' | 传输协议，默认 http，可设 https 等 |
| address | VARCHAR(2048) | DEFAULT '' | 访问地址 |
| intranet_port | INT | NULL | 内网端口，NULL=不支持内网访问 |
| easytier_port | INT | NULL | EasyTier 端口 |
| account | VARCHAR(255) | DEFAULT '' | 账号 |
| password_hint | VARCHAR(255) | DEFAULT '' | 密码提示（非真实密码，明文返回） |
| remark | VARCHAR(500) | DEFAULT '' | 备注 |
| project_id | BIGINT | NULL | 关联 `zzz_development_manager_project.id`（NULL=未关联） |
| container_id | BIGINT | NULL | 关联 `zzz_development_manager_container.id`（NULL=未关联） |
| access_point_id | BIGINT | NULL | 关联 `zzz_development_manager_access_point.id`，用于导入幂等 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

唯一索引：`uk_name (name)`

> `project_id / container_id / access_point_id` 由模块 `initTables` 的 `ensureColumn` 兼容补充（历史库启动自动加列）；查询时 LEFT JOIN 项目与容器表返回 `project_key / project_name / container_name` 供展示。

### 2.2 `services_nav_type` — 服务类型

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(50) | NOT NULL, UNIQUE | 类型名称 |
| remark | VARCHAR(200) | DEFAULT '' | 备注 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

唯一索引：`uk_name (name)`

---

## 3. Go 数据模型

```go
package servicenav

import "time"

// ServiceType 服务类型，对应 services_nav_type 表
type ServiceType struct {
    ID        string    `json:"_id" db:"id"`
    Name      string    `json:"name" db:"name"`
    Remark    string    `json:"remark,omitempty" db:"remark"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ServiceItem 服务条目（服务导航），对应 services_nav_item 表
// IntranetPort 为 NULL 表示不支持内网访问
type ServiceItem struct {
    ID           string    `json:"_id" db:"id"`
    Name         string    `json:"name" db:"name"`
    Types        []string  `json:"types" db:"types"`
    Protocol     string    `json:"protocol" db:"protocol"`
    Address      string    `json:"address,omitempty" db:"address"`
    IntranetPort *int      `json:"intranet_port" db:"intranet_port"`
    EasyTierPort *int      `json:"easytier_port" db:"easytier_port"`
    Account      string    `json:"account,omitempty" db:"account"`
    PasswordHint  string    `json:"password_hint,omitempty" db:"password_hint"`
    Remark        string    `json:"remark,omitempty" db:"remark"`
    ProjectID     *int64    `json:"project_id,omitempty" db:"project_id"`
    ContainerID   *int64    `json:"container_id,omitempty" db:"container_id"`
    AccessPointID *int64    `json:"access_point_id,omitempty" db:"access_point_id"`
    ProjectKey    string    `json:"project_key,omitempty" db:"project_key"`
    ProjectName   string    `json:"project_name,omitempty" db:"project_name"`
    ContainerName string    `json:"container_name,omitempty" db:"container_name"`
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
```

---

## 4. DTO

```go
// 创建服务类型
type CreateTypeReq struct {
    Name   string `json:"name" binding:"required"`
    Remark string `json:"remark"`
}

// 更新服务类型（指针字段，仅更新非 nil 字段）
type UpdateTypeReq struct {
    Name   *string `json:"name"`
    Remark *string `json:"remark"`
}

// 创建服务条目
type CreateItemReq struct {
    Name         string   `json:"name" binding:"required"`
    Types        []string `json:"types" binding:"required"`
    Protocol     string   `json:"protocol"`   // 默认 http，空则自动填 http
    Address      string   `json:"address"`
    IntranetPort *int     `json:"intranet_port"`
    EasyTierPort *int     `json:"easytier_port"`
    Account      string   `json:"account"`
    PasswordHint string   `json:"password_hint"`
    Remark       string   `json:"remark"`
}

// 更新服务条目（指针字段，仅更新非 nil 字段）
// ClearIntranetPort / ClearEasyTierPort 为 true 时将对应端口置为 NULL
type UpdateItemReq struct {
    Name              *string  `json:"name"`
    Types             []string `json:"types"`
    Protocol          *string  `json:"protocol"`
    Address           *string  `json:"address"`
    IntranetPort      *int     `json:"intranet_port"`
    ClearIntranetPort bool     `json:"clear_intranet_port"`
    EasyTierPort      *int     `json:"easytier_port"`
    ClearEasyTierPort bool     `json:"clear_easytier_port"`
    Account           *string  `json:"account"`
    PasswordHint      *string  `json:"password_hint"`
    Remark            *string  `json:"remark"`
}

// 服务条目列表查询
type ListItemsReq struct {
    Page      int    `form:"page"`
    PageSize  int    `form:"page_size"`
    Keyword   string `form:"keyword"`
    Type      string `form:"type"`
    SortBy    string `form:"sort_by"`
    SortOrder string `form:"sort_order"`
}

// 分页响应
type PaginatedResp[T any] struct {
    Items   []T   `json:"items"`
    HasMore bool  `json:"has_more"`
    Total   int   `json:"total"`
}
```

> 说明：`intranet_port`/`easytier_port` 为指针，`nil` 即 JSON `null`（NULL 端口）；更新时通过 `clear_intranet_port`/`clear_easytier_port` 区分「不修改」与「显式置空」。
