package taxonomy

import (
	"context"
	"time"
)

// Category is a hierarchical document category.
// Category 是文档层级分类。
type Category struct {
	ID            int64
	Name          string
	Slug          string
	Path          string
	ParentID      int64
	Sort          int
	DocumentCount int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Tag is a flat document label.
// Tag 是扁平文档标签。
type Tag struct {
	ID            int64
	Name          string
	Slug          string
	DocumentCount int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CategoryCommand struct {
	Name     string
	Slug     string
	ParentID *int64
	Sort     *int
}

type TagCommand struct {
	Name string
	Slug string
}

type CategoryUpdateCommand struct {
	Name     *string
	Slug     *string
	ParentID *int64
	Sort     *int
}

type TagUpdateCommand struct {
	Name *string
	Slug *string
}

type TaxonomyService interface {
	ListPublicCategories(ctx context.Context) ([]Category, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, cmd CategoryCommand) (Category, error)
	UpdateCategory(ctx context.Context, id int64, cmd CategoryUpdateCommand) (Category, error)
	DeleteCategory(ctx context.Context, id int64) error
	ListPublicTags(ctx context.Context) ([]Tag, error)
	ListTags(ctx context.Context) ([]Tag, error)
	CreateTag(ctx context.Context, cmd TagCommand) (Tag, error)
	UpdateTag(ctx context.Context, id int64, cmd TagUpdateCommand) (Tag, error)
	DeleteTag(ctx context.Context, id int64) error
}
