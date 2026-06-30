package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListMigrationFilesSortsTopLevelPostgresMigrations(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "002_second.sql", "SELECT 2;")
	writeTestFile(t, dir, "001_first.sql", "SELECT 1;")
	writeTestFile(t, dir, "note.txt", "ignored")

	files, err := listMigrationFiles(dir)
	if err != nil {
		t.Fatalf("listMigrationFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 migration files, got %d", len(files))
	}
	if files[0].Version != "001_first" || files[1].Version != "002_second" {
		t.Fatalf("unexpected migration order: %s, %s", files[0].Version, files[1].Version)
	}
}

func TestInitialMigrationDefinesUserAuthTables(t *testing.T) {
	root := repoRootFromWorkingDir(t)
	body, err := os.ReadFile(filepath.Join(root, "sql", "migrations", "001_users.sql"))
	if err != nil {
		t.Fatalf("read initial migration failed: %v", err)
	}

	sqlText := string(body)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS users",
		"id BIGSERIAL PRIMARY KEY",
		"avatar TEXT NOT NULL DEFAULT ''",
		"bio TEXT NOT NULL DEFAULT ''",
		"CREATE TABLE IF NOT EXISTS refresh_tokens",
		"token_version BIGINT NOT NULL DEFAULT 0",
		"rotated_to_hash TEXT",
		"idx_refresh_tokens_user_revoked",
		"idx_users_role_status",
	} {
		if !strings.Contains(sqlText, want) {
			t.Fatalf("initial migration missing %q", want)
		}
	}
}

func writeTestFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s failed: %v", name, err)
	}
}

func repoRootFromWorkingDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
