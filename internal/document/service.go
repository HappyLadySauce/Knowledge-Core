package document

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

type Service struct {
	repo       *Repository
	taxonomies *taxonomy.Repository
	files      *fileStore
}

func NewService(db *sql.DB, libraryRoot string) (*Service, error) {
	files, err := newFileStore(libraryRoot)
	if err != nil {
		return nil, err
	}
	return &Service{
		repo:       NewRepository(db),
		taxonomies: taxonomy.NewRepository(db),
		files:      files,
	}, nil
}

func (s *Service) ListPublic(ctx context.Context, query ListQuery) (ListResult, error) {
	query.Status = StatusPublished
	return s.repo.List(ctx, query)
}

func (s *Service) GetPublic(ctx context.Context, id int64) (Detail, error) {
	detail, err := s.detail(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	if detail.Status != StatusPublished {
		return Detail{}, apperrors.NotFound
	}
	return detail, nil
}

func (s *Service) ListAdmin(ctx context.Context, actor user.User, query ListQuery) (ListResult, error) {
	if actor.Role != user.RoleAdmin {
		return ListResult{}, apperrors.Forbidden
	}
	if query.Status != "" && !validStatus(query.Status) {
		return ListResult{}, apperrors.InvalidRequest
	}
	return s.repo.List(ctx, query)
}

func (s *Service) CreateAdmin(ctx context.Context, actor user.User, cmd CreateCommand) (Detail, error) {
	if actor.Role != user.RoleAdmin {
		return Detail{}, apperrors.Forbidden
	}
	normalized, categoryPath, tagNames, err := s.normalizeCreate(ctx, actor.ID, cmd)
	if err != nil {
		return Detail{}, err
	}
	exists, err := s.repo.SlugExists(ctx, normalized.Slug, 0)
	if err != nil {
		return Detail{}, err
	}
	if exists {
		return Detail{}, apperrors.Conflict
	}
	fileExists, err := s.files.exists(normalized.ContentPath)
	if err != nil {
		return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if fileExists {
		return Detail{}, apperrors.Conflict
	}
	if err := s.files.writeDocument(normalized.ContentPath, normalized.Document, categoryPath, tagNames, cmd.Content); err != nil {
		return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	created, err := s.repo.Create(ctx, normalized)
	if err != nil {
		_ = s.files.remove(normalized.ContentPath)
		return Detail{}, err
	}
	return Detail{Document: created, Content: cmd.Content}, nil
}

func (s *Service) GetAdmin(ctx context.Context, actor user.User, id int64) (Detail, error) {
	if actor.Role != user.RoleAdmin {
		return Detail{}, apperrors.Forbidden
	}
	return s.detail(ctx, id)
}

func (s *Service) UpdateAdmin(ctx context.Context, actor user.User, id int64, cmd UpdateCommand) (Detail, error) {
	if actor.Role != user.RoleAdmin {
		return Detail{}, apperrors.Forbidden
	}
	if id <= 0 {
		return Detail{}, apperrors.InvalidRequest
	}
	current, err := s.detail(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	next, categoryPath, tagNames, content, err := s.normalizeUpdate(ctx, current, cmd)
	if err != nil {
		return Detail{}, err
	}
	if next.Slug != current.Slug {
		exists, err := s.repo.SlugExists(ctx, next.Slug, id)
		if err != nil {
			return Detail{}, err
		}
		if exists {
			return Detail{}, apperrors.Conflict
		}
	}

	samePath := next.ContentPath == current.ContentPath
	var oldBytes []byte
	if samePath {
		_, oldBytes, err = s.files.readContent(current.ContentPath)
		if err != nil {
			return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
		}
	} else {
		exists, err := s.files.exists(next.ContentPath)
		if err != nil {
			return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
		}
		if exists {
			return Detail{}, apperrors.Conflict
		}
	}

	if err := s.files.writeDocument(next.ContentPath, next.Document, categoryPath, tagNames, content); err != nil {
		return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updated, err := s.repo.Update(ctx, id, next)
	if err != nil {
		if samePath {
			_ = s.files.writeBytes(current.ContentPath, oldBytes)
		} else {
			_ = s.files.remove(next.ContentPath)
		}
		return Detail{}, err
	}
	if !samePath {
		_ = s.files.remove(current.ContentPath)
	}
	return Detail{Document: updated, Content: content}, nil
}

func (s *Service) DeleteAdmin(ctx context.Context, actor user.User, id int64) error {
	if actor.Role != user.RoleAdmin {
		return apperrors.Forbidden
	}
	if id <= 0 {
		return apperrors.InvalidRequest
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.files.remove(current.ContentPath); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func (s *Service) detail(ctx context.Context, id int64) (Detail, error) {
	if id <= 0 {
		return Detail{}, apperrors.InvalidRequest
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	content, _, err := s.files.readContent(item.ContentPath)
	if err != nil {
		return Detail{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return Detail{Document: item, Content: content}, nil
}

func (s *Service) normalizeCreate(ctx context.Context, actorID int64, cmd CreateCommand) (record, string, []string, error) {
	title := strings.TrimSpace(cmd.Title)
	content := strings.TrimSpace(cmd.Content)
	if title == "" || content == "" {
		return record{}, "", nil, apperrors.InvalidRequest
	}
	slug := normalizeSlug(cmd.Slug, title)
	if slug == "" {
		return record{}, "", nil, apperrors.InvalidRequest
	}
	status := normalizeStatus(cmd.Status)
	source := normalizeSource(cmd.Source)
	if status == "" || source == "" {
		return record{}, "", nil, apperrors.InvalidRequest
	}
	category, categoryPath, err := s.resolveCategory(ctx, cmd.CategoryID)
	if err != nil {
		return record{}, "", nil, err
	}
	tags, err := s.resolveTags(ctx, cmd.TagIDs)
	if err != nil {
		return record{}, "", nil, err
	}
	tagIDs, tagNames := tagIDsAndNames(tags)
	contentPath, err := s.files.contentPath(categoryPath, slug)
	if err != nil {
		return record{}, "", nil, apperrors.InvalidRequest
	}
	now := time.Now().UTC()
	publishedAt := publishedAtFor("", status, nil, now)
	doc := Document{
		Slug:        slug,
		Title:       title,
		Summary:     strings.TrimSpace(cmd.Summary),
		ContentPath: contentPath,
		CategoryID:  category.ID,
		Source:      source,
		Status:      status,
		Confidence:  cmd.Confidence,
		WordCount:   countWords(content),
		CoverURL:    strings.TrimSpace(cmd.CoverURL),
		AuthorID:    actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
		PublishedAt: publishedAt,
	}
	return record{Document: doc, SearchText: buildSearchText(doc, categoryPath, tagNames, content), TagIDs: tagIDs}, categoryPath, tagNames, nil
}

func (s *Service) normalizeUpdate(ctx context.Context, current Detail, cmd UpdateCommand) (record, string, []string, string, error) {
	doc := current.Document
	content := current.Content
	if cmd.Title != nil {
		doc.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Slug != nil {
		doc.Slug = normalizeSlug(*cmd.Slug, doc.Title)
	}
	if cmd.Summary != nil {
		doc.Summary = strings.TrimSpace(*cmd.Summary)
	}
	if cmd.Content != nil {
		content = strings.TrimSpace(*cmd.Content)
	}
	if cmd.Source != nil {
		doc.Source = normalizeSource(*cmd.Source)
	}
	if cmd.Status != nil {
		doc.Status = normalizeStatus(*cmd.Status)
	}
	if cmd.Confidence != nil {
		doc.Confidence = *cmd.Confidence
	}
	if cmd.CoverURL != nil {
		doc.CoverURL = strings.TrimSpace(*cmd.CoverURL)
	}
	if doc.Title == "" || doc.Slug == "" || content == "" || doc.Source == "" || doc.Status == "" {
		return record{}, "", nil, "", apperrors.InvalidRequest
	}

	nextCategoryID := doc.CategoryID
	if cmd.CategoryID != nil {
		nextCategoryID = *cmd.CategoryID
	}
	category, categoryPath, err := s.resolveCategory(ctx, nextCategoryID)
	if err != nil {
		return record{}, "", nil, "", err
	}
	doc.CategoryID = category.ID

	tagIDs := make([]int64, 0, len(doc.Tags))
	for _, tag := range doc.Tags {
		tagIDs = append(tagIDs, tag.ID)
	}
	if cmd.TagIDs != nil {
		tagIDs = *cmd.TagIDs
	}
	tags, err := s.resolveTags(ctx, tagIDs)
	if err != nil {
		return record{}, "", nil, "", err
	}
	nextTagIDs, tagNames := tagIDsAndNames(tags)
	contentPath, err := s.files.contentPath(categoryPath, doc.Slug)
	if err != nil {
		return record{}, "", nil, "", apperrors.InvalidRequest
	}
	now := time.Now().UTC()
	doc.ContentPath = contentPath
	doc.WordCount = countWords(content)
	doc.UpdatedAt = now
	doc.PublishedAt = publishedAtFor(current.Status, doc.Status, current.PublishedAt, now)
	return record{Document: doc, SearchText: buildSearchText(doc, categoryPath, tagNames, content), TagIDs: nextTagIDs}, categoryPath, tagNames, content, nil
}

func (s *Service) resolveCategory(ctx context.Context, id int64) (CategorySummary, string, error) {
	if id == 0 {
		return CategorySummary{}, "", nil
	}
	if id < 0 {
		return CategorySummary{}, "", apperrors.InvalidRequest
	}
	category, err := s.taxonomies.GetCategoryByID(ctx, id)
	if err != nil {
		return CategorySummary{}, "", err
	}
	return CategorySummary{ID: category.ID, Name: category.Name, Slug: category.Slug, Path: category.Path}, category.Path, nil
}

func (s *Service) resolveTags(ctx context.Context, ids []int64) ([]taxonomy.Tag, error) {
	ids = dedupeIDs(ids)
	for _, id := range ids {
		if id <= 0 {
			return nil, apperrors.InvalidRequest
		}
	}
	tags, err := s.taxonomies.ListTagsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(tags) != len(ids) {
		return nil, apperrors.InvalidRequest
	}
	return tags, nil
}

func normalizeSlug(slug, title string) string {
	if strings.TrimSpace(slug) == "" {
		slug = taxonomy.SlugFromName(title)
		if slug == "" {
			slug = "document"
		}
		return slug
	}
	return taxonomy.NormalizeSlug(slug)
}

func normalizeStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return StatusDraft
	}
	if validStatus(status) {
		return status
	}
	return ""
}

func validStatus(status string) bool {
	return status == StatusDraft || status == StatusPublished
}

func normalizeSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return SourceManual
	}
	switch source {
	case SourceManual, SourceImport, SourceAgent:
		return source
	default:
		return ""
	}
}

func publishedAtFor(previousStatus, nextStatus string, current *time.Time, now time.Time) *time.Time {
	if nextStatus != StatusPublished {
		return nil
	}
	if previousStatus == StatusPublished && current != nil {
		return current
	}
	published := now
	return &published
}

func buildSearchText(doc Document, categoryPath string, tagNames []string, content string) string {
	parts := []string{doc.Title, doc.Summary, categoryPath, strings.Join(tagNames, " "), content}
	return strings.ToLower(strings.Join(parts, "\n"))
}

func countWords(content string) int {
	fields := strings.Fields(content)
	if len(fields) > 1 {
		return len(fields)
	}
	count := 0
	for _, r := range content {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		count++
	}
	return count
}

func tagIDsAndNames(tags []taxonomy.Tag) ([]int64, []string) {
	ids := make([]int64, 0, len(tags))
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
		names = append(names, tag.Name)
	}
	return ids, names
}

func dedupeIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (d Detail) String() string {
	return fmt.Sprintf("document id=%d slug=%s status=%s", d.ID, d.Slug, d.Status)
}
