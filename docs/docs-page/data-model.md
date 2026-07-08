---
title: 数据模型
nav_order: 2
---

# 数据模型规划

## 数据库策略

| 数据库 | 用途 | 部署位置 |
|--------|------|----------|
| **MySQL** | **主业务数据**（用户、订单、业务数据） | Docker / 云服务器 |
| Redis | 缓存、会话、队列、排行榜 | Docker / NAS |
| PostgreSQL | NAS 辅助存储、JSONB/高级查询场景 | NAS 设备 |
| RustFS | 文件/图片/对象存储 | 云服务器 |

## MySQL 表规划（主数据库）

### 公共服务表

```sql
-- 用户表
CREATE TABLE users (
    id            BIGINT       AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    phone         VARCHAR(20),
    password_hash VARCHAR(255) NOT NULL,
    nickname      VARCHAR(128),
    avatar_url    TEXT,
    status        TINYINT      NOT NULL DEFAULT 1, -- 1:正常 0:禁用
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 用户设备绑定表（多端登录）
CREATE TABLE user_devices (
    id         BIGINT       AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    platform   VARCHAR(32)  NOT NULL, -- wechat/android/ios/hmos/web/desktop
    device_id  VARCHAR(255) NOT NULL,
    push_token TEXT,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 第三方社交账号绑定表
-- 一个用户可以绑定多个第三方账号（不同平台 / 同平台不同账号均可）
CREATE TABLE social_accounts (
    id              BIGINT       AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT       NOT NULL,
    provider        VARCHAR(32)  NOT NULL, -- wechat/feishu/qq/apple/huawei/honor/google/github/...
    open_id         VARCHAR(255) NOT NULL,
    union_id        VARCHAR(255),          -- 微信 UnionID / 飞书 UnionID
    display_name    VARCHAR(128),          -- 第三方平台昵称（冗余展示）
    access_token    TEXT,
    refresh_token   TEXT,
    token_expiry    TIMESTAMP,
    raw_user_info   JSON,                  -- 平台返回的原始用户信息
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE(provider, open_id),             -- 同一第三方账号只能被一个本地用户绑定
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 若需限制同平台只能绑一个账号，可额外加此唯一约束（默认不启用）
-- UNIQUE(user_id, provider);

CREATE INDEX idx_social_accounts_user_id ON social_accounts(user_id);
CREATE INDEX idx_social_accounts_provider_open ON social_accounts(provider, open_id);

-- 文件记录表
CREATE TABLE files (
    id           BIGINT       AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    filename     VARCHAR(512) NOT NULL,
    size         BIGINT       NOT NULL,
    mime_type    VARCHAR(128),
    storage_key  TEXT         NOT NULL, -- RustFS object key
    storage_url  TEXT         NOT NULL,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 通知记录表
CREATE TABLE notifications (
    id         BIGINT       AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    title      VARCHAR(256) NOT NULL,
    content    TEXT,
    type       VARCHAR(64)  NOT NULL, -- system/marketing/order
    is_read    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 业务模块表（示例）

```sql
-- s1 业务表（按实际需求设计）
CREATE TABLE s1_orders (
    id          BIGINT         AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT         NOT NULL,
    order_no    VARCHAR(64)    NOT NULL UNIQUE,
    amount      DECIMAL(12,2)  NOT NULL,
    status      TINYINT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- s2 业务表（按实际需求设计）
CREATE TABLE s2_products (
    id          BIGINT         AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(256)   NOT NULL,
    description TEXT,
    price       DECIMAL(12,2)  NOT NULL,
    status      TINYINT        NOT NULL DEFAULT 1,
    created_at  TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 数据库 | 小写蛇形 | `go_backend_core` |
| 表名 | 复数蛇形 | `users`, `user_devices` |
| 业务表前缀 | `{模块名}_` | `s1_orders`, `s2_products` |
| 主键 | `id` (BIGINT AUTO_INCREMENT) | - |
| 创建时间 | `created_at` | - |
| 更新时间 | `updated_at` | - |
| 外键 | 显式 REFERENCES | `user_id BIGINT REFERENCES users(id)` |

## Redis Key 设计

```
# 会话
session:{user_id}:{platform} → JWT payload

# 缓存（自动过期）
cache:user:{user_id} → User JSON
cache:config:{key} → value

# 队列
queue:notify:push → asynq task
queue:email:send → asynq task

# 计数器
counter:api:{endpoint}:{date} → integer
counter:user:visit:{user_id}:{date} → integer
```

## 备份记录表

```sql
CREATE TABLE backup_records (
    id              BIGINT       AUTO_INCREMENT PRIMARY KEY,
    task_name       VARCHAR(128) NOT NULL,
    backup_type     VARCHAR(32)  NOT NULL,
    status          VARCHAR(16)  NOT NULL,
    file_path       TEXT,
    file_size       BIGINT,
    checksum        VARCHAR(64),
    source_info     JSON,
    error_message   TEXT,
    started_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at        TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_backup_records_type ON backup_records(backup_type);
CREATE INDEX idx_backup_records_status ON backup_records(status);
CREATE INDEX idx_backup_records_created ON backup_records(created_at DESC);
```

## 数据库迁移管理

使用 `golang-migrate/migrate` 进行版本化迁移，不同于 GORM 的 AutoMigrate，迁移文件是 SQL 脚本，支持回滚。

### 迁移文件存放位置

```
migrations/
├── mysql/                    # MySQL 迁移（主数据库）
│   ├── 000001_create_users_table.up.sql
│   ├── 000001_create_users_table.down.sql
│   ├── 000002_create_social_accounts.up.sql
│   ├── 000002_create_social_accounts.down.sql
│   ├── 000003_create_scheduler_tasks.up.sql
│   └── 000003_create_scheduler_tasks.down.sql
├── postgres/                 # PostgreSQL 迁移（NAS）
│   └── ...
└── redis/                    # Redis 无 schema，跳过
```

### 命名规范

```
{序列号}_{描述}.up.sql      # 正向迁移
{序列号}_{描述}.down.sql    # 回滚
```

- 序列号从 `000001` 开始递增
- 序列号全局唯一，不重复使用
- 已发布的迁移禁止修改（通过新迁移修正）

### GORM 与 migrate 的分工

| 场景 | 工具 | 说明 |
|------|------|------|
| 开发阶段快速迭代 | GORM AutoMigrate | 自动增减列，方便原型 |
| 生产环境发布 | golang-migrate | 版本化，可回滚，记录执行历史 |
| 敏感操作（删表、改类型） | golang-migrate | 手动编写 SQL，review 后执行 |

### 迁移命令

```bash
# 执行迁移（通过 Makefile）
make migrate-up        # 正向迁移到最新
make migrate-down      # 回滚一步
make migrate-to N      # 迁移到指定版本
make migrate-force N   # 强制设置版本（应急用）
```

## GORM 迁移策略

- 开发环境使用 `AutoMigrate` 自动同步
- 生产环境使用版本化 Migration SQL（`golang-migrate/migrate`）
- 每个模块的 Migration 独立编号
- 只增不减，不修改已发布的 Migration
