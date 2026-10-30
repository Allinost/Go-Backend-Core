# 服务导航 Go 模块结构

## 1. 模块文件组织

```
internal/modules/servicenav/
├── module.go              # Module 接口实现（自动建表 + 路由注册）
├── handler.go             # HTTP 处理器（类型/条目 API + swagger 注释）
├── service.go             # 业务逻辑（名称唯一、类型校验、删除保护）
├── repository.go          # 数据访问层（MySQL，含排序白名单、项目/容器 JOIN）
├── import.go              # 从项目部署接入点导入导航（类型范围、URL 拆分、幂等/重名处理）
├── model.go               # 数据模型 + DTO + 分页响应
├── errors.go              # 模块级错误码
└── servicenav_test.go     # 单元测试
```

---

## 2. module.go — 模块入口

```go
package servicenav

const (
    tableItem = "services_nav_item"
    tableType = "services_nav_type"
)

func (m *Module) Name() string { return "servicenav" }

func (m *Module) Init(cfg *config.Config) error {
    pool := database.DB.MySQL["main"]
    if pool == nil {
        return fmt.Errorf("servicenav: MySQL 连接不可用")
    }
    if err := initTables(pool.DB); err != nil {
        return fmt.Errorf("servicenav: 初始化数据表失败: %w", err)
    }
    m.repo = NewRepository(pool.DB)
    m.svc = NewService(m.repo)
    m.h = NewHandler(m.svc)
    return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    r.GET("/types", m.h.ListTypes)
    r.POST("/types", m.h.CreateType)
    r.PUT("/types/:id", m.h.UpdateType)
    r.DELETE("/types/:id", m.h.DeleteType)

    r.GET("/services", m.h.ListItems)
    r.POST("/services", m.h.CreateItem)
    r.GET("/services/:id", m.h.GetItem)
    r.PUT("/services/:id", m.h.UpdateItem)
    r.DELETE("/services/:id", m.h.DeleteItem)
}
```

自动建表语句见 `module.go#initTables`：`CREATE TABLE IF NOT EXISTS services_nav_item / services_nav_type`。

---

## 3. Service 层 — 核心接口

```go
type Service struct {
    repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// 服务类型
func (s *Service) ListTypes(ctx) ([]ServiceType, error)
func (s *Service) GetType(ctx, id) (*ServiceType, error)
func (s *Service) CreateType(ctx, req) (*ServiceType, error)   // name 唯一校验
func (s *Service) UpdateType(ctx, id, req) (*ServiceType, error) // name 唯一校验
func (s *Service) DeleteType(ctx, id) error                     // 引用保护

// 服务条目
func (s *Service) ListItems(ctx, req) ([]ServiceItem, bool, int, error) // 筛选/排序/分页
func (s *Service) GetItem(ctx, id) (*ServiceItem, error)
func (s *Service) CreateItem(ctx, req) (*ServiceItem, error)  // name 唯一 + 类型校验
func (s *Service) UpdateItem(ctx, id, req) (*ServiceItem, error)
func (s *Service) DeleteItem(ctx, id) error

// 校验条目引用的类型名称均已注册
func (s *Service) validateItemTypes(ctx, types []string) error
```

### 创建/更新流程要点

```go
func (s *Service) CreateItem(ctx, req) (*ServiceItem, error) {
    if req.Name == "" {
        return nil, NewNavError(ErrParamNameRequired)
    }
    exists, _ := s.repo.GetItemByName(ctx, req.Name)
    if exists != nil {
        return nil, NewNavError(ErrItemNameConflict)
    }
    if err := s.validateItemTypes(ctx, req.Types); err != nil {
        return nil, err
    }
    return s.repo.CreateItem(ctx, req)
}

// 删除类型前检查引用
func (s *Service) DeleteType(ctx, id) error {
    t, err := s.repo.GetType(ctx, id)
    // ... not found 处理
    count, _ := s.repo.CountItemsByType(ctx, t.Name)
    if count > 0 {
        return NewNavError(ErrTypeInUse)
    }
    return s.repo.DeleteType(ctx, id)
}
```

---

## 4. Repository 层 — 关键实现

### 4.1 列表查询（模糊搜索 + 类型筛选 + 分页）

```go
func (r *Repository) ListItems(ctx, req) ([]ServiceItem, bool, int, error) {
    var conditions []string
    var args []any
    conditions = append(conditions, "1=1")
    if req.Keyword != "" {
        conditions = append(conditions, "name LIKE ?")
        args = append(args, "%"+req.Keyword+"%")
    }
    if req.Type != "" {
        typeJSON, _ := json.Marshal(req.Type)
        conditions = append(conditions, "JSON_CONTAINS(types, ?)")
        args = append(args, string(typeJSON))
    }
    // COUNT(*) → total
    sortBy, sortOrder := sanitizeSort(req.SortBy, req.SortOrder)
    // SELECT ... WHERE ... ORDER BY {sortBy} {sortOrder} LIMIT ? OFFSET ?
}
```

### 4.2 排序白名单（防 SQL 注入）

```go
func sanitizeSort(sortBy, sortOrder string) (string, string) {
    switch sortBy {
    case "name", "created_at":
    default:
        sortBy = "name"   // 非法字段回退
    }
    switch sortOrder {
    case "asc":
        sortOrder = "ASC"
    case "desc":
        sortOrder = "DESC"
    default:
        sortOrder = "ASC"
    }
    return sortBy, sortOrder
}
```

### 4.3 类型校验与引用计数

```go
// 校验 names 中哪些未注册
func (r *Repository) ValidateTypes(ctx, names []string) ([]string, error)

// 统计引用了某类型名称的条目数（类型删除保护）
func (r *Repository) CountItemsByType(ctx, name string) (int, error) {
    typeJSON, _ := json.Marshal(name)
    // SELECT COUNT(*) FROM services_nav_item WHERE JSON_CONTAINS(types, ?)
}
```

---

## 5. 端口 NULL 的处理

- **存储**：`intranet_port` / `easytier_port` 为 `INT NULL`
- **扫描**：`sql.NullInt64`，`Valid` 为 true 时转 `*int`，否则保持 `nil`
- **更新**：`UpdateItemReq.ClearIntranetPort bool` → true 时置 `nil`；否则取 `IntranetPort *int`

```go
if req.ClearIntranetPort {
    item.IntranetPort = nil
} else if req.IntranetPort != nil {
    item.IntranetPort = req.IntranetPort
}
```

---

## 6. 错误码定义

```go
package servicenav

const (
    // 服务条目错误 (41001-41099)
    ErrItemNotFound     = 41001
    ErrItemNameConflict = 41002
    ErrInvalidTypeRef   = 41003

    // 服务类型错误 (41100-41199)
    ErrTypeNotFound     = 41100
    ErrTypeNameConflict = 41101
    ErrTypeInUse        = 41102

    // 参数错误 (41200-41299)
    ErrParamNameRequired = 41200
)
```