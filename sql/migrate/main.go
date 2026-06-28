// Package main provides the SQLite migration CLI for Knowledge Core.
// Package main 提供 Knowledge Core 的 SQLite 迁移命令行工具。
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const defaultDBPath = ".knowledge-core/index.db"

type migrationFile struct {
	Version  string
	Path     string
	Body     string
	Checksum string
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
		os.Exit(1)
	}
}

func run() (err error) {
	var (
		dbPath        = flag.String("db", envOrDefault("KNOWLEDGE_CORE_SQLITE_PATH", defaultDBPath), "SQLite database file path")
		migrationsDir = flag.String("dir", "sql/migrations", "migration SQL directory")
		force         = flag.Bool("force", false, "re-apply a migration when checksum changed")
	)
	flag.Parse()

	files, err := listMigrationFiles(*migrationsDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("no migration files found, skipped")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := openSQLite(ctx, *dbPath, 5000)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}
	for _, mf := range files {
		applied, err := isApplied(ctx, db, mf.Version, mf.Checksum, *force)
		if err != nil {
			return err
		}
		if applied {
			fmt.Printf("skip %s\n", mf.Version)
			continue
		}
		if err := applyMigration(ctx, db, mf); err != nil {
			return err
		}
		fmt.Printf("applied %s\n", mf.Version)
	}

	fmt.Println("migrations completed")
	return nil
}

func openSQLite(ctx context.Context, path string, busyTimeoutMS int) (*sql.DB, error) {
	dbPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	values := url.Values{}
	values.Set("_pragma", "busy_timeout("+strconv.Itoa(busyTimeoutMS)+")")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?"+values.Encode())
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`)
	return err
}

func applyMigration(ctx context.Context, db *sql.DB, mf migrationFile) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, mf.Body); err != nil {
		return fmt.Errorf("apply %s failed: %w", mf.Version, err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)
ON CONFLICT(version) DO UPDATE SET
    checksum = excluded.checksum,
    applied_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`, mf.Version, mf.Checksum); err != nil {
		return fmt.Errorf("record %s failed: %w", mf.Version, err)
	}
	return tx.Commit()
}

func listMigrationFiles(dir string) ([]migrationFile, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("migration directory not found %s: %w", absDir, err)
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") && len(name) >= 5 && name[0] >= '0' && name[0] <= '9' {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	files := make([]migrationFile, 0, len(names))
	for _, name := range names {
		path := filepath.Join(absDir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, migrationFile{
			Version:  strings.TrimSuffix(name, filepath.Ext(name)),
			Path:     path,
			Body:     string(body),
			Checksum: sha256Hex(body),
		})
	}
	return files, nil
}

func isApplied(ctx context.Context, db *sql.DB, version, checksum string, force bool) (bool, error) {
	var existing string
	err := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", version).Scan(&existing)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if existing != checksum {
		if force {
			fmt.Fprintf(os.Stderr, "warning: %s checksum changed, re-applying\n", version)
			return false, nil
		}
		return false, fmt.Errorf("migration %s checksum mismatch; use -force to re-apply", version)
	}
	return true, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
