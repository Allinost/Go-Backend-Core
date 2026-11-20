#!/usr/bin/env bash
# V3 PostgreSQL 恢复。危险操作，必须显式 CONFIRM_RESTORE=YES。
# 用法：CONFIRM_RESTORE=YES ./scripts/pg-restore.sh backups/postgres/xxx.dump
set -euo pipefail

: "${PGHOST:?PGHOST required}"
: "${PGPORT:=5432}"
: "${PGUSER:?PGUSER required}"
: "${PGDATABASE:?PGDATABASE required}"
: "${CONFIRM_RESTORE:=NO}"

if [[ "$CONFIRM_RESTORE" != "YES" ]]; then
  echo "[pg-restore] refused: set CONFIRM_RESTORE=YES" >&2
  exit 2
fi

file="${1:-}"
if [[ -z "$file" || ! -f "$file" ]]; then
  echo "[pg-restore] dump file not found: $file" >&2
  exit 2
fi

# 如存在校验和，先校验
if [[ -f "$file.sha256" ]] && command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c "$file.sha256"
fi

echo "[pg-restore] restoring $file -> $PGHOST:$PGPORT/$PGDATABASE"
pg_restore \
  --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$PGDATABASE" \
  --clean --if-exists --no-owner --no-privileges --single-transaction \
  "$file"

echo "[pg-restore] done"
