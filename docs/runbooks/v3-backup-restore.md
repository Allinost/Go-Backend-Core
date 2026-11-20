# V3 PostgreSQL 备份与恢复 Runbook

## 目标

- 每日冷备份 V3 数据库 `GoBackendCoreDatabase`（L1）。
- 备份失败告警，保留 14 天。
- 每季度至少一次恢复演练（在 rehearsal schema 或临时库）。
- 维护切换前必须手动备份。

## 环境变量

```bash
export PGHOST=10.144.144.10
export PGPORT=58432
export PGUSER=GoBackendCoreDatabase
export PGDATABASE=GoBackendCoreDatabase
export PGPASSWORD='...'
export BACKUP_DIR=/data/backups/postgres
export RETENTION_DAYS=14
```

## 备份

```bash
./scripts/pg-backup.sh
```

输出：

```
$BACKUP_DIR/GoBackendCoreDatabase-YYYYmmdd-HHMMSS.dump
$BACKUP_DIR/...dump.sha256
```

建议 cron：

```cron
15 3 * * * cd /opt/go-backend-core && . ./.env && ./scripts/pg-backup.sh >> logs/pg-backup.log 2>&1
```

## 恢复

危险操作，必须显式确认：

```bash
CONFIRM_RESTORE=YES ./scripts/pg-restore.sh /data/backups/postgres/xxx.dump
```

- 恢复会 `--clean --if-exists`，覆盖同名对象。
- 生产恢复前先停 V3 应用，避免连接占用。
- 恢复后执行：
  - `POST /api/v1/migrate/schema/migrate` 确认迁移版本
  - `go run ./cmd/v3migrate verify` 或业务冒烟
  - 健康检查 `/api/v1/health/health`

## 演练

1. 在临时 schema 或临时库执行恢复。
2. 对比关键表行数：`v3migrate plan` / `report`。
3. 记录演练时间、备份文件、耗时、异常。
4. 演练记录归档到项目台账。

## 与维护模式的关系

- `migrate_return`：开始维护前备份 L1；结束维护前完成 L2→L1 反向同步。
- `migrate_no_return`：开始维护前备份 L1；L2 成为主库后重新配置备份任务。
- `shutdown`：短停机可不恢复，但必须保留维护前备份。
