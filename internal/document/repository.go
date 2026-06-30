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

func (r *Repository) GetBlocks(ctx context.Context, documentID int64) ([]Block, error) {
	return r.getBlocks(ctx, r.db, documentID)
}

func (r *Repository) GetPublishedRevision(ctx context.Context, documentID int64) (string, []Block, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT content_text, snapshot_json
FROM document_revisions
WHERE document_id = ?
ORDER BY version DESC
LIMIT 1`, documentID)
	var content, snapshot string
	if err := row.Scan(&content, &snapshot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, apperrors.NotFound
		}
		return "", nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	blocks, err := decodeBlocksSnapshot(snapshot)
	if err != nil {
		return "", nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return content, blocks, nil
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

func (r *Repository) Create(ctx context.Context, next record, blocks []Block, revisionContent string) (Document, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	result, err := tx.ExecContext(ctx, `
INSERT INTO documents (
    slug, title, summary, category_id, source, status, confidence,
    word_count, search_text, cover_url, author_id, current_version, created_at, updated_at, published_at
)
VALUES (?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?)`,
		next.Slug, next.Title, next.Summary, next.CategoryID, next.Source, next.Status,
		next.Confidence, next.WordCount, next.SearchText, next.CoverURL, next.AuthorID, next.CurrentVersion,
		formatTime(next.CreatedAt), formatTime(next.UpdatedAt), formatMaybeTime(next.PublishedAt))
	if err != nil {
		if isSQLConstraint(err) {
			return Document{}, apperrors.Conflict
		}
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := replaceDocumentBlocksTx(ctx, tx, id, blocks); err != nil {
		return Document{}, err
	}
	if err := replaceDocumentTagsTx(ctx, tx, id, next.TagIDs); err != nil {
		return Document{}, err
	}
	if next.Status == StatusPublished {
		if err := insertRevisionTx(ctx, tx, id, next.CurrentVersion, blocks, revisionContent, next.AuthorID, next.UpdatedAt); err != nil {
			return Document{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, next record, blocks []Block, revisionContent string) (Document, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	result, err := tx.ExecContext(ctx, `
UPDATE documents
SET slug = ?, title = ?, summary = ?, category_id = NULLIF(?, 0),
    source = ?, status = ?, confidence = ?, word_count = ?, search_text = ?,
    cover_url = ?, current_version = ?, updated_at = ?, published_at = ?
WHERE id = ?`,
		next.Slug, next.Title, next.Summary, next.CategoryID, next.Source, next.Status,
		next.Confidence, next.WordCount, next.SearchText, next.CoverURL, next.CurrentVersion,
		formatTime(next.UpdatedAt), formatMaybeTime(next.PublishedAt), id)
	if err != nil {
		if isSQLConstraint(err) {
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
	if err := replaceDocumentBlocksTx(ctx, tx, id, blocks); err != nil {
		return Document{}, err
	}
	if err := replaceDocumentTagsTx(ctx, tx, id, next.TagIDs); err != nil {
		return Document{}, err
	}
	if next.Status == StatusPublished {
		if err := insertRevisionTx(ctx, tx, id, next.CurrentVersion, blocks, revisionContent, next.AuthorID, next.UpdatedAt); err != nil {
			return Document{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Document{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) ApplyOps(ctx context.Context, documentID, actorID int64, ops []Operation) (ApplyOpsResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ApplyOpsResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	doc, err := r.getDocumentByID(ctx, tx, documentID)
	if err != nil {
		return ApplyOpsResult{}, err
	}
	result := ApplyOpsResult{Acks: make([]OperationAck, 0, len(ops)), Conflicts: make([]OperationConflict, 0)}
	for _, op := range ops {
		if existing, ok, err := r.getExistingAck(ctx, tx, documentID, op.OpID); err != nil {
			return ApplyOpsResult{}, err
		} else if ok {
			result.Acks = append(result.Acks, existing)
			continue
		}
		block, err := r.getBlock(ctx, tx, documentID, op.BlockID)
		if err != nil {
			return ApplyOpsResult{}, err
		}
		if block.Version != op.ExpectedBlockVersion {
			result.Conflicts = append(result.Conflicts, OperationConflict{
				OpID:            op.OpID,
				DocumentID:      documentID,
				DocumentVersion: doc.CurrentVersion,
				Block:           block,
			})
			continue
		}
		nextBlock, err := applyBlockOperation(block, op, actorID, time.Now().UTC())
		if err != nil {
			return ApplyOpsResult{}, err
		}
		doc.CurrentVersion++
		if err := updateBlockTx(ctx, tx, nextBlock); err != nil {
			return ApplyOpsResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO document_ops (
    op_id, document_id, actor_id, base_document_version, block_id, op_type,
    payload_json, document_version, created_at
) VALUES (?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?)`,
			op.OpID, documentID, actorID, op.BaseDocumentVersion, op.BlockID, op.Type, op.PayloadJSON,
			doc.CurrentVersion, formatTime(time.Now().UTC())); err != nil {
			if isSQLConstraint(err) {
				return ApplyOpsResult{}, apperrors.Conflict
			}
			return ApplyOpsResult{}, apperrors.Wrap(apperrors.InternalError, err)
		}
		result.Acks = append(result.Acks, OperationAck{
			OpID:            op.OpID,
			DocumentID:      documentID,
			DocumentVersion: doc.CurrentVersion,
			BlockID:         nextBlock.BlockID,
			BlockVersion:    nextBlock.Version,
		})
	}
	blocks, err := r.getBlocks(ctx, tx, documentID)
	if err != nil {
		return ApplyOpsResult{}, err
	}
	searchText := buildSearchTextFromBlocks(doc, blocks)
	wordCount := countWords(blocksToMarkdown(blocks))
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE documents
SET current_version = ?, search_text = ?, word_count = ?, updated_at = ?
WHERE id = ?`, doc.CurrentVersion, searchText, wordCount, formatTime(now), documentID); err != nil {
		return ApplyOpsResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return ApplyOpsResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updated, err := r.GetByID(ctx, documentID)
	if err != nil {
		return ApplyOpsResult{}, err
	}
	updatedBlocks, err := r.GetBlocks(ctx, documentID)
	if err != nil {
		return ApplyOpsResult{}, err
	}
	result.Document = updated
	result.Blocks = updatedBlocks
	return result, nil
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

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *Repository) getDocumentByID(ctx context.Context, q queryer, id int64) (Document, error) {
	row := q.QueryRowContext(ctx, documentSelectSQL+` WHERE d.id = ?`, id)
	return scanDocument(row)
}

func (r *Repository) getBlocks(ctx context.Context, q queryer, documentID int64) ([]Block, error) {
	rows, err := q.QueryContext(ctx, `
