#!/usr/bin/env bash
# Knowledge-Core SQLite migration entrypoint for Unix shell.
# Knowledge-Core SQLite 数据库迁移入口（Unix shell）。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS="${ROOT}/sql/migrations"
DB_PATH="${KNOWLEDGE_CORE_SQLITE_PATH:-.knowledge-core/index.db}"

GO_ARGS=(run ./sql/migrate/main.go -db "$DB_PATH" -dir "$MIGRATIONS")
if [[ "${MIGRATION_FORCE:-}" == "1" ]]; then
  GO_ARGS+=(-force)
fi

cd "$ROOT"
exec go "${GO_ARGS[@]}"
