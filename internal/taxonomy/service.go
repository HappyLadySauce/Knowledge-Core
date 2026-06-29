package taxonomy

import (
	"context"
	"database/sql"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type Service struct {
	repo *Repository
}

func NewService(db *sql.DB) TaxonomyService {
	return &Service{repo: NewRepository(db)}
}

func (s *Service) ListPublicCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListPublicCategories(ctx)
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, cmd CategoryCommand) (Category, error) {
	cmd.Name = normalizeName(cmd.Name)
	cmd.Slug = normalizeInputSlug(cmd.Slug, cmd.Name)
	if cmd.Name == "" || cmd.Slug == "" || parentIDValue(cmd.ParentID) < 0 {
		return Category{}, apperrors.InvalidRequest
	}
	path, err := s.categoryPath(ctx, cmd.ParentID, cmd.Slug)
	if err != nil {
		return Category{}, err
	}
	return s.repo.CreateCategory(ctx, cmd, path)
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, cmd CategoryUpdateCommand) (Category, error) {
	if id <= 0 {
		return Category{}, apperrors.InvalidRequest
	}
	if cmd.Name == nil && cmd.Slug == nil && cmd.ParentID == nil && cmd.Sort == nil {
		return Category{}, apperrors.InvalidRequest
	}
	current, err := s.repo.GetCategoryByID(ctx, id)
	if err != nil {
		return Category{}, err
	}
	nextName := current.Name
	if cmd.Name != nil {
		nextName = normalizeName(*cmd.Name)
		if nextName == "" {
			return Category{}, apperrors.InvalidRequest
		}
		cmd.Name = &nextName
	}
	nextSlug := current.Slug
	if cmd.Slug != nil {
		nextSlug = normalizeInputSlug(*cmd.Slug, nextName)
		if nextSlug == "" {
			return Category{}, apperrors.InvalidRequest
		}
		cmd.Slug = &nextSlug
	}
	nextParentID := current.ParentID
	if cmd.ParentID != nil {
		nextParentID = *cmd.ParentID
		if nextParentID < 0 || nextParentID == id {
			return Category{}, apperrors.InvalidRequest
		}
	}
	pathChanged := nextSlug != current.Slug || nextParentID != current.ParentID
	if pathChanged {
		if err := s.ensureCategoryPathChangeAllowed(ctx, id); err != nil {
			return Category{}, err
		}
	}
	path, err := s.categoryPath(ctx, &nextParentID, nextSlug)
	if err != nil {
		return Category{}, err
	}
	cmd.ParentID = &nextParentID
	return s.repo.UpdateCategory(ctx, id, cmd, path)
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperrors.InvalidRequest
	}
	if _, err := s.repo.GetCategoryByID(ctx, id); err != nil {
		return err
	}
	children, err := s.repo.CountCategoryChildren(ctx, id)
	if err != nil {
		return err
	}
	documents, err := s.repo.CountCategoryDocuments(ctx, id)
	if err != nil {
		return err
	}
	if children > 0 || documents > 0 {
		return apperrors.Conflict
	}
	return s.repo.DeleteCategory(ctx, id)
}

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListTags(ctx)
}

func (s *Service) ListPublicTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListPublicTags(ctx)
}

func (s *Service) CreateTag(ctx context.Context, cmd TagCommand) (Tag, error) {
	cmd.Name = normalizeName(cmd.Name)
	cmd.Slug = normalizeInputSlug(cmd.Slug, cmd.Name)
	if cmd.Name == "" || cmd.Slug == "" {
		return Tag{}, apperrors.InvalidRequest
	}
	return s.repo.CreateTag(ctx, cmd)
}

func (s *Service) UpdateTag(ctx context.Context, id int64, cmd TagUpdateCommand) (Tag, error) {
	if id <= 0 || (cmd.Name == nil && cmd.Slug == nil) {
		return Tag{}, apperrors.InvalidRequest
	}
	if _, err := s.repo.GetTagByID(ctx, id); err != nil {
		return Tag{}, err
	}
	if err := s.ensureTagUnused(ctx, id); err != nil {
		return Tag{}, err
	}
	nextName := ""
	if cmd.Name != nil {
		nextName = normalizeName(*cmd.Name)
		if nextName == "" {
			return Tag{}, apperrors.InvalidRequest
		}
		cmd.Name = &nextName
	}
	if cmd.Slug != nil {
		sourceName := nextName
		if sourceName == "" {
			current, err := s.repo.GetTagByID(ctx, id)
			if err != nil {
				return Tag{}, err
			}
			sourceName = current.Name
		}
		nextSlug := normalizeInputSlug(*cmd.Slug, sourceName)
		if nextSlug == "" {
			return Tag{}, apperrors.InvalidRequest
		}
		cmd.Slug = &nextSlug
	}
	return s.repo.UpdateTag(ctx, id, cmd)
}

func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperrors.InvalidRequest
	}
	if _, err := s.repo.GetTagByID(ctx, id); err != nil {
		return err
	}
	if err := s.ensureTagUnused(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteTag(ctx, id)
}

func (s *Service) categoryPath(ctx context.Context, parentID *int64, slug string) (string, error) {
	if slug == "" || hasPathTraversalSegment(slug) {
		return "", apperrors.InvalidRequest
	}
	parent := parentIDValue(parentID)
	if parent == 0 {
		return slug, nil
	}
	parentCategory, err := s.repo.GetCategoryByID(ctx, parent)
	if err != nil {
		return "", err
	}
	return parentCategory.Path + "/" + slug, nil
}

func (s *Service) ensureCategoryPathChangeAllowed(ctx context.Context, id int64) error {
	children, err := s.repo.CountCategoryChildren(ctx, id)
	if err != nil {
		return err
	}
	documents, err := s.repo.CountCategoryDocuments(ctx, id)
	if err != nil {
		return err
	}
	if children > 0 || documents > 0 {
		return apperrors.Conflict
	}
	return nil
}

func (s *Service) ensureTagUnused(ctx context.Context, id int64) error {
	documents, err := s.repo.CountTagDocuments(ctx, id)
	if err != nil {
		return err
	}
	if documents > 0 {
		return apperrors.Conflict
	}
	return nil
}

func normalizeInputSlug(slug, name string) string {
	if slug != "" {
		return NormalizeSlug(slug)
	}
	return SlugFromName(name)
}