SELECT block_id, document_id, parent_id, position_key, type, content_json, text_content,
       version, COALESCE(updated_by, 0), updated_at
FROM document_blocks
WHERE document_id = ?
ORDER BY position_key ASC, block_id ASC`, documentID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()
	blocks := make([]Block, 0)
	for rows.Next() {
		block, err := scanBlock(rows)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return blocks, nil
}

func (r *Repository) getBlock(ctx context.Context, q queryer, documentID int64, blockID string) (Block, error) {
	row := q.QueryRowContext(ctx, `
SELECT block_id, document_id, parent_id, position_key, type, content_json, text_content,
       version, COALESCE(updated_by, 0), updated_at
FROM document_blocks
WHERE document_id = ? AND block_id = ?`, documentID, blockID)
	return scanBlock(row)
}

func (r *Repository) getExistingAck(ctx context.Context, q queryer, documentID int64, opID string) (OperationAck, bool, error) {
	var ack OperationAck
	var blockVersion sql.NullInt64
	err := q.QueryRowContext(ctx, `
SELECT o.op_id, o.document_id, o.document_version, o.block_id, b.version
FROM document_ops o
LEFT JOIN document_blocks b ON b.document_id = o.document_id AND b.block_id = o.block_id
WHERE o.document_id = ? AND o.op_id = ?`, documentID, opID).
		Scan(&ack.OpID, &ack.DocumentID, &ack.DocumentVersion, &ack.BlockID, &blockVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OperationAck{}, false, nil
		}
		return OperationAck{}, false, apperrors.Wrap(apperrors.InternalError, err)
	}
	if blockVersion.Valid {
		ack.BlockVersion = blockVersion.Int64
	}
	return ack, true, nil
}

func updateBlockTx(ctx context.Context, tx *sql.Tx, block Block) error {
	_, err := tx.ExecContext(ctx, `
