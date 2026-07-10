# Rust → Go 迁移手册

## 1. 对应关系

将 Rust 后端的每个组件映射到 Go 实现：

| Rust 文件 | Go 位置 | 说明 |
|-----------|---------|------|
| `src/main.rs` | `zzz/goodser/module.go` + `cmd/server/main.go` | 路由注册 + 模块初始化 |
| `src/config.rs` | `internal/config/config.go`（ZzzGoodserConfig） | 框架统一配置管理 |
| `src/error.rs` | `zzz/goodser/errors.go` + 框架 `internal/pkg/errors` | 模块错误码 |
| `src/models/*.rs` | `zzz/goodser/model.go` | 数据模型 + DTO |
| `src/db/mysql.rs` | `zzz/goodser/repository.go` | 数据访问层 |
| `src/handlers/*.rs` | `zzz/goodser/handler.go` | HTTP 处理器 |
| `src/middleware/mod.rs` | `internal/middleware/` | 框架已提供 |
| `src/storage/` | `zzz/goodser/service.go`（图片存储部分） | 对象存储操作 |

---

## 2. 关键代码迁移示例

### 2.1 错误处理

**Rust** (`error.rs`):
```rust
#[derive(Debug, thiserror::Error)]
pub enum AppError {
    #[error("Not found: {0}")]
    NotFound(String),
    #[error("Bad request: {0}")]
    BadRequest(String),
}
```

**Go** (`errors.go`):
```go
package goodser

const (
    ErrInventoryNotFound = 20001
    ErrProductNotFound   = 20002
)

type GoodserError struct {
    Code    int
    Message string
}

func (e *GoodserError) Error() string {
    return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewGoodserError(code int, msg string) *GoodserError {
    return &GoodserError{Code: code, Message: msg}
}
```

### 2.2 数据模型

**Rust** (`models/inventory.rs`):
```rust
#[derive(Debug, Clone, Serialize, Deserialize, sqlx::FromRow)]
pub struct Inventory {
    #[serde(rename(serialize = "_id"))]
    pub id: String,
    pub name: String,
    pub owner_openid: String,
    pub sort_order: i32,
    pub created_at: NaiveDateTime,
    pub updated_at: NaiveDateTime,
}
```

**Go** (`model.go`):
```go
type Inventory struct {
    ID          string    `json:"_id"`
    Name        string    `json:"name"`
    OwnerOpenid string    `json:"owner_openid"`
    SortOrder   int       `json:"sort_order"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

### 2.3 Handler

**Rust** (`handlers/inventory.rs`):
```rust
pub async fn load_inventories(
    Extension(repo): Extension<MysqlRepository>,
) -> Result<Json<Value>, AppError> {
    let inventories = repo.load_inventories().await?;
    Ok(Json(serde_json::to_value(inventories)?))
}
```

**Go** (`handler.go`):
```go
func (h *Handler) LoadInventories(c *gin.Context) {
    inventories, err := h.svc.ListInventories(c.Request.Context())
    if err != nil {
        response.Fail(c, appErr.New(appErr.CodeInternal, err.Error()))
        return
    }
    response.Success(c, inventories)
}
```

### 2.4 Repository

**Rust** (`db/mysql.rs`):
```rust
pub async fn load_inventories(&self) -> Result<Vec<Inventory>, sqlx::Error> {
    sqlx::query_as::<_, Inventory>(
        "SELECT * FROM inventories ORDER BY sort_order"
    )
    .fetch_all(&self.pool)
    .await
}
```

**Go** (`repository.go`):
```go
func (r *Repository) ListInventories(ctx context.Context) ([]Inventory, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, name, owner_openid, sort_order, created_at, updated_at FROM zzz_inventories ORDER BY sort_order")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var items []Inventory
    for rows.Next() {
        var item Inventory
        err := rows.Scan(&item.ID, &item.Name, &item.OwnerOpenid,
            &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
        if err != nil {
            return nil, err
        }
        items = append(items, item)
    }
    return items, rows.Err()
}
```

### 2.5 存储

**Rust** (`storage/mod.rs`):
```rust
#[async_trait]
pub trait ImageStorage: Send + Sync {
    async fn upload(&self, key: &str, data: &[u8]) -> Result<(), AppError>;
    async fn get_url(&self, key: &str) -> Result<String, AppError>;
}
```

**Go** (`service.go` 集成):
```go
type ImageService struct {
    rustfs *rustfs.Client
}

func (s *ImageService) Upload(ctx context.Context, key string, data []byte) error {
    _, err := s.rustfs.Client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
        minio.PutObjectOptions{ContentType: "image/jpeg"})
    return err
}

func (s *ImageService) GetURL(key string) string {
    return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, key)
}
```

---

## 3. 测试迁移

### Rust 测试模式

Rust 使用 DI 和 mock 进行测试隔离：
```rust
fn mock_env(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Result<String, VarError> {
    // 返回闭包模拟 env::var
}
```

### Go 测试模式

Go 使用 `testify` + table-driven tests：
```go
func TestListInventories(t *testing.T) {
    // 使用 mock sql.DB
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    repo := repository.New(db)
    rows := sqlmock.NewRows([]string{"id", "name"}).
        AddRow("inv_001", "仓库1")
    mock.ExpectQuery("SELECT .+ FROM zzz_inventories").
        WillReturnRows(rows)

    items, err := repo.ListInventories(context.Background())
    require.NoError(t, err)
    assert.Len(t, items, 1)
}
```

---

## 4. 数据库差异

| 项目 | Rust SQL | Go SQL | 说明 |
|------|----------|--------|------|
| 数据库名 | `goodser` | `zzz_goodser` | 改名隔离 |
| 表前缀 | 无 | `zzz_` | 加前缀隔离 |
| ID 生成 | `Uuid::new_v4()` | `uuid.New().String()` | 兼容 |
| JSON 处理 | `serde_json::Value` | `json.RawMessage` | `encoding/json` |
| 时间类型 | `NaiveDateTime` | `time.Time` | 自动解析 |
| 连接池 | `sqlx::Pool` | `database/sql.DB` | Go 标准库 |

迁移过程中需特别注意：
1. **表名变更**: 所有查询中的表名需加 `zzz_` 前缀
2. **数据库选择**: 使用 `USE zzz_goodser` 或 DSN 中指定数据库名
3. **事务**: Go 中使用 `tx, _ := db.BeginTx()` 代替 Rust 的 `pool.begin().await`
4. **NULL 处理**: Rust 的 `Option<T>` 对应 Go 的 `*T` 或 `sql.NullType`
5. **JSON**: MySQL JSON 列在 Go 中使用 `json.RawMessage` 读写
