package document

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
	if created.Content != "Goroutine and channel examples" || len(created.Blocks) != 1 {
		t.Fatalf("unexpected created blocks/content: %+v content=%q", created.Blocks, created.Content)
	}
	publicDetail, err := service.GetPublic(ctx, created.ID)
	if err != nil {
		t.Fatalf("get public published document failed: %v", err)
	}
	if publicDetail.Content != "Goroutine and channel examples" || len(publicDetail.Blocks) != 1 {
		t.Fatalf("unexpected public revision: %+v content=%q", publicDetail.Blocks, publicDetail.Content)
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
	if _, err := service.GetAdmin(ctx, admin, created.ID); !errors.Is(err, apperrors.NotFound) {
		t.Fatalf("get deleted document error = %v, want not found", err)
	}
}

func TestDocumentServiceApplyOpsIdempotencyAndConflict(t *testing.T) {
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
		Slug:       "ops-doc",
		Title:      "Ops Doc",
		Content:    "body",
		CategoryID: category.ID,
	}
	created, err := service.CreateAdmin(ctx, admin, cmd)
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if len(created.Blocks) != 1 || created.Blocks[0].TextContent != "body" {
		t.Fatalf("created blocks = %+v, want one body block", created.Blocks)
	}
	payload := `{"text_content":"updated body"}`
	op := Operation{
		OpID:                 "op-1",
		BaseDocumentVersion:  created.CurrentVersion,
		BlockID:              created.Blocks[0].BlockID,
		ExpectedBlockVersion: created.Blocks[0].Version,
		Type:                 OpTypeUpdateBlock,
		PayloadJSON:          payload,
	}
	first, err := service.ApplyOpsAdmin(ctx, admin, created.ID, ApplyOpsCommand{Ops: []Operation{op}})
	if err != nil {
		t.Fatalf("apply op failed: %v", err)
	}
	if len(first.Acks) != 1 || first.Blocks[0].TextContent != "updated body" {
		t.Fatalf("first apply = %+v blocks=%+v, want ack and updated body", first.Acks, first.Blocks)
	}
	second, err := service.ApplyOpsAdmin(ctx, admin, created.ID, ApplyOpsCommand{Ops: []Operation{op}})
	if err != nil {
		t.Fatalf("duplicate op failed: %v", err)
	}
	if len(second.Acks) != 1 || second.Document.CurrentVersion != first.Document.CurrentVersion {
		t.Fatalf("duplicate result = %+v, want same version %d", second, first.Document.CurrentVersion)
	}
	stale := Operation{
		OpID:                 "op-2",
		BaseDocumentVersion:  created.CurrentVersion,
		BlockID:              created.Blocks[0].BlockID,
		ExpectedBlockVersion: created.Blocks[0].Version,
		Type:                 OpTypeUpdateBlock,
		PayloadJSON:          `{"text_content":"stale"}`,
	}
	conflict, err := service.ApplyOpsAdmin(ctx, admin, created.ID, ApplyOpsCommand{Ops: []Operation{stale}})
	if !errors.Is(err, apperrors.Conflict) {
		t.Fatalf("stale op error = %v, want conflict", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Block.TextContent != "updated body" {
		t.Fatalf("conflict = %+v, want current updated block", conflict.Conflicts)
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
	// Insert a test admin user so document author_id foreign keys are satisfied.
	// The migrations no longer auto-create an admin user.
	// 插入测试 admin 用户以满足文档 author_id 外键约束。
	// 迁移不再自动创建 admin 用户。
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO users (username, email, avatar, bio, password_hash, role, status, token_version, created_at, updated_at)
VALUES ('admin', '', '', '', '', 'admin', 'active', 0, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert test admin failed: %v", err)
	}
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
