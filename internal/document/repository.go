package document

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, query ListQuery) (ListResult, error) {
	query = normalizeListQuery(query)
	where, args := listWhere(query)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT d.id) FROM documents d LEFT JOIN categories c ON d.category_id = c.id`+where, args...).Scan(&total); err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}

	offset := (query.Page - 1) * query.PageSize
	listArgs := append(append([]any{}, args...), query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, documentSelectSQL+where+`
ORDER BY d.updated_at DESC, d.id DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}

	// Batch-load tags for all documents in one query to avoid N+1.
	// 批量查询所有文档的标签，避免 N+1 查询。
	if len(items) > 0 {
		tagsByDoc, err := r.listTagsByDocumentIDs(ctx, documentIDs(items))
		if err != nil {
			return ListResult{}, err
		}
		for i := range items {
			items[i].Tags = tagsByDoc[items[i].ID]
		}
	}
	return ListResult{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func documentIDs(items []Document) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// listTagsByDocumentIDs fetches tags for multiple documents in a single query.
// listTagsByDocumentIDs 单次查询获取多个文档的标签。
func (r *Repository) listTagsByDocumentIDs(ctx context.Context, ids []int64) (map[int64][]TagSummary, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT dt.document_id, t.id, t.name, t.slug
FROM document_tags dt
JOIN tags t ON t.id = dt.tag_id
WHERE dt.document_id IN (`+strings.Join(placeholders, ",")+`)
ORDER BY t.name ASC`, args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	result := make(map[int64][]TagSummary, len(ids))
	for rows.Next() {
		var docID int64
		var tag TagSummary
		if err := rows.Scan(&docID, &tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, apperrors.Wrap(apperrors.InternalError, err)
		}
		result[docID] = append(result[docID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return result, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Document, error) {
	row := r.db.QueryRowContext(ctx, documentSelectSQL+` WHERE d.id = ?`, id)
	item, err := scanDocument(row)
	if err != nil {
		return Document{}, err
	}
	tags, err := r.listDocumentTags(ctx, id)
	if err != nil {
		return Document{}, err
	}
	item.Tags = tags
	return item, nil
}

func (r *Repository) SlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM documents
WHERE slug = ? AND id <> ?`, slug, excludeID).Scan(&count)
	if err != nil {
		return false, apperrors.Wrap(apperrors.InternalError, err)
	}
	return count > 0, nil
}

func (r *Repository) Create(ctx context.Context, next record) (Document, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	result, err := tx.ExecContext(ctx, `
INSERT INTO documents (
    slug, title, summary, content_path, category_id, source, status, confidence,
    word_count, search_text, cover_url, author_id, created_at, updated_at, published_at
)
VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)`,
		next.Slug, next.Title, next.Summary, next.ContentPath, next.CategoryID, next.Source, next.Status,
		next.Confidence, next.WordCount, next.SearchText, next.CoverURL, next.AuthorID,
		formatTime(next.CreatedAt), formatTime(next.UpdatedAt), formatMaybeTime(next.PublishedAt))
	if err != nil {
		if isSQLiteConstraint(err) {
			return Document{}, apperrors.Conflict
		}
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := replaceDocumentTagsTx(ctx, tx, id, next.TagIDs); err != nil {
		return Document{}, err
	}
	if err := tx.Commit(); err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, next record) (Document, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	result, err := tx.ExecContext(ctx, `
UPDATE documents
SET slug = ?, title = ?, summary = ?, content_path = ?, category_id = NULLIF(?, 0),
    source = ?, status = ?, confidence = ?, word_count = ?, search_text = ?,
    cover_url = ?, updated_at = ?, published_at = ?
WHERE id = ?`,
		next.Slug, next.Title, next.Summary, next.ContentPath, next.CategoryID, next.Source, next.Status,
		next.Confidence, next.WordCount, next.SearchText, next.CoverURL, formatTime(next.UpdatedAt),
		formatMaybeTime(next.PublishedAt), id)
	if err != nil {
		if isSQLiteConstraint(err) {
			return Document{}, apperrors.Conflict
		}
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return Document{}, apperrors.NotFound
	}
	if err := replaceDocumentTagsTx(ctx, tx, id, next.TagIDs); err != nil {
		return Document{}, err
	}
	if err := tx.Commit(); err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return apperrors.NotFound
	}
	return nil
}

