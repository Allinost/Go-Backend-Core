#!/bin/bash
# 清理构建产物
# 用法: ./scripts/clean.sh

set -euo pipefail

echo "==> 清理构建产物..."
rm -f server server.debug
rm -rf tmp/

echo "==> 清理完成"
