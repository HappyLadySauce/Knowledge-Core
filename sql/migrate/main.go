// Package main provides the PostgreSQL migration CLI for Knowledge Core.
// Package main 提供 Knowledge Core 的 PostgreSQL 迁移命令行工具。
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultDatabaseURL = "postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable"
	migrationLockID    = int64(0x4b435f6d696772)
)

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
		databaseURL   = flag.String("database-url", envOrDefault("KNOWLEDGE_CORE_DATABASE_URL", defaultDatabaseURL), "PostgreSQL database URL")
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

	db, err := openPostgres(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if err := withMigrationLock(ctx, db, func() error {
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
		return nil
	}); err != nil {
		return err
	}

	fmt.Println("migrations completed")
	return nil
}

func openPostgres(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func withMigrationLock(ctx context.Context, db *sql.DB, fn func() error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID)
	}()

	return fn()
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)
ON CONFLICT(version) DO UPDATE SET
    checksum = excluded.checksum,
    applied_at = now()`, mf.Version, mf.Checksum); err != nil {
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
	err := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = $1", version).Scan(&existing)
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
