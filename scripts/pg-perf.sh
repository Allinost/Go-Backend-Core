#!/usr/bin/env bash
# V3 PostgreSQL 关键查询性能基线。输出 EXPLAIN (ANALYZE, BUFFERS)。
# 用法：./scripts/pg-perf.sh
# 必填：PGHOST/PGPORT/PGUSER/PGDATABASE；可用 PGSCHEMA 指定 schema（默认 gobackendcore）
set -euo pipefail

: "${PGHOST:?PGHOST required}"
: "${PGPORT:=5432}"
: "${PGUSER:?PGUSER required}"
: "${PGDATABASE:?PGDATABASE required}"
: "${PGSCHEMA:=gobackendcore}"

run() {
  local title="$1"
  local sql="$2"
  echo ""
  echo "==> $title"
  psql --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$PGDATABASE" \
    -v ON_ERROR_STOP=1 -c "SET search_path TO \"$PGSCHEMA\", public; EXPLAIN (ANALYZE, BUFFERS) $sql"
}

run "任务列表：project + status + 分页" \
  "SELECT id, code, title, status, priority, owner, due_date FROM zzz_development_manager_task WHERE project_id = 1 AND deleted = false AND status = 'doing' ORDER BY order_no, id LIMIT 50"

run "项目看板：按状态分组" \
  "SELECT status, count(*) FROM zzz_development_manager_task WHERE project_id = 1 AND deleted = false GROUP BY status"

run "风险列表：project + status" \
  "SELECT id, title, level, status FROM zzz_development_manager_risk WHERE project_id = 1 AND deleted = false AND status IN ('open','mitigating') ORDER BY id DESC LIMIT 50"

run "发布列表：project + status" \
  "SELECT id, version, name, status, released_at FROM zzz_development_manager_release WHERE project_id = 1 AND deleted = false ORDER BY released_at DESC NULLS LAST LIMIT 50"

run "审计日志：action + 时间倒序" \
  "SELECT id, action, username, status, created_at FROM audit_log WHERE action = 'settings.config' ORDER BY created_at DESC LIMIT 50"

run "文件引用：storage + object_key" \
  "SELECT id, storage_name, object_key, module_name FROM file_references WHERE storage_name = 'rustfs' AND object_key = 'demo' LIMIT 10"

echo ""
echo "[pg-perf] done"
