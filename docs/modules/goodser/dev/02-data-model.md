# Goodser 数据模型

## 1. 数据库

- **数据库**: 使用当前开发库 `go_backend_core`，不新建库
- **表名格式**: `zzz_{模块名}_{表名}`
- **字符集**: `utf8mb4` / `utf8mb4_unicode_ci`
- **引擎**: InnoDB

---

## 2. 表结构

### 2.1 `zzz_goodser_inventories` — 库存目录

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(255) | NOT NULL | 仓库/目录名 |
| owner_openid | VARCHAR(255) | NOT NULL, INDEX | 所属用户 openid |
| sort_order | INT | NOT NULL DEFAULT 0 | 排序序号 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 2.2 `zzz_goodser_products` — 商品

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| inventory_id | VARCHAR(36) | NOT NULL, INDEX | 所属仓库 ID |
| code | VARCHAR(50) | NOT NULL | 五段编码: A-B-0001-0015-A |
| main_zone | CHAR(1) | NOT NULL | 主区编码 |
| sub_zone | CHAR(1) | NOT NULL | 子区编码 |
| seq_number | INT | NOT NULL | 序号 |
| quantity | INT | NOT NULL DEFAULT 0 | 库存数量 |
| reserved_quantity | INT | NOT NULL DEFAULT 0 | 预留数量 |
| status_code | CHAR(1) | NOT NULL DEFAULT 'A' | 状态编码 |
| name | VARCHAR(500) | NOT NULL, FULLTEXT | 商品名称 |
| original_price | DECIMAL(12,2) | DEFAULT 0 | 原价 |
| market_price | DECIMAL(12,2) | DEFAULT 0 | 市场价 |
| expected_price | DECIMAL(12,2) | DEFAULT 0 | 期望价 |
| remark | TEXT | DEFAULT '' | 备注 |
| storage_location | VARCHAR(255) | DEFAULT '' | 存放位置 |
| image_url | VARCHAR(1024) | DEFAULT '' | 主图 URL |
| tags | JSON | NULL | 标签 ID 数组 |
| owner_openid | VARCHAR(255) | NOT NULL | 所属用户 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

索引：
- `idx_inventory` (inventory_id)
- `idx_zone` (inventory_id, main_zone, sub_zone)
- `idx_seq` (inventory_id, seq_number)
- `idx_status` (status_code)
- `ft_name` FULLTEXT (name)

### 2.3 `zzz_goodser_recycled_seq_numbers` — 序号回收池

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| inventory_id | VARCHAR(36) | NOT NULL | 所属仓库 |
| main_zone | CHAR(1) | NOT NULL | 主区 |
| sub_zone | CHAR(1) | NOT NULL | 子区 |
| seq_number | INT | NOT NULL | 回收的序号 |
| recycled_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 回收时间 |

索引：`idx_zone_seq` (inventory_id, main_zone, sub_zone, seq_number)

### 2.4 `zzz_goodser_seq_counters` — 序号计数器

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| inventory_id | VARCHAR(36) | NOT NULL | 所属仓库 |
| main_zone | CHAR(1) | NOT NULL | 主区 |
| sub_zone | CHAR(1) | NOT NULL | 子区 |
| current_max | INT | NOT NULL DEFAULT 0 | 当前最大序号 |

唯一索引：`idx_unique_zone` (inventory_id, main_zone, sub_zone)

### 2.5 `zzz_goodser_outbound_orders` — 出库单/预留单

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| inventory_id | VARCHAR(36) | NOT NULL, INDEX | 所属仓库 |
| order_no | VARCHAR(50) | NOT NULL | 单号 |
| type | ENUM('outbound','reserve') | NOT NULL | 类型 |
| status | ENUM('pending','reserved','confirmed','cancelled') | NOT NULL | 状态 |
| order_info | TEXT | NULL | 订单信息 |
| remark | TEXT | NULL | 备注 |
| items | JSON | NOT NULL | 商品项数组 |
| source_reserve_id | VARCHAR(36) | NULL | 来源预留单 ID |
| owner_openid | VARCHAR(255) | NOT NULL | 所属用户 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |
| confirmed_at | DATETIME | NULL | 确认时间 |
| cancelled_at | DATETIME | NULL | 取消时间 |

items JSON 格式：
```json
[{
  "product_id": "uuid",
  "product_name": "商品名",
  "product_code": "A-B-0001-0015-A",
  "quantity": 5,
  "image_url": "https://..."
}]
```

### 2.6 `zzz_goodser_inbound_logs` — 入库日志

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| inventory_id | VARCHAR(36) | NOT NULL, INDEX | 所属仓库 |
| order_no | VARCHAR(50) | DEFAULT '' | 单号 |
| type | ENUM('single','batch','search') | NOT NULL | 入库类型 |
| remark | TEXT | NULL | 备注 |
| items | JSON | NOT NULL | 商品项数组 |
| owner_openid | VARCHAR(255) | NOT NULL | 所属用户 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

### 2.7 `zzz_goodser_status_codes` — 状态编码

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| code | CHAR(1) | NOT NULL, UNIQUE | 编码 (A-Z) |
| label | VARCHAR(100) | NOT NULL | 标签名 |
| is_system | BOOLEAN | NOT NULL DEFAULT FALSE | 是否系统预设 |
| owner_openid | VARCHAR(255) | NOT NULL | 所属用户 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

系统预设编码：A(正常)、B(预留)、C(已拆)、D(损坏)、E(过期)、F(停用)、N(全新)

