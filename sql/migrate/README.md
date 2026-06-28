# SQLite Migration

Migration CLI lives in [`main.go`](main.go). Entry scripts: [`migrate.ps1`](../migrate.ps1) on Windows and [`migrate.sh`](../migrate.sh) on Unix.

## Common Commands

```powershell
.\sql\migrate.ps1
.\sql\migrate.ps1 -Db '.knowledge-core/index.db'
.\sql\migrate.ps1 -Force
```

```bash
./sql/migrate.sh
KNOWLEDGE_CORE_SQLITE_PATH=.knowledge-core/index.db ./sql/migrate.sh
MIGRATION_FORCE=1 ./sql/migrate.sh
```

## Behavior

The migrator scans top-level numeric `.sql` files under `sql/migrations`, applies them in lexical order, and records `version`, `checksum`, and `applied_at` in `schema_migrations`.
