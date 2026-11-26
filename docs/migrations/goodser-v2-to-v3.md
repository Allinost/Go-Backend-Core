# Goodser V2 MySQL → V3 PostgreSQL 迁移 Runbook

> 适用：`cmd/v3migrate` + `migrations/postgres/008_goodser.sql`
> 前置：V3 已应用 PG 迁移 008/009；`.env` 同时配置 V2 MySQL 与 V3 PostgreSQL main。

---

## 1. 前置检查

```bash
cd Go-Backend-Core

# V2 MySQL（源）
#   APP_DATABASE_MYSQL_MAIN_HOST/PORT/USER/PASSWORD/DBNAME
# V3 PostgreSQL（目标）
#   APP_DATABASE_POSTGRES_MAIN_*

go run ./cmd/v3migrate plan -scope goodser
```

`plan` 会列出 allowlist 中每张表的 V2/V3 行数。Goodser 的 19 张表在清单末尾：

```text
zzz_goodser_inventories
zzz_goodser_status_codes
zzz_goodser_tags
zzz_goodser_dictionaries
zzz_goodser_seq_counters
zzz_goodser_recycled_seq_numbers
zzz_goodser_products
zzz_goodser_product_characters
zzz_goodser_combos
zzz_goodser_combo_items
zzz_goodser_bundles
zzz_goodser_bundle_main_products
zzz_goodser_bundle_sub_products
zzz_goodser_bundle_pools
zzz_goodser_bundle_pool_items
zzz_goodser_inbound_logs
zzz_goodser_outbound_orders
zzz_goodser_returns
zzz_goodser_stock_movements
```

> 表名 V2 与 V3 一致，无需映射。V2 不存在的表会显示 `(V2 不存在)` 并在导出时自动跳过。
>
> `-scope goodser` 只处理这 19 张表，不会触碰 users / projects 等其他模块的表；
> 后续做整库迁移时不传 `-scope`（默认 `all`）即可。

---

## 2. 冻结写入并导出

维护窗口内停止 V2 Goodser 写入（Web / Android / 小程序），然后：

```bash
go run ./cmd/v3migrate dump -scope goodser -file backups/goodser.json
```

输出示例：

```text
导出完成：tables=19 rows=12345 file=backups/goodser.json
```

> dump 为单文件 JSON，包含 allowlist 内所有已存在的表；本文件含业务数据，不要提交 Git。

---

## 3. 演练（推荐先做）

在独立 schema 中建表、导入、校验，不影响生产库：

```bash
go run ./cmd/v3migrate rehearsal \
  -scope goodser \
  -file backups/goodser.json \
  -schema v3_rehearsal_goodser
```

- 每次会 `DROP SCHEMA IF EXISTS v3_rehearsal_goodser CASCADE` 后重建，可反复执行。
- 加 `-keep` 可保留 schema 供人工核对。
- schema 名必须以 `v3_rehearsal` 开头。

预期结尾：

```text
校验通过：allowlist 行数、序列、外键约束、任务工时均正常
```

---

## 4. 正式导入

```bash
go run ./cmd/v3migrate load -scope goodser -file backups/goodser.json
go run ./cmd/v3migrate verify -scope goodser
```

`load` 行为：

- 按依赖顺序插入；`ON CONFLICT DO NOTHING`，重复执行不会产生重复行（但也不会覆盖已存在行）。
- 显式 ID 插入后自动重置 identity 序列（`stock_movements`、`combo_items`、`bundle_*`）。
- 时区：V2 MySQL `DATETIME` 按 `-source-tz` 解释后再写入 `timestamptz`。默认 `Asia/Shanghai`，与 V2 服务器一致；若 V2 实际存 UTC，用：
  ```bash
  go run ./cmd/v3migrate load -file backups/goodser.json -source-tz UTC
  ```
- 图片：`/goodser/images/` 下的预签名 URL 会去掉 `?...` 签名参数，与运行时 `normalizeImageURL` 规则一致。

---

## 5. 校验内容

`verify` 除行数外，还会检查 Goodser 业务不变量：

| 检查 | 说明 |
|---|---|
| 行数 | V2 与 V3 每张表一致 |
| identity 序列 | `last_value >= max(id)` |
| 商品负库存 | `quantity < 0 OR reserved_quantity < 0` |
| 流水负数量 | `quantity_after < 0 OR reserved_after < 0` |
| jsonb 结构 | `tags` / `images` / `items` 必须是数组 |
| 图片签名 | `image_url` 不应残留 `/goodser/images/...?...` |
| 未验证外键 | `pg_constraint.convalidated` |

任何一项非 0/失败都会返回非零退出码。

---

## 6. 启用 V3 Goodser

数据校验通过后：

```env
APP_MODULES_GOODSER_ENABLED=true
```

或在 `config.yaml`：

```yaml
modules_goodser_enabled: true
```

重启后端，确认：

```bash
curl -s http://127.0.0.1:29090/api/v1/health/modules
curl -s http://127.0.0.1:29090/api/v1/zzz-goodser/inventories \
  -H "Authorization: Bearer <Token>"
```

> `APP_MODULES_GOODSER_ENABLED` 与 `APP_LEGACY_MODULES_ENABLED` 解耦。
> 后者只控制 shopchecker / login3rd / ia 等仍未迁移的 V2 MySQL 模块。

---

## 7. 回滚

- 7 天内保留 V2 MySQL 快照与 V2 服务只读实例。
- 关闭 V3 Goodser 路由：
  ```env
  APP_MODULES_GOODSER_ENABLED=false
  ```
  重启后端即可；V3 PG 中的数据不影响其他模块。
- 客户端切回 V2 地址，目标 ≤ 30 分钟。
- 若需清空 V3 Goodser 数据重来（仅在确认无新增数据时）：
  ```sql
  TRUNCATE zzz_goodser_bundle_pool_items, zzz_goodser_bundle_pools,
           zzz_goodser_bundle_sub_products, zzz_goodser_bundle_main_products,
           zzz_goodser_bundles, zzz_goodser_combo_items, zzz_goodser_combos,
           zzz_goodser_product_characters, zzz_goodser_stock_movements,
           zzz_goodser_returns, zzz_goodser_outbound_orders,
           zzz_goodser_inbound_logs, zzz_goodser_products,
           zzz_goodser_recycled_seq_numbers, zzz_goodser_seq_counters,
           zzz_goodser_dictionaries, zzz_goodser_tags,
           zzz_goodser_status_codes, zzz_goodser_inventories
  CASCADE;
  ```

---

## 8. 已知边界

- `dump` 读取的是 `SELECT *`，V2 新增列若 V3 不存在会自动忽略。
- V3 新增列（如 `combo_id`、`bundle_snapshot`）在 V2 无值时为 `NULL` 或默认值。
- `owner_user_id` 首期为 `NULL`（单 owner）；后续按用户隔离时再回填。
- 迁移期间不要同时写 V2 和 V3，避免行数校验不一致。
- 图片本体不迁移：只迁移 RustFS URL，确认 V2 与 V3 指向同一桶。
