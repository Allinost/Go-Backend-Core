#!/bin/bash
# 运行 golangci-lint
# 用法: ./scripts/lint.sh

set -euo pipefail

echo "==> 代码检查..."
golangci-lint run ./...