func (r *Repository) listDocumentTags(ctx context.Context, documentID int64) ([]TagSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT t.id, t.name, t.slug
FROM tags t
JOIN document_tags dt ON dt.tag_id = t.id
WHERE dt.document_id = ?
ORDER BY t.name ASC`, documentID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	tags := make([]TagSummary, 0)
	for rows.Next() {
		var tag TagSummary
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, apperrors.Wrap(apperrors.InternalError, err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return tags, nil
}

const documentSelectSQL = `
SELECT d.id, d.slug, d.title, d.summary, d.content_path, COALESCE(d.category_id, 0),
       d.source, d.status, d.confidence, d.word_count, d.cover_url, COALESCE(d.author_id, 0),
       d.created_at, d.updated_at, d.published_at,
       c.id, c.name, c.slug, c.path
FROM documents d
LEFT JOIN categories c ON d.category_id = c.id`

func listWhere(query ListQuery) (string, []any) {
	parts := make([]string, 0, 5)
	args := make([]any, 0, 8)
	if query.Status != "" {
		parts = append(parts, "d.status = ?")
		args = append(args, query.Status)
	}
	if query.Q != "" {
		// Use FTS5 MATCH via subquery instead of LIKE '%q%' full-table scan.
		// 使用 FTS5 MATCH 子查询替代 LIKE '%q%' 全表扫描。
		parts = append(parts, "d.id IN (SELECT rowid FROM documents_fts WHERE documents_fts MATCH ?)")
		args = append(args, fts5Phrase(query.Q))
	}
	if query.Category != "" {
		parts = append(parts, "c.path = ?")
		args = append(args, query.Category)
	}
	if query.Tag != "" {
		parts = append(parts, `EXISTS (
SELECT 1 FROM document_tags dt
JOIN tags t ON t.id = dt.tag_id
WHERE dt.document_id = d.id AND (t.slug = ? OR t.name = ?)
)`)
		args = append(args, query.Tag, query.Tag)
	}
	if len(parts) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// fts5Phrase wraps user input as an FTS5 phrase query so special characters
// (*, :, ", etc.) are treated as literals instead of FTS5 operators.
// fts5Phrase 将用户输入包装为 FTS5 短语查询，使特殊字符（*、:、"等）
// 被视为字面量而非 FTS5 操作符。
func fts5Phrase(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func scanDocument(row interface {
	Scan(dest ...any) error
}) (Document, error) {
	var item Document
	var createdText, updatedText string
	var publishedText sql.NullString
	var categoryID sql.NullInt64
	var categoryName, categorySlug, categoryPath sql.NullString
	err := row.Scan(
		&item.ID, &item.Slug, &item.Title, &item.Summary, &item.ContentPath, &item.CategoryID,
		&item.Source, &item.Status, &item.Confidence, &item.WordCount, &item.CoverURL, &item.AuthorID,
		&createdText, &updatedText, &publishedText,
		&categoryID, &categoryName, &categorySlug, &categoryPath,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Document{}, apperrors.NotFound
		}
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	createdAt, err := parseTime(createdText)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	if publishedText.Valid && publishedText.String != "" {
		publishedAt, err := parseTime(publishedText.String)
		if err != nil {
			return Document{}, apperrors.Wrap(apperrors.InternalError, err)
		}
		item.PublishedAt = &publishedAt
	}
	if categoryID.Valid {
		item.Category = &CategorySummary{
			ID:   categoryID.Int64,
			Name: categoryName.String,
			Slug: categorySlug.String,
			Path: categoryPath.String,
		}
	}
	return item, nil
}

func replaceDocumentTagsTx(ctx context.Context, tx *sql.Tx, documentID int64, tagIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_tags WHERE document_id = ?`, documentID); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO document_tags (document_id, tag_id)
VALUES (?, ?)`, documentID, tagID); err != nil {
			if isSQLiteConstraint(err) {
				return apperrors.InvalidRequest
			}
			return apperrors.Wrap(apperrors.InternalError, err)
		}
	}
	return nil
}

func normalizeListQuery(query ListQuery) ListQuery {
	if query.Page <= 0 {
		query.Page = defaultPage
	}
	if query.PageSize <= 0 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	query.Q = strings.TrimSpace(query.Q)
	query.Category = strings.TrimSpace(query.Category)
	query.Tag = strings.TrimSpace(query.Tag)
	query.Status = strings.TrimSpace(query.Status)
	return query
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatMaybeTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func isSQLiteConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint")
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		// Rollback failures after a failed transaction are not actionable here.
		// 事务失败后的回滚错误在此处不可恢复。
		return
	}
}
