package v1

import "time"

type CategoryResponse struct {
	ID            int64              `json:"id"`
	Name          string             `json:"name"`
	Slug          string             `json:"slug"`
	Path          string             `json:"path"`
	ParentID      int64              `json:"parent_id"`
	Sort          int                `json:"sort"`
	DocumentCount int64              `json:"document_count"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Children      []CategoryResponse `json:"children,omitempty"`
}

type TagResponse struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DocumentCount int64     `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DocumentCategoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Path string `json:"path"`
}

type DocumentTagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type DocumentResponse struct {
	ID          int64                     `json:"id"`
	Slug        string                    `json:"slug"`
	Title       string                    `json:"title"`
	Summary     string                    `json:"summary"`
	Content     string                    `json:"content,omitempty"`
	CategoryID  int64                     `json:"category_id"`
	Category    *DocumentCategoryResponse `json:"category,omitempty"`
	Tags        []DocumentTagResponse     `json:"tags"`
	Source      string                    `json:"source"`
	Status      string                    `json:"status"`
	Confidence  float64                   `json:"confidence"`
	WordCount   int                       `json:"word_count"`
	CoverURL    string                    `json:"cover_url"`
	AuthorID    int64                     `json:"author_id"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	PublishedAt *time.Time                `json:"published_at,omitempty"`
}

type ListDocumentsRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Q        string `form:"q"`
	Category string `form:"category"`
	Tag      string `form:"tag"`
	Status   string `form:"status"`
}

type ListDocumentsResponse struct {
	Items    []DocumentResponse `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type CreateDocumentRequest struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Content    string  `json:"content"`
	CategoryID int64   `json:"category_id"`
	TagIDs     []int64 `json:"tag_ids"`
	Source     string  `json:"source"`
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
	CoverURL   string  `json:"cover_url"`
}

type UpdateDocumentRequest struct {
	Slug       *string  `json:"slug"`
	Title      *string  `json:"title"`
	Summary    *string  `json:"summary"`
	Content    *string  `json:"content"`
	CategoryID *int64   `json:"category_id"`
	TagIDs     *[]int64 `json:"tag_ids"`
	Source     *string  `json:"source"`
	Status     *string  `json:"status"`
	Confidence *float64 `json:"confidence"`
	CoverURL   *string  `json:"cover_url"`
}

type ListCategoriesResponse struct {
	Items []CategoryResponse `json:"items"`
}

type CreateCategoryRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ParentID *int64 `json:"parent_id"`
	Sort     *int   `json:"sort"`
}

type UpdateCategoryRequest struct {
	Name     *string `json:"name"`
	Slug     *string `json:"slug"`
	ParentID *int64  `json:"parent_id"`
	Sort     *int    `json:"sort"`
}

type ListTagsResponse struct {
	Items []TagResponse `json:"items"`
}

type CreateTagRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type UpdateTagRequest struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}
