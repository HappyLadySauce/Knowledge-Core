package document

import (
	"context"
	"time"

	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"

	SourceManual = "manual"
	SourceImport = "import"
	SourceAgent  = "agent"

	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// CategorySummary is the category snapshot returned with a document.
// CategorySummary 是文档响应中携带的分类快照。
type CategorySummary struct {
	ID   int64
	Name string
	Slug string
	Path string
}

// TagSummary is the tag snapshot returned with a document.
// TagSummary 是文档响应中携带的标签快照。
type TagSummary struct {
	ID   int64
	Name string
	Slug string
}

// Document is the indexed Markdown document metadata.
// Document 是 Markdown 文档索引元数据。
type Document struct {
	ID          int64
	Slug        string
	Title       string
	Summary     string
	ContentPath string
	CategoryID  int64
	Category    *CategorySummary
	Tags        []TagSummary
	Source      string
	Status      string
	Confidence  float64
	WordCount   int
	CoverURL    string
	AuthorID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
}

type Detail struct {
	Document
	Content string
}

type ListQuery struct {
	Page     int
	PageSize int
	Q        string
	Category string
	Tag      string
	Status   string
}

type ListResult struct {
	Items    []Document
	Total    int64
	Page     int
	PageSize int
}

type CreateCommand struct {
	Slug       string
	Title      string
	Summary    string
	Content    string
	CategoryID int64
	TagIDs     []int64
	Source     string
	Status     string
	Confidence float64
	CoverURL   string
}

type UpdateCommand struct {
	Slug       *string
	Title      *string
	Summary    *string
	Content    *string
	CategoryID *int64
	TagIDs     *[]int64
	Source     *string
	Status     *string
	Confidence *float64
	CoverURL   *string
}

type DocumentService interface {
	ListPublic(ctx context.Context, query ListQuery) (ListResult, error)
	GetPublic(ctx context.Context, id int64) (Detail, error)
	ListAdmin(ctx context.Context, actor user.User, query ListQuery) (ListResult, error)
	CreateAdmin(ctx context.Context, actor user.User, cmd CreateCommand) (Detail, error)
	GetAdmin(ctx context.Context, actor user.User, id int64) (Detail, error)
	UpdateAdmin(ctx context.Context, actor user.User, id int64, cmd UpdateCommand) (Detail, error)
	DeleteAdmin(ctx context.Context, actor user.User, id int64) error
}

type record struct {
	Document
	SearchText string
	TagIDs     []int64
}
