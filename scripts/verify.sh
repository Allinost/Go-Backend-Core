#!/usr/bin/env bash
# 本地/CI 通用验证：格式、构建、vet、short 测试。
# 设置 TEST_DATABASE_URL 后可额外跑 PostgreSQL 集成测试：
#   TEST_DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=disable' ./scripts/verify.sh
set -euo pipefail

echo "==> gofmt"
out="$(gofmt -l .)"
if [[ -n "$out" ]]; then
  echo "$out"
  exit 1
fi

echo "==> go build"
go build ./...

echo "==> go vet"
go vet ./...

echo "==> go test -short"
go test -short -count=1 ./...

if [[ -n "${TEST_DATABASE_URL:-}" ]]; then
  echo "==> PostgreSQL integration tests"
  go test -count=1 -v -run 'TestApplyMigrations|TestPostgres|TestRepository|TestPMRepository' \
    ./internal/testutil ./internal/services/audit ./internal/services/feature \
    ./internal/services/auth ./internal/services/outbox ./internal/modules/webhook \
    ./internal/modules/servicenav ./zzz/developmentManager
fi

echo "==> verify ok"
