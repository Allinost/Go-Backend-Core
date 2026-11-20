#!/usr/bin/env bash
# V3 PostgreSQL 冷备份。默认自定义格式，便于 pg_restore。
# 必填：PGHOST/PGPORT/PGUSER/PGDATABASE；密码可用 PGPASSWORD 或 ~/.pgpass。
set -euo pipefail

: "${PGHOST:?PGHOST required}"
: "${PGPORT:=5432}"
: "${PGUSER:?PGUSER required}"
: "${PGDATABASE:?PGDATABASE required}"
: "${BACKUP_DIR:=./backups/postgres}"
: "${RETENTION_DAYS:=14}"

mkdir -p "$BACKUP_DIR"
ts="$(date +%Y%m%d-%H%M%S)"
file="$BACKUP_DIR/${PGDATABASE}-${ts}.dump"

echo "[pg-backup] host=$PGHOST port=$PGPORT db=$PGDATABASE"
pg_dump \
  --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$PGDATABASE" \
  --format=custom --compress=9 --no-owner --no-privileges \
  --file="$file"

# 生成校验和，恢复前可对比
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$file" > "$file.sha256"
fi

echo "[pg-backup] written $file"
find "$BACKUP_DIR" -type f -name '*.dump' -mtime "+$RETENTION_DAYS" -print -delete
find "$BACKUP_DIR" -type f -name '*.dump.sha256' -mtime "+$RETENTION_DAYS" -print -delete
echo "[pg-backup] retention=${RETENTION_DAYS}d ok"
