package taxonomy

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

func TestCategoryAndTagCRUD(t *testing.T) {
	ctx := context.Background()
	service := NewService(newTaxonomyTestDB(t))

	tech, err := service.CreateCategory(ctx, CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create root category failed: %v", err)
	}
	ai, err := service.CreateCategory(ctx, CategoryCommand{Name: "AI", Slug: "ai", ParentID: &tech.ID})
	if err != nil {
		t.Fatalf("create child category failed: %v", err)
	}
	if ai.Path != "tech/ai" {
		t.Fatalf("unexpected category path: %s", ai.Path)
	}

	categories, err := service.ListCategories(ctx)
	if err != nil {
		t.Fatalf("list categories failed: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("category count = %d, want 2", len(categories))
	}

	if err := service.DeleteCategory(ctx, tech.ID); !errors.Is(err, apperrors.Conflict) {
		t.Fatalf("delete parent category error = %v, want conflict", err)
	}
	renamedSlug := "machine-learning"
	updated, err := service.UpdateCategory(ctx, ai.ID, CategoryUpdateCommand{Slug: &renamedSlug})
	if err != nil {
		t.Fatalf("update leaf category failed: %v", err)
	}
	if updated.ParentID != tech.ID {
		t.Fatalf("updated parent_id = %d, want %d", updated.ParentID, tech.ID)
	}
	if updated.Path != "tech/machine-learning" {
		t.Fatalf("updated path = %s, want tech/machine-learning", updated.Path)
	}
	if err := service.DeleteCategory(ctx, ai.ID); err != nil {
		t.Fatalf("delete child category failed: %v", err)
	}

	tag, err := service.CreateTag(ctx, TagCommand{Name: "Go", Slug: "go"})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	if _, err := service.CreateTag(ctx, TagCommand{Name: "Go", Slug: "go"}); !errors.Is(err, apperrors.Conflict) {
		t.Fatalf("duplicate tag error = %v, want conflict", err)
	}
	name := "Golang"
	updatedTag, err := service.UpdateTag(ctx, tag.ID, TagUpdateCommand{Name: &name})
	if err != nil {
		t.Fatalf("update tag failed: %v", err)
	}
	if updatedTag.Name != "Golang" {
		t.Fatalf("updated tag name = %s, want Golang", updatedTag.Name)
	}
	if err := service.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("delete tag failed: %v", err)
	}
}

func TestPublicTaxonomyListsOnlyPublishedMetadata(t *testing.T) {
	ctx := context.Background()
	db := newTaxonomyTestDB(t)
	service := NewService(db)

	publishedCategory, err := service.CreateCategory(ctx, CategoryCommand{Name: "Published", Slug: "published"})
	if err != nil {
		t.Fatalf("create published category failed: %v", err)
	}
	draftCategory, err := service.CreateCategory(ctx, CategoryCommand{Name: "Draft", Slug: "draft"})
	if err != nil {
		t.Fatalf("create draft category failed: %v", err)
	}
	publishedTag, err := service.CreateTag(ctx, TagCommand{Name: "Public", Slug: "public"})
	if err != nil {
		t.Fatalf("create published tag failed: %v", err)
	}
	draftTag, err := service.CreateTag(ctx, TagCommand{Name: "Private", Slug: "private"})
	if err != nil {
		t.Fatalf("create draft tag failed: %v", err)
	}
	insertTaxonomyDocument(t, db, "published-doc", publishedCategory.ID, publishedTag.ID, "published")
	insertTaxonomyDocument(t, db, "draft-doc", draftCategory.ID, draftTag.ID, "draft")

	publicCategories, err := service.ListPublicCategories(ctx)
	if err != nil {
		t.Fatalf("list public categories failed: %v", err)
	}
	if len(publicCategories) != 1 || publicCategories[0].ID != publishedCategory.ID || publicCategories[0].DocumentCount != 1 {
		t.Fatalf("public categories = %+v, want only published category", publicCategories)
	}
	adminCategories, err := service.ListCategories(ctx)
	if err != nil {
		t.Fatalf("list admin categories failed: %v", err)
	}
	if len(adminCategories) != 2 {
		t.Fatalf("admin category count = %d, want 2", len(adminCategories))
	}

	publicTags, err := service.ListPublicTags(ctx)
	if err != nil {
		t.Fatalf("list public tags failed: %v", err)
	}
	if len(publicTags) != 1 || publicTags[0].ID != publishedTag.ID || publicTags[0].DocumentCount != 1 {
		t.Fatalf("public tags = %+v, want only published tag", publicTags)
	}
	adminTags, err := service.ListTags(ctx)
	if err != nil {
		t.Fatalf("list admin tags failed: %v", err)
	}
	if len(adminTags) != 2 {
		t.Fatalf("admin tag count = %d, want 2", len(adminTags))
	}
}

func TestUsedTagCannotBeRenamedOrDeleted(t *testing.T) {
	ctx := context.Background()
	db := newTaxonomyTestDB(t)
	service := NewService(db)

	category, err := service.CreateCategory(ctx, CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	tag, err := service.CreateTag(ctx, TagCommand{Name: "Go", Slug: "go"})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	insertTaxonomyDocument(t, db, "go-doc", category.ID, tag.ID, "published")

	name := "Golang"
	if _, err := service.UpdateTag(ctx, tag.ID, TagUpdateCommand{Name: &name}); !errors.Is(err, apperrors.Conflict) {
		t.Fatalf("update used tag error = %v, want conflict", err)
	}
	if err := service.DeleteTag(ctx, tag.ID); !errors.Is(err, apperrors.Conflict) {
		t.Fatalf("delete used tag error = %v, want conflict", err)
	}
}

func newTaxonomyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "taxonomy.db")))
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys failed: %v", err)
	}
	applyMigrationFiles(t, db)
	return db
}

func applyMigrationFiles(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationsDir := filepath.Join(findRepoRoot(t), "sql", "migrations")
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

func findRepoRoot(t *testing.T) string {
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

func insertTaxonomyDocument(t *testing.T, db *sql.DB, slug string, categoryID int64, tagID int64, status string) {
	t.Helper()
	now := formatTime(time.Now().UTC())
	result, err := db.ExecContext(context.Background(), `
INSERT INTO documents (
    slug, title, summary, category_id, source, status, confidence,
    word_count, search_text, cover_url, current_version, created_at, updated_at
)
VALUES (?, ?, '', ?, 'manual', ?, 1, 1, ?, '', 1, ?, ?)`,
		slug, slug, categoryID, status, slug, now, now)
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	documentID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted document id failed: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO document_tags (document_id, tag_id)
VALUES (?, ?)`, documentID, tagID); err != nil {
		t.Fatalf("insert document tag failed: %v", err)
	}
}
