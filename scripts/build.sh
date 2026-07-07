#!/bin/bash
# 构建脚本：debug 带调试符号，release 剥离体积
# 用法: ./scripts/build.sh debug|release

set -euo pipefail

MODE="${1:-debug}"
OUTPUT="${2:-server}"
GOOS="${GOOS:-}"
GOARCH="${GOARCH:-}"

# 检测默认 GOOS
if [ -z "$GOOS" ]; then
  GOOS=$(go env GOOS)
fi
if [ -z "$GOARCH" ]; then
  GOARCH=$(go env GOARCH)
fi

echo "==> 构建模式: $MODE"
echo "==> 目标平台: $GOOS/$GOARCH"
echo "==> 输出文件: $OUTPUT"

LD_FLAGS="-s -w"
GC_FLAGS=""

if [ "$MODE" = "debug" ]; then
  LD_FLAGS=""                                    # 保留调试符号
  GC_FLAGS="-N -l"                               # 禁用内联和优化（dlv 调试用）
  echo "==> 构建 DEBUG 版本"
elif [ "$MODE" = "release" ]; then
  LD_FLAGS="-s -w"                               # 剥离调试信息
  echo "==> 构建 RELEASE 版本"
else
  echo "错误: 模式必须为 debug 或 release"
  exit 1
fi

GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
  -ldflags="$LD_FLAGS" \
  -gcflags="$GC_FLAGS" \
  -o "$OUTPUT" \
  ./cmd/server/

echo "==> 构建完成: $(ls -lh "$OUTPUT" | awk '{print $5}')"
