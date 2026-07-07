#!/bin/bash
# 手动触发 release：打 tag 并推送到 GitHub，触发 CI 流水线
# 用法: ./scripts/ci.sh v0.1.0
# 前提: 当前分支已提交所有变更，且已关联 GitHub 远程仓库

set -euo pipefail

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "用法: $0 <版本号>"
  echo "示例: $0 v0.2.0"
  exit 1
fi

# 校验版本号格式
if ! echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "错误: 版本号格式必须为 vMAJOR.MINOR.PATCH（如 v0.2.0）"
  exit 1
fi

# 检查工作区是否干净
if ! git diff --stat --exit-code; then
  echo "错误: 工作区有未提交的变更，请先提交"
  exit 1
fi

# 检查是否在 main 分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "警告: 当前不在 main 分支 ($CURRENT_BRANCH)，将在当前分支打 tag"
fi

echo "==> 创建 tag: $VERSION"
git tag -a "$VERSION" -m "Release $VERSION"

echo "==> 推送 tag 到 GitHub"
git push origin "$VERSION"

echo ""
echo "✅ Release $VERSION 已触发"
echo "    查看进度: https://github.com/Allinost/go-backend-core/actions"
echo "    Docker 镜像: ghcr.io/Allinost/go-backend-core:$VERSION"
