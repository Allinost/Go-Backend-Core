# Goodser 实施计划

## 1. 工作量估算

| 模块 | 文件数 | 预估代码行 | Rust 参考行数 | 难度 |
|------|--------|-----------|-------------|------|
| 数据模型 + DTO (model.go) | 1 | ~200 | 610 | 低 |
| 枚举常量 (model.go) | - | ~50 | 80 | 低 |
| Repository (repository.go) | 1 | ~600 | 800 | 中 |
| Service (service.go) | 1 | ~500 | - | 高 |
| Handler (handler.go) | 1 | ~400 | 1300 | 中 |
| 路由注册 (module.go) | 1 | ~120 | 170 | 低 |
| 错误码 (errors.go) | 1 | ~30 | 60 | 低 |
| 模块入口 (module.go) | 1 | ~60 | - | 低 |
| 图片存储 | 1 | ~100 | 130 | 中 |
| **合计** | **8** | **~2060** | **~3150** | - |

> 注意：Go 代码利用框架现有基础设施（DB 连接池、统一响应、错误码），总行数少于 Rust 版本。

---

## 2. 实施步骤

### Step 1: 基础设施准备（1天）

- [ ] 创建 `zzz/` 目录
- [ ] 创建 `zzz/goodser/` 目录及子包
- [ ] 在 `config.yaml` 添加 `zzz_goodser` 配置段（可选，仅用于模块开关）
- [ ] 确认 MySQL 主库 `go_backend_core` 可用（Goodser 表共用此库）

### Step 2: 数据模型 & Repository（1天）

- [ ] 定义枚举类型（OrderType, OrderStatus, InboundType, WhitelistRole）
- [ ] 定义核心结构体（Inventory, Product, OutboundOrder, InboundLog, Tag, StatusCode, WhitelistEntry）
- [ ] 定义请求/响应 DTO（Create/Update/List 等）
- [ ] 实现 Repository 层（CRUD + 事务操作）

### Step 3: Service 业务逻辑（2天）

- [ ] 库存目录 CRUD
- [ ] 商品 CRUD + 序号分配逻辑
- [ ] 入库（单品/批量/搜索导入）
- [ ] 出库（创建/确认/取消）
- [ ] 预留（创建/取消/转出库）
- [ ] 标签 CRUD
- [ ] 状态码 CRUD

### Step 4: Handler & Routes（1天）

- [ ] 实现所有 Legacy 端点处理器
- [ ] 实现所有 RESTful 端点处理器
- [ ] 实现图片上传/预签名
- [ ] 注册路由

### Step 5: 图片存储集成（0.5天）

- [ ] 使用 `database.DB.RustFS` 或 `database.DB.MinIO` 实现 S3 图片存储
- [ ] 预签名上传 URL
- [ ] 图片访问 URL 生成

### Step 6: 测试（1天）

- [ ] Repository 单元测试（Mock DB）
- [ ] Service 单元测试
- [ ] Handler 集成测试
- [ ] Legacy 兼容性测试（对比 Rust 后端输出）

### Step 7: 集成与部署（0.5天）

- [ ] 注册 Module 到 `main.go`
- [ ] 创建数据库迁移文件 `migrations/mysql/002_zzz_goodser_tables.up.sql`
- [ ] 全流程联调

---

## 3. 数据库迁移

所有表建在现有开发库 `go_backend_core` 中，使用 `zzz_goodser_` 表名前缀。

### 迁移文件: `migrations/mysql/002_zzz_goodser_tables.up.sql`

