#!/usr/bin/env bash
# 模块开发 SOP 自检：gofmt / 建表方式 / 模块文档 / vet / 短测试
# 用法: bash scripts/sop-check.sh

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

fail=0

echo "==> [1/5] gofmt"
bad=$(gofmt -l zzz internal cmd 2>/dev/null | grep -v "docs.go" || true)
if [ -n "$bad" ]; then
  echo "$bad"
  fail=1
else
  echo "ok"
fi

echo "==> [2/5] 禁止模块内 initTables 建表"
hits=$(grep -rn "func initTables" --include="*.go" zzz internal/modules 2>/dev/null | grep -v "ops" || true)
if [ -n "$hits" ]; then
  echo "$hits"
  fail=1
else
  echo "ok"
fi

echo "==> [3/5] 模块文档索引"
missing=0
for dir in docs/modules/*/; do
  [ -d "$dir" ] || continue
  if [ -f "$dir/index.md" ] || [ -f "$dir/dev/index.md" ]; then
    continue
  fi
  echo "missing index: $dir"
  missing=1
done
if [ "$missing" -eq 0 ]; then
  echo "ok"
else
  fail=1
fi

echo "==> [4/5] go vet"
if go vet ./zzz/... ./internal/modules/...; then
  echo "ok"
else
  fail=1
fi

echo "==> [5/5] go test -short"
if go test -short -count=1 ./zzz/... ./internal/modules/...; then
  echo "ok"
else
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "SOP check FAILED"
  exit 1
fi
echo "SOP check PASSED"
