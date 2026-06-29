package document

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

func TestDocumentServiceSerializesSameSlugCreate(t *testing.T) {
	ctx := context.Background()
	db := newDocumentTestDB(t)
	libraryRoot := t.TempDir()
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	service, err := NewService(db, libraryRoot)
	if err != nil {
		t.Fatalf("create document service failed: %v", err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	cmd := CreateCommand{
		Slug:       "same-slug",
		Title:      "Same Slug",
		Content:    "body",
		CategoryID: category.ID,
	}

	type createResult struct {
		detail Detail
		err    error
	}
	results := make([]createResult, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			detail, err := service.CreateAdmin(ctx, admin, cmd)
			results[index] = createResult{detail: detail, err: err}
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	conflicts := 0
	var created Detail
	for _, result := range results {
		if result.err == nil {
			successes++
			created = result.detail
			continue
		}
		if errors.Is(result.err, apperrors.Conflict) {
			conflicts++
			continue
		}
		t.Fatalf("create error = %v, want nil or conflict", result.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1 success and 1 conflict", successes, conflicts)
	}
	markdownPath := filepath.Join(libraryRoot, filepath.FromSlash(created.ContentPath))
	if _, err := os.Stat(markdownPath); err != nil {
		t.Fatalf("created markdown missing: %v", err)
	}
	list, err := service.ListAdmin(ctx, admin, ListQuery{PageSize: 10})
	if err != nil {
		t.Fatalf("list admin failed: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("document total = %d, want 1", list.Total)
	}
}

func TestDocumentCategoryFilterUsesPathOnly(t *testing.T) {
	ctx := context.Background()
	db := newDocumentTestDB(t)
	libraryRoot := t.TempDir()
	taxonomies := taxonomy.NewService(db)
	tech, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create tech category failed: %v", err)
	}
	life, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Life", Slug: "life"})
	if err != nil {
		t.Fatalf("create life category failed: %v", err)
	}
	aiTech, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "AI", Slug: "ai", ParentID: &tech.ID})
	if err != nil {
		t.Fatalf("create tech ai category failed: %v", err)
	}
	aiLife, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "AI", Slug: "ai", ParentID: &life.ID})
	if err != nil {
		t.Fatalf("create life ai category failed: %v", err)
	}
	service, err := NewService(db, libraryRoot)
	if err != nil {
		t.Fatalf("create document service failed: %v", err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	if _, err := service.CreateAdmin(ctx, admin, CreateCommand{
		Slug:       "tech-ai-doc",
		Title:      "Tech AI",
		Content:    "published body",
		CategoryID: aiTech.ID,
		Status:     StatusPublished,
	}); err != nil {
		t.Fatalf("create tech document failed: %v", err)
	}
	if _, err := service.CreateAdmin(ctx, admin, CreateCommand{
		Slug:       "life-ai-doc",
		Title:      "Life AI",
		Content:    "published body",
		CategoryID: aiLife.ID,
		Status:     StatusPublished,
	}); err != nil {
		t.Fatalf("create life document failed: %v", err)
	}

	ambiguous, err := service.ListPublic(ctx, ListQuery{Category: "ai"})
	if err != nil {
		t.Fatalf("list ambiguous category failed: %v", err)
	}
	if ambiguous.Total != 0 {
		t.Fatalf("ambiguous category total = %d, want 0", ambiguous.Total)
	}
	filtered, err := service.ListPublic(ctx, ListQuery{Category: "tech/ai"})
	if err != nil {
		t.Fatalf("list category path failed: %v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].Slug != "tech-ai-doc" {
		t.Fatalf("filtered result = %+v, want tech-ai-doc", filtered)
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