UPDATE document_blocks
SET parent_id = ?, position_key = ?, type = ?, content_json = ?, text_content = ?,
    version = ?, updated_by = NULLIF(?, 0), updated_at = ?
WHERE document_id = ? AND block_id = ?`,
		block.ParentID, block.PositionKey, block.Type, block.ContentJSON, block.TextContent,
		block.Version, block.UpdatedBy, formatTime(block.UpdatedAt), block.DocumentID, block.BlockID)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func replaceDocumentBlocksTx(ctx context.Context, tx *sql.Tx, documentID int64, blocks []Block) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_blocks WHERE document_id = ?`, documentID); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	for _, block := range blocks {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO document_blocks (
    block_id, document_id, parent_id, position_key, type, content_json, text_content,
    version, updated_by, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?)`,
			block.BlockID, documentID, block.ParentID, block.PositionKey, block.Type, block.ContentJSON,
			block.TextContent, block.Version, block.UpdatedBy, formatTime(block.UpdatedAt)); err != nil {
			if isSQLConstraint(err) {
				return apperrors.Conflict
			}
			return apperrors.Wrap(apperrors.InternalError, err)
		}
	}
	return nil
}

func insertRevisionTx(ctx context.Context, tx *sql.Tx, documentID, version int64, blocks []Block, content string, createdBy int64, now time.Time) error {
	snapshot, err := encodeBlocksSnapshot(blocks)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO document_revisions (document_id, version, snapshot_json, content_text, created_by, created_at)
VALUES (?, ?, ?, ?, NULLIF(?, 0), ?)
ON CONFLICT(document_id, version) DO UPDATE SET
    snapshot_json = excluded.snapshot_json,
    content_text = excluded.content_text,
    created_by = excluded.created_by,
    created_at = excluded.created_at`,
		documentID, version, snapshot, content, createdBy, formatTime(now))
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func documentIDs(items []Document) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

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
SELECT d.id, d.slug, d.title, d.summary, COALESCE(d.category_id, 0),
       d.source, d.status, d.confidence, d.word_count, d.cover_url, COALESCE(d.author_id, 0),
       d.current_version, d.created_at, d.updated_at, d.published_at,
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
		&item.ID, &item.Slug, &item.Title, &item.Summary, &item.CategoryID,
		&item.Source, &item.Status, &item.Confidence, &item.WordCount, &item.CoverURL, &item.AuthorID,
		&item.CurrentVersion, &createdText, &updatedText, &publishedText,
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

func scanBlock(row interface {
	Scan(dest ...any) error
}) (Block, error) {
	var block Block
	var updatedText string
	err := row.Scan(
		&block.BlockID, &block.DocumentID, &block.ParentID, &block.PositionKey, &block.Type,
		&block.ContentJSON, &block.TextContent, &block.Version, &block.UpdatedBy, &updatedText,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Block{}, apperrors.NotFound
		}
		return Block{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return Block{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	block.UpdatedAt = updatedAt
	return block, nil
}

func replaceDocumentTagsTx(ctx context.Context, tx *sql.Tx, documentID int64, tagIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_tags WHERE document_id = ?`, documentID); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO document_tags (document_id, tag_id)
VALUES (?, ?)`, documentID, tagID); err != nil {
			if isSQLConstraint(err) {
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

func isSQLConstraint(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "constraint") || strings.Contains(text, "duplicate key")
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}
