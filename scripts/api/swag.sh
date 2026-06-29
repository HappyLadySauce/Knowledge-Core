#!/usr/bin/env bash
# Regenerate Swagger / OpenAPI artifacts consumed by gin-swagger (package api/swagger/docs).
# 重新生成 gin-swagger 使用的 Swagger / OpenAPI 产物（目标包 api/swagger/docs）。
#
# Safe to run from any working directory; resolves repo root from this script path.
# 可在任意当前工作目录执行；根据本脚本路径解析仓库根目录。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

cd "${REPO_ROOT}"

echo "swag: repo root = ${REPO_ROOT}"
echo "swag: output dir = api/swagger/docs"

# -g root.go with -d ./cmd matches swag's expected layout (avoids cmd/cmd/root.go resolution bugs).
# 使用 -g root.go 与 -d ./cmd 符合 swag 预期布局（避免出现 cmd/cmd/root.go 解析错误）。
go run github.com/swaggo/swag/cmd/swag@v1.8.12 init \
  -g root.go \
  -d ./cmd \
  -o api/swagger/docs \
  --parseInternal

echo "swag: done."
