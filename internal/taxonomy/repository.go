package taxonomy

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/pkg/postgres"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.name, c.slug, c.path, COALESCE(c.parent_id, 0), c.sort,
       COUNT(d.id) AS document_count, c.created_at, c.updated_at
FROM categories c
LEFT JOIN documents d ON d.category_id = c.id
GROUP BY c.id
ORDER BY COALESCE(c.parent_id, 0) ASC, c.sort ASC, c.name ASC`)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return categories, nil
}

func (r *Repository) ListPublicCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
WITH RECURSIVE visible_categories(id) AS (
    SELECT DISTINCT c.id
    FROM categories c
    JOIN documents d ON d.category_id = c.id
    WHERE d.status = 'published'
    UNION
    SELECT parent.id
    FROM categories parent
    JOIN categories child ON child.parent_id = parent.id
    JOIN visible_categories vc ON vc.id = child.id
)
SELECT c.id, c.name, c.slug, c.path, COALESCE(c.parent_id, 0), c.sort,
       COUNT(d.id) AS document_count, c.created_at, c.updated_at
FROM categories c
JOIN visible_categories vc ON vc.id = c.id
LEFT JOIN documents d ON d.category_id = c.id AND d.status = 'published'
GROUP BY c.id
ORDER BY COALESCE(c.parent_id, 0) ASC, c.sort ASC, c.name ASC`)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return categories, nil
}

func (r *Repository) GetCategoryByID(ctx context.Context, id int64) (Category, error) {
	row := r.db.QueryRowContext(ctx, categorySelectSQL+` WHERE c.id = $1 GROUP BY c.id`, id)
	return scanCategory(row)
}

func (r *Repository) GetCategoryByPathOrSlug(ctx context.Context, value string) (Category, error) {
	row := r.db.QueryRowContext(ctx, categorySelectSQL+` WHERE c.path = $1 OR c.slug = $2 GROUP BY c.id`, value, value)
	return scanCategory(row)
}

func (r *Repository) CreateCategory(ctx context.Context, cmd CategoryCommand, path string) (Category, error) {
	now := time.Now().UTC()
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO categories (name, slug, path, parent_id, sort, created_at, updated_at)
VALUES ($1, $2, $3, NULLIF($4, 0), $5, $6, $7)
RETURNING id`,
		cmd.Name, cmd.Slug, path, parentIDValue(cmd.ParentID), sortValue(cmd.Sort), now, now).Scan(&id)
	if err != nil {
		if postgres.IsConstraintViolation(err) {
			return Category{}, apperrors.Conflict
		}
		return Category{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetCategoryByID(ctx, id)
}

func (r *Repository) UpdateCategory(ctx context.Context, id int64, cmd CategoryUpdateCommand, path string) (Category, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
UPDATE categories
SET name = COALESCE($1, name),
    slug = COALESCE($2, slug),
    path = $3,
    parent_id = NULLIF($4, 0),
    sort = COALESCE($5, sort),
    updated_at = $6
WHERE id = $7`,
		cmd.Name, cmd.Slug, path, parentIDValue(cmd.ParentID), cmd.Sort, now, id)
	if err != nil {
		if postgres.IsConstraintViolation(err) {
			return Category{}, apperrors.Conflict
		}
		return Category{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Category{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return Category{}, apperrors.NotFound
	}
	return r.GetCategoryByID(ctx, id)
}

func (r *Repository) DeleteCategory(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		if postgres.IsConstraintViolation(err) {
			return apperrors.Conflict
		}
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

func (r *Repository) CountCategoryChildren(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id = $1`, id).Scan(&count)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.InternalError, err)
	}
	return count, nil
}

func (r *Repository) CountCategoryDocuments(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE category_id = $1`, id).Scan(&count)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.InternalError, err)
	}
	return count, nil
}

