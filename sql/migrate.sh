#!/usr/bin/env bash
# Beehive-Blog 数据库迁移入口（Unix shell）
#
# 全覆盖（默认）：MODE=versioned
# 适应：MODE=adaptive  （详见 sql/migrate/main.go 头部注释）
#
# 迁移目录：固定递归 sql/migrations（如 identity/*.sql、attachment/*.sql）。
#
# 用法:
#   ./sql/migrate.sh
#   MODE=adaptive VERBOSE=1 ./sql/migrate.sh
#   DB_DSN='postgres://...' ./sql/migrate.sh
#   MIGRATION_FORCE=1 ./sql/migrate.sh
#     改过迁移 SQL 后与库 checksum 不一致时仍执行并覆盖记录。
#   MIGRATION_REAPPLY=1 MODE=adaptive ./sql/migrate.sh
#     已应用的迁移再执行一遍（多为 DML）。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS="${ROOT}/sql/migrations"
MODE="${MODE:-versioned}"
DSN="${DB_DSN:-postgres://Beehive-Blog:Beehive-Blog@127.0.0.1:5432/Beehive-Blog?sslmode=disable}"

GO_ARGS=(run ./sql/migrate/main.go -dsn "$DSN" -dir "$MIGRATIONS" -catalog "$MIGRATIONS" -mode "$MODE")
if [[ "${VERBOSE:-}" == "1" ]]; then
  GO_ARGS+=(-v)
fi
if [[ "${MIGRATION_FORCE:-}" == "1" ]]; then
  GO_ARGS+=(-force)
fi
if [[ "${MIGRATION_REAPPLY:-}" == "1" ]]; then
  GO_ARGS+=(-reapply)
fi

cd "$ROOT"
exec go "${GO_ARGS[@]}"
