#!/bin/bash
# 运行单元测试
# 用法: ./scripts/test.sh [包路径] [-v]

set -euo pipefail

PKG="${1:-./...}"
shift 2>/dev/null || true

echo "==> 运行测试: $PKG $@"
go test "$@" -count=1 -race -cover "$PKG"
