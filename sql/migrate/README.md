# Database migration

Migration CLI lives in [`main.go`](main.go). Entry scripts: [`migrate.ps1`](../migrate.ps1) (Windows) and [`migrate.sh`](../migrate.sh) (Unix).

## Modes

| Mode | Behavior |
|------|----------|
| **versioned** (default) | Each `.sql` file runs in one transaction; `schema_migrations` stores path + SHA-256 checksum. Best for empty databases or strict alignment with the repo. |
| **adaptive** | Splits a file by `;` and skips "already exists" SQLSTATEs (`42P07`, `42701`, `42710`). Use when a dev DB already has partial schema. |

## Common commands

From the repository root:

```powershell
.\sql\migrate.ps1
.\sql\migrate.ps1 -Mode adaptive -Verbose
.\sql\migrate.ps1 -Dsn 'postgres://user:pass@host:5432/dbname?sslmode=disable'
.\sql\migrate.ps1 -Force    # Re-apply when file checksum changed; updates schema_migrations
.\sql\migrate.ps1 -Reapply -Mode adaptive
```

Environment variable `DB_DSN` is used when `-Dsn` is omitted.

## After migration file renumber / squash

When migration files are merged or renamed (e.g. `019_content_text_search.sql` folded into `content/009_content_contents.sql`), existing databases may have stale `schema_migrations.version` rows or checksum mismatches.

| Scenario | Recommended action |
|----------|-------------------|
| **Empty DB / data can be dropped** | Drop and recreate the database, then run `.\sql\migrate.ps1` (versioned). |
| **Dev DB, schema mostly correct** | `.\sql\migrate.ps1 -Mode adaptive`; if checksum errors appear, `.\sql\migrate.ps1 -Force` (backs up recommended). |
| **Orphan old version rows, duplicate DDL noise** | After backup, remove obsolete rows, e.g. `DELETE FROM schema_migrations WHERE version LIKE '%019_content_text_search%';`, then `-Mode adaptive`. Do not delete rows for files that still exist in `sql/migrations/`. |

Fresh installs only need the current files under `sql/migrations/<domain>/` sorted by numeric prefix (`000_` … `008_`).

### Post-migration verification SQL

```sql
-- Owner FK on attachments (added in identity/004)
SELECT 1 FROM pg_constraint
WHERE conname = 'fk_attachment_attachments_owner_user'
  AND conrelid = 'attachment.attachments'::regclass;

-- Trigram search indexes (in content/009)
SELECT indexname FROM pg_indexes
WHERE schemaname = 'content' AND tablename = 'contents'
  AND indexname = 'idx_content_contents_title_trgm';

-- Version history table
SELECT 1 FROM information_schema.tables
WHERE table_schema = 'content' AND table_name = 'content_versions';
```

## Layout

- `sql/migrations/attachment/` — storage drivers, mounts, attachments, categories
- `sql/migrations/content/` — contents, versions, relations, tags, categories
- `sql/migrations/identity/` — users, credentials, sessions, identities
- `sql/migrations/setting/` — application settings

See also [CLAUDE.md](../../CLAUDE.md) for application-level migration notes.
