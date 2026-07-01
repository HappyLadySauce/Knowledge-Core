package taxonomy

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/testutil"
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
	return testutil.NewDB(t)
}

func insertTaxonomyDocument(t *testing.T, db *sql.DB, slug string, categoryID int64, tagID int64, status string) {
	t.Helper()
	now := time.Now().UTC()
	var documentID int64
	err := db.QueryRowContext(context.Background(), `
INSERT INTO documents (
    slug, title, summary, category_id, source, status, confidence,
    word_count, search_text, cover_url, current_version, created_at, updated_at
)
VALUES ($1, $2, '', $3, 'manual', $4, 1, 1, $5, '', 1, $6, $7)
RETURNING id`,
		slug, slug, categoryID, status, slug, now, now).Scan(&documentID)
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO document_tags (document_id, tag_id)
VALUES ($1, $2)`, documentID, tagID); err != nil {
		t.Fatalf("insert document tag failed: %v", err)
	}
}
