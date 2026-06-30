#!/usr/bin/env bash
# Knowledge-Core PostgreSQL migration entrypoint for Unix shell.
# Knowledge-Core PostgreSQL 数据库迁移入口（Unix shell）。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS="${ROOT}/sql/migrations"
DATABASE_URL="${KNOWLEDGE_CORE_DATABASE_URL:-postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable}"

GO_ARGS=(run ./sql/migrate/main.go -database-url "$DATABASE_URL" -dir "$MIGRATIONS")
if [[ "${MIGRATION_FORCE:-}" == "1" ]]; then
  GO_ARGS+=(-force)
fi

cd "$ROOT"
exec go "${GO_ARGS[@]}"