```sql
-- 库存目录
CREATE TABLE IF NOT EXISTS zzz_goodser_inventories (
    id          VARCHAR(36) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- 商品
CREATE TABLE IF NOT EXISTS zzz_goodser_products (
    id                VARCHAR(36) PRIMARY KEY,
    inventory_id      VARCHAR(36) NOT NULL,
    code              VARCHAR(50) NOT NULL,
    main_zone         CHAR(1) NOT NULL,
    sub_zone          CHAR(1) NOT NULL,
    seq_number        INT NOT NULL,
    quantity          INT NOT NULL DEFAULT 0,
    reserved_quantity INT NOT NULL DEFAULT 0,
    status_code       CHAR(1) NOT NULL DEFAULT 'A',
    name              VARCHAR(500) NOT NULL,
    original_price    DECIMAL(12,2) NOT NULL DEFAULT 0,
    market_price      DECIMAL(12,2) NOT NULL DEFAULT 0,
    expected_price    DECIMAL(12,2) NOT NULL DEFAULT 0,
    remark            TEXT,
    storage_location  VARCHAR(255) DEFAULT '',
    image_url         VARCHAR(1024) DEFAULT '',
    image_list        JSON,
    tags              JSON,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_inventory (inventory_id),
    INDEX idx_zone (inventory_id, main_zone, sub_zone),
    INDEX idx_seq (inventory_id, seq_number),
    INDEX idx_status (status_code),
    FULLTEXT INDEX ft_name (name)
) ENGINE=InnoDB;

-- 序号回收池
CREATE TABLE IF NOT EXISTS zzz_goodser_recycled_seq_numbers (
    id            VARCHAR(36) PRIMARY KEY,
    inventory_id  VARCHAR(36) NOT NULL,
    main_zone     CHAR(1) NOT NULL,
    sub_zone      CHAR(1) NOT NULL,
    seq_number    INT NOT NULL,
    recycled_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_zone_seq (inventory_id, main_zone, sub_zone, seq_number)
) ENGINE=InnoDB;

-- 序号计数器
CREATE TABLE IF NOT EXISTS zzz_goodser_seq_counters (
    id            VARCHAR(36) PRIMARY KEY,
    inventory_id  VARCHAR(36) NOT NULL,
    main_zone     CHAR(1) NOT NULL,
    sub_zone      CHAR(1) NOT NULL,
    current_max   INT NOT NULL DEFAULT 0,
    UNIQUE INDEX idx_unique_zone (inventory_id, main_zone, sub_zone)
) ENGINE=InnoDB;

-- 出库单/预留单
CREATE TABLE IF NOT EXISTS zzz_goodser_outbound_orders (
    id                VARCHAR(36) PRIMARY KEY,
    inventory_id      VARCHAR(36) NOT NULL,
    order_no          VARCHAR(50) NOT NULL,
    type              ENUM('outbound','reserve') NOT NULL DEFAULT 'outbound',
    status            ENUM('pending','reserved','confirmed','cancelled') NOT NULL DEFAULT 'pending',
    order_info        TEXT,
    remark            TEXT,
    items             JSON NOT NULL,
    source_reserve_id VARCHAR(36) DEFAULT NULL,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    confirmed_at      DATETIME DEFAULT NULL,
    cancelled_at      DATETIME DEFAULT NULL,
    INDEX idx_inventory (inventory_id),
    INDEX idx_type (inventory_id, type),
    INDEX idx_status (inventory_id, status),
    INDEX idx_created (created_at)
) ENGINE=InnoDB;

-- 入库日志
CREATE TABLE IF NOT EXISTS zzz_goodser_inbound_logs (
    id            VARCHAR(36) PRIMARY KEY,
    inventory_id  VARCHAR(36) NOT NULL,
    order_no      VARCHAR(50) DEFAULT '',
    type          ENUM('single','batch','search') NOT NULL DEFAULT 'single',
    remark        TEXT,
    items         JSON NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_inventory (inventory_id),
    INDEX idx_created (created_at)
) ENGINE=InnoDB;

-- 状态编码
CREATE TABLE IF NOT EXISTS zzz_goodser_status_codes (
    id            VARCHAR(36) PRIMARY KEY,
    code          CHAR(1) NOT NULL,
    label         VARCHAR(100) NOT NULL,
    is_system     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_code (code)
) ENGINE=InnoDB;

-- 标签
CREATE TABLE IF NOT EXISTS zzz_goodser_tags (
    id            VARCHAR(36) PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    color         VARCHAR(7) NOT NULL DEFAULT '#1890ff',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_name (name)
) ENGINE=InnoDB;

-- 预设数据
INSERT IGNORE INTO zzz_goodser_status_codes (id, code, label, is_system) VALUES
    ('sc_a', 'A', '正常', TRUE),
    ('sc_b', 'B', '预留', TRUE),
    ('sc_c', 'C', '已拆', TRUE),
    ('sc_d', 'D', '损坏', TRUE),
    ('sc_e', 'E', '过期', TRUE),
    ('sc_f', 'F', '停用', TRUE),
    ('sc_n', 'N', '全新', TRUE);
```

### 迁移文件: `migrations/mysql/002_zzz_goodser_tables.down.sql`

```sql
DROP TABLE IF EXISTS zzz_goodser_inventories;
DROP TABLE IF EXISTS zzz_goodser_products;
DROP TABLE IF EXISTS zzz_goodser_recycled_seq_numbers;
DROP TABLE IF EXISTS zzz_goodser_seq_counters;
DROP TABLE IF EXISTS zzz_goodser_outbound_orders;
DROP TABLE IF EXISTS zzz_goodser_inbound_logs;
DROP TABLE IF EXISTS zzz_goodser_status_codes;
DROP TABLE IF EXISTS zzz_goodser_tags;
```

---

## 5. main.go 注册

```go
import (
    "github.com/Allinost/go-backend-core/zzz/goodser"
)

func main() {
    // ... 已有初始化代码

    // 注册 Goodser 业务模块
    modules.Register(&goodser.Module{})

    // ... 其余代码不变
}
```

---

## 6. 与 Rust 后端的差异

| 方面 | Rust (Axum) | Go (Gin) | 备注 |
|------|-------------|----------|------|
| 框架 | Axum 0.7 | Gin 1.12 | 自动路由注册 |
| ORM | sqlx (直接 SQL) | database/sql | 复用 DBManager 连接池 |
| 错误处理 | thiserror + IntoResponse | internal/pkg/errors + response | 统一错误码 |
| 配置 | 环境变量 | config.yaml + Viper | 统一配置管理 |
| 日志 | tracing | zerolog | 框架统一日志 |
| 存储 | aws-sdk-s3 (Rust) | minio-go (Go) | 复用 database 层 |
| 测试 | 内置单元测试 | testify | 框架测试工具 |
| 图片 | 预签名上传 | 预签名上传 | 实现保持一致 |

---

## 7. 注意事项

1. **数据库**: 使用当前开发库 `go_backend_core`，不新建库，通过表名前缀 `zzz_goodser_` 隔离
2. **zzz 前缀**: 所有 Goodser 相关的表名使用 `zzz_goodser_` 前缀
3. **UUID**: 使用标准 UUID v4（通过 `github.com/google/uuid`），与 Rust 的 `uuid` crate 兼容
4. **JSON 字段**: MySQL 的 JSON 类型对应 Go 的 `json.RawMessage`
5. **事务**: 商品入库/出库等操作涉及多表变更，必须使用数据库事务
6. **兼容性**: Legacy 端点必须与 Rust 后端的输入输出格式完全一致
7. **认证**: 当前阶段**不接入认证**，所有接口直接可调用。后续接入 AUTH 模块时只需在路由组添加 `middleware.AuthRequired()` 即可
