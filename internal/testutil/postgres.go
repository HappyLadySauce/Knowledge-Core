package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultDatabaseURL = "postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable"

var schemaCounter uint64

// NewPostgresDB creates an isolated PostgreSQL schema and applies all migrations.
// NewPostgresDB 创建隔离的 PostgreSQL schema 并执行全部迁移。
func NewPostgresDB(t testing.TB) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	databaseURL := strings.TrimSpace(os.Getenv("KNOWLEDGE_CORE_DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}
	schema := fmt.Sprintf("test_%d_%d_%d", os.Getpid(), time.Now().UnixNano(), atomic.AddUint64(&schemaCounter, 1))

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping postgres failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		_ = db.Close()
		t.Fatalf("create test schema failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `SET search_path TO `+quoteIdentifier(schema)+`, public`); err != nil {
		_ = db.Close()
		t.Fatalf("set test schema search_path failed: %v", err)
	}
	applyMigrations(t, ctx, db)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)
		_ = db.Close()
	})
	return db
}

func applyMigrations(t testing.TB, ctx context.Context, db *sql.DB) {
	t.Helper()
	migrationsDir := filepath.Join(RepoRoot(t), "sql", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations directory failed: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		path := filepath.Join(migrationsDir, entry.Name())
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s failed: %v", entry.Name(), err)
		}
		if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s failed: %v", entry.Name(), err)
		}
	}
}

// RepoRoot returns the repository root by walking up from the current working directory.
// RepoRoot 从当前工作目录向上查找仓库根目录。
func RepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