func (r *Repository) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT t.id, t.name, t.slug, COUNT(dt.document_id) AS document_count, t.created_at, t.updated_at
FROM tags t
LEFT JOIN document_tags dt ON dt.tag_id = t.id
GROUP BY t.id
ORDER BY document_count DESC, t.name ASC`)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		item, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return tags, nil
}

func (r *Repository) ListPublicTags(ctx context.Context) ([]Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT t.id, t.name, t.slug, COUNT(dt.document_id) AS document_count, t.created_at, t.updated_at
FROM tags t
JOIN document_tags dt ON dt.tag_id = t.id
JOIN documents d ON d.id = dt.document_id AND d.status = 'published'
GROUP BY t.id
ORDER BY document_count DESC, t.name ASC`)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		item, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return tags, nil
}

func (r *Repository) GetTagByID(ctx context.Context, id int64) (Tag, error) {
	row := r.db.QueryRowContext(ctx, tagSelectSQL+` WHERE t.id = $1 GROUP BY t.id`, id)
	return scanTag(row)
}

func (r *Repository) ListTagsByIDs(ctx context.Context, ids []int64) ([]Tag, error) {
	if len(ids) == 0 {
		return []Tag{}, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)+1))
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, tagSelectSQL+` WHERE t.id IN (`+strings.Join(placeholders, ",")+`) GROUP BY t.id ORDER BY t.name ASC`, args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	tags := make([]Tag, 0, len(ids))
	for rows.Next() {
		item, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.InternalError, err)
	}
	return tags, nil
}

func (r *Repository) CreateTag(ctx context.Context, cmd TagCommand) (Tag, error) {
	now := time.Now().UTC()
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO tags (name, slug, created_at, updated_at)
VALUES ($1, $2, $3, $4)
RETURNING id`,
		cmd.Name, cmd.Slug, now, now).Scan(&id)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return Tag{}, apperrors.Conflict
		}
		return Tag{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetTagByID(ctx, id)
}

func (r *Repository) UpdateTag(ctx context.Context, id int64, cmd TagUpdateCommand) (Tag, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
UPDATE tags
SET name = COALESCE($1, name),
    slug = COALESCE($2, slug),
    updated_at = $3
WHERE id = $4`,
		cmd.Name, cmd.Slug, now, id)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return Tag{}, apperrors.Conflict
		}
		return Tag{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Tag{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return Tag{}, apperrors.NotFound
	}
	return r.GetTagByID(ctx, id)
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = $1`, id)
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

func (r *Repository) CountTagDocuments(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_tags WHERE tag_id = $1`, id).Scan(&count)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.InternalError, err)
	}
	return count, nil
}

const categorySelectSQL = `
SELECT c.id, c.name, c.slug, c.path, COALESCE(c.parent_id, 0), c.sort,
       COUNT(d.id) AS document_count, c.created_at, c.updated_at
FROM categories c
LEFT JOIN documents d ON d.category_id = c.id`

const tagSelectSQL = `
SELECT t.id, t.name, t.slug, COUNT(dt.document_id) AS document_count, t.created_at, t.updated_at
FROM tags t
LEFT JOIN document_tags dt ON dt.tag_id = t.id`

func scanCategory(row interface {
	Scan(dest ...any) error
}) (Category, error) {
	var item Category
	err := row.Scan(&item.ID, &item.Name, &item.Slug, &item.Path, &item.ParentID, &item.Sort, &item.DocumentCount, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, apperrors.NotFound
		}
		return Category{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return item, nil
}

func scanTag(row interface {
	Scan(dest ...any) error
}) (Tag, error) {
	var item Tag
	err := row.Scan(&item.ID, &item.Name, &item.Slug, &item.DocumentCount, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tag{}, apperrors.NotFound
		}
		return Tag{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return item, nil
}

func parentIDValue(parentID *int64) int64 {
	if parentID == nil {
		return 0
	}
	return *parentID
}

func sortValue(sort *int) int {
	if sort == nil {
		return 0
	}
	return *sort
}
