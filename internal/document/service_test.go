package document

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

func TestDocumentServiceWritesMarkdownAndIndexesPublishedDocuments(t *testing.T) {
	ctx := context.Background()
	db := newDocumentTestDB(t)
	libraryRoot := t.TempDir()
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	tag, err := taxonomies.CreateTag(ctx, taxonomy.TagCommand{Name: "Go", Slug: "go"})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	service, err := NewService(db, libraryRoot)
	if err != nil {
		t.Fatalf("create document service failed: %v", err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}

	created, err := service.CreateAdmin(ctx, admin, CreateCommand{
		Title:      "Go Concurrency",
		Summary:    "Goroutine and channel notes",
		Content:    "Goroutine and channel examples",
		CategoryID: category.ID,
		TagIDs:     []int64{tag.ID},
		Status:     StatusPublished,
	})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if created.Status != StatusPublished || created.PublishedAt == nil {
		t.Fatalf("unexpected published document: %+v", created.Document)
	}
	markdownPath := filepath.Join(libraryRoot, filepath.FromSlash(created.ContentPath))
	markdownBytes, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown failed: %v", err)
	}
	markdown := string(markdownBytes)
	if !strings.Contains(markdown, `title: "Go Concurrency"`) || !strings.Contains(markdown, `tags: ["Go"]`) {
		t.Fatalf("markdown frontmatter missing expected fields:\n%s", markdown)
	}

	list, err := service.ListPublic(ctx, ListQuery{Q: "goroutine"})
	if err != nil {
		t.Fatalf("list public failed: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("public list = %+v, want one item", list)
	}

	draftStatus := StatusDraft
	updated, err := service.UpdateAdmin(ctx, admin, created.ID, UpdateCommand{Status: &draftStatus})
	if err != nil {
		t.Fatalf("unpublish document failed: %v", err)
	}
	if updated.Status != StatusDraft || updated.PublishedAt != nil {
		t.Fatalf("unexpected draft document: %+v", updated.Document)
	}
	if _, err := service.GetPublic(ctx, created.ID); !errors.Is(err, apperrors.NotFound) {
		t.Fatalf("public get draft error = %v, want not found", err)
	}

	if err := service.DeleteAdmin(ctx, admin, created.ID); err != nil {
		t.Fatalf("delete document failed: %v", err)
	}
	if _, err := os.Stat(markdownPath); !os.IsNotExist(err) {
		t.Fatalf("markdown file still exists or stat failed: %v", err)
	}
}

func newDocumentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "document.db")))
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys failed: %v", err)
	}
	applyDocumentMigrationFiles(t, db)
	return db
}

func applyDocumentMigrationFiles(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationsDir := filepath.Join(findDocumentRepoRoot(t), "sql", "migrations")
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
		if _, err := db.ExecContext(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s failed: %v", entry.Name(), err)
		}
	}
}

func findDocumentRepoRoot(t *testing.T) string {
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
