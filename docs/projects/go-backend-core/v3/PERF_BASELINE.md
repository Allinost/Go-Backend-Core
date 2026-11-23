# V3 性能基线

## 1. 目标

V3.0 先保证关键列表在 50 行分页下稳定，PM 看板/任务/风险/发布不出现全表扫描。

建议目标（L1 内网、数据量 < 10 万任务）：

| 查询 | p95 目标 |
|---|---|
| 任务列表（project + status + 分页） | < 80ms |
| 看板按状态聚合 | < 60ms |
| 风险列表（project + status） | < 50ms |
| 发布列表（project + released_at） | < 50ms |
| 审计日志（action + 时间倒序） | < 120ms |
| 文件引用按 object_key | < 30ms |

## 2. 执行

```bash
export PGHOST=... PGPORT=... PGUSER=... PGDATABASE=... PGSCHEMA=gobackendcore
./scripts/pg-perf.sh
```

脚本输出 `EXPLAIN (ANALYZE, BUFFERS)`，重点看：

- 是否走 `Index Scan` / `Bitmap Index Scan`
- 是否出现 `Seq Scan` 大表
- `rows` 估算是否严重偏离
- `actual time` 与 `Buffers`

## 3. 关键索引

已在 `005_v3_pm_full.sql` / `004_v3_baseline.sql` 建立：

- `zzz_development_manager_task(project_id, status) WHERE deleted=false`
- `zzz_development_manager_task(milestone_id/iteration_id/requirement_id/parent_id/owner)`
- `zzz_development_manager_risk(project_id, status) WHERE deleted=false`
- `zzz_development_manager_release(project_id, status) WHERE deleted=false`
- `zzz_development_manager_release(released_at DESC)`
- `audit_log(action/user_id/resource/created_at)`
- `file_references(storage_name, object_key)`
- `file_references(module_name, table_name, record_id)`

## 4. 调优原则

1. 先补索引，不改 SQL 语义。
2. 列表一律分页，禁止无 `LIMIT` 全量查询。
3. 软删除表使用 `deleted=false` 部分索引。
4. 聚合查询（看板/周报）优先命中 `project_id + status`。
5. 慢查询 > 200ms 记录到日志，后续用 `pg_stat_statements` 复核。
6. 数据量增长后，再评估分区/物化视图，不在 V3.0 提前优化。

## 5. 记录模板

dev 环境（2026-11-23，61 任务）实测：

| 日期 | 数据量 | 查询 | p50 | p95 | 计划 | 备注 |
|---|---|---|---:|---:|---|---|
| 2026-11-23 | 61 任务 | 任务列表 | 0.06ms | - | Limit → Index Scan | dev 小数据 |
| 2026-11-23 | 61 任务 | 看板聚合 | 0.08ms | - | Aggregate → Index Scan | dev 小数据 |
| 2026-11-23 | 61 任务 | 风险列表 | 0.04ms | - | Limit → Index Scan | dev 小数据 |
| 2026-11-23 | 61 任务 | 发布列表 | 0.05ms | - | Limit → Index Scan | dev 小数据 |
| 2026-11-23 | - | 审计日志 | 0.03ms | - | Limit → Index Scan | dev 小数据 |
| 2026-11-23 | - | 文件引用 | 0.05ms | - | Limit → Index Scan | dev 小数据 |

生产数据量上来后重新执行 `./scripts/pg-perf.sh` 并补 p95。