### 2.8 `zzz_goodser_tags` — 标签

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(36) | PK | UUID |
| name | VARCHAR(100) | NOT NULL, UNIQUE | 标签名 |
| color | VARCHAR(7) | NOT NULL DEFAULT '#1890ff' | 颜色 HEX |
| owner_openid | VARCHAR(255) | NOT NULL | 所属用户 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

---

## 3. Go 数据模型

### 枚举常量

```go
// OrderType 出库单类型
type OrderType string
const (
    OrderTypeOutbound OrderType = "outbound"
    OrderTypeReserve  OrderType = "reserve"
)

// OrderStatus 出库单状态
type OrderStatus string
const (
    OrderStatusPending   OrderStatus = "pending"
    OrderStatusReserved  OrderStatus = "reserved"
    OrderStatusConfirmed OrderStatus = "confirmed"
    OrderStatusCancelled OrderStatus = "cancelled"
)

// InboundType 入库类型
type InboundType string
const (
    InboundTypeSingle InboundType = "single"
    InboundTypeBatch  InboundType = "batch"
    InboundTypeSearch InboundType = "search"
)

```

### 核心结构体

```go
// Inventory 库存目录
type Inventory struct {
    ID          string    `json:"_id" db:"id"`
    Name        string    `json:"name" db:"name"`
    OwnerOpenid string    `json:"owner_openid" db:"owner_openid"`
    SortOrder   int       `json:"sort_order" db:"sort_order"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Product 商品
type Product struct {
    ID               string          `json:"_id" db:"id"`
    InventoryID      string          `json:"inventory_id" db:"inventory_id"`
    Code             string          `json:"code" db:"code"`
    MainZone         string          `json:"main_zone" db:"main_zone"`
    SubZone          string          `json:"sub_zone" db:"sub_zone"`
    SeqNumber        int             `json:"seq_number" db:"seq_number"`
    Quantity         int             `json:"quantity" db:"quantity"`
    ReservedQuantity int             `json:"reserved_quantity" db:"reserved_quantity"`
    StatusCode       string          `json:"status_code" db:"status_code"`
    Name             string          `json:"name" db:"name"`
    OriginalPrice    float64         `json:"original_price,omitempty" db:"original_price"`
    MarketPrice      float64         `json:"market_price,omitempty" db:"market_price"`
    ExpectedPrice    float64         `json:"expected_price,omitempty" db:"expected_price"`
    Remark           string          `json:"remark,omitempty" db:"remark"`
    StorageLocation  string          `json:"storage_location,omitempty" db:"storage_location"`
    ImageURL         *string         `json:"image_url" db:"image_url"`
    Tags             json.RawMessage `json:"tags,omitempty" db:"tags"`
    CreatedAt        time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}

// OutboundOrder 出库单
type OutboundOrder struct {
    ID              string          `json:"_id" db:"id"`
    InventoryID     string          `json:"inventory_id" db:"inventory_id"`
    OrderNo         string          `json:"order_no" db:"order_no"`
    Type            string          `json:"type" db:"type"`
    Status          string          `json:"status" db:"status"`
    OrderInfo       *string         `json:"order_info" db:"order_info"`
    Remark          *string         `json:"remark" db:"remark"`
    Items           json.RawMessage `json:"items" db:"items"`
    SourceReserveID *string         `json:"source_reserve_id" db:"source_reserve_id"`
    OwnerOpenid     string          `json:"owner_openid" db:"owner_openid"`
    CreatedAt       time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
    ConfirmedAt     *time.Time      `json:"confirmed_at" db:"confirmed_at"`
    CancelledAt     *time.Time      `json:"cancelled_at" db:"cancelled_at"`
}

// InboundLog 入库日志
type InboundLog struct {
    ID          string          `json:"_id" db:"id"`
    InventoryID string          `json:"inventory_id" db:"inventory_id"`
    OrderNo     *string         `json:"order_no" db:"order_no"`
    Type        string          `json:"type" db:"type"`
    Remark      *string         `json:"remark" db:"remark"`
    Items       json.RawMessage `json:"items" db:"items"`
    OwnerOpenid string          `json:"owner_openid" db:"owner_openid"`
    CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// Tag 标签
type Tag struct {
    ID          string    `json:"_id" db:"id"`
    Name        string    `json:"name" db:"name"`
    Color       string    `json:"color" db:"color"`
    OwnerOpenid string    `json:"owner_openid" db:"owner_openid"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// StatusCode 状态编码
type StatusCode struct {
    ID          string    `json:"_id" db:"id"`
    Code        string    `json:"code" db:"code"`
    Label       string    `json:"label" db:"label"`
    IsSystem    bool      `json:"is_system" db:"is_system"`
    OwnerOpenid string    `json:"owner_openid" db:"owner_openid"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

```

---

## 4. Go <-> Rust 类型映射

| Go | Rust | 说明 |
|----|------|------|
| `string` | `String` | UUID/字符串 |
| `int` | `i32` | 整数 |
| `float64` | `f64` | 价格 |
| `bool` | `bool` | 布尔 |
| `*string` | `Option<String>` | 可选字符串 |
| `*time.Time` | `Option<NaiveDateTime>` | 可选时间 |
| `json.RawMessage` | `serde_json::Value` | JSON 字段 |
| `[]T` | `Vec<T>` | 数组 |
| `string (enum)` | `enum (as_str/from_str)` | 枚举用字符串 |
