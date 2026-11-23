#!/usr/bin/env bash
# V3 PostgreSQL 恢复演练：把 dump 恢复到临时库，做基本校验后删除临时库。
# 用法：
#   CONFIRM_RESTORE_DRILL=YES ./scripts/pg-restore-drill.sh backups/postgres/xxx.dump
# 必填：PGHOST/PGPORT/PGUSER/PGDATABASE；密码可用 PGPASSWORD 或 ~/.pgpass
set -euo pipefail

: "${PGHOST:?PGHOST required}"
: "${PGPORT:=5432}"
: "${PGUSER:?PGUSER required}"
: "${PGDATABASE:?PGDATABASE required}"
: "${CONFIRM_RESTORE_DRILL:=NO}"

if [[ "$CONFIRM_RESTORE_DRILL" != "YES" ]]; then
  echo "[pg-restore-drill] refused: set CONFIRM_RESTORE_DRILL=YES" >&2
  exit 2
fi

file="${1:-}"
if [[ -z "$file" || ! -f "$file" ]]; then
  echo "[pg-restore-drill] dump file not found: $file" >&2
  exit 2
fi

if [[ -f "$file.sha256" ]] && command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c "$file.sha256"
fi

ts="$(date +%Y%m%d-%H%M%S)"
drill_db="${PGDATABASE}_drill_${ts}"

cleanup() {
  if [[ "${PGDRILL_KEEP:-NO}" == "YES" ]]; then
    echo "[pg-restore-drill] kept temp db: $drill_db"
    return
  fi
  dropdb --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --if-exists "$drill_db" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[pg-restore-drill] create $drill_db"
createdb --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" "$drill_db"

echo "[pg-restore-drill] restore $file"
pg_restore \
  --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$drill_db" \
  --no-owner --no-privileges --exit-on-error \
  "$file"

echo "[pg-restore-drill] verify"
psql --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$drill_db" \
  -v ON_ERROR_STOP=1 -v schema="${PGSCHEMA:-gobackendcore}" <<'SQL'
SET search_path TO :"schema", public;
SELECT current_database() AS database,
       current_schema() AS schema,
       (SELECT count(*) FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_type = 'BASE TABLE') AS tables;
SELECT COALESCE(max(version), 0) AS migration_version FROM _migrations;
SELECT count(*) AS users FROM users;
SQL

echo "[pg-restore-drill] ok: $drill_db"
