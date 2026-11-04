#!/usr/bin/env bash
# 将 GHCR 容器包设为 public。
# 需要 GH_TOKEN：带 write:packages 的 PAT，且账号对组织包有 admin 权限。
# 用法：GH_TOKEN=ghp_xxx ./scripts/ghcr-public.sh go-backend-core go-frontend-core
set -euo pipefail

: "${GH_TOKEN:?请设置 GH_TOKEN（需要 write:packages）}"
org="${GHCR_ORG:-Allinost}"

if [ "$#" -eq 0 ]; then
  set -- go-backend-core go-frontend-core
fi

for pkg in "$@"; do
  code=$(curl -sS -o /tmp/ghcr-public-"$pkg".json -w '%{http_code}' \
    -X PATCH \
    -H "Authorization: Bearer $GH_TOKEN" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "https://api.github.com/orgs/$org/packages/container/$pkg" \
    -d '{"visibility":"public"}')
  if [ "$code" = "200" ]; then
    echo "$org/$pkg -> public"
  else
    echo "$org/$pkg -> HTTP $code"
    cat /tmp/ghcr-public-"$pkg".json
    exit 1
  fi
done
