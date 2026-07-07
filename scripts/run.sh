#!/bin/bash
# 本地运行（先构建再启动）
# 用法: ./scripts/run.sh [debug|release]

set -euo pipefail

MODE="${1:-debug}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# 构建
echo "==> 编译 ($MODE)..."
bash "$SCRIPT_DIR/build.sh" "$MODE" "server"

# 运行
echo "==> 启动服务..."
./server
