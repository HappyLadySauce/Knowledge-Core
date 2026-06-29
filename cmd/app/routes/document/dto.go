package document

import (
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internaldocument "github.com/HappyLadySauce/Knowledge-Core/internal/document"
)

func toDocumentResponse(item internaldocument.Document) v1.DocumentResponse {
	response := v1.DocumentResponse{
		ID:          item.ID,
		Slug:        item.Slug,
		Title:       item.Title,
		Summary:     item.Summary,
		CategoryID:  item.CategoryID,
		Tags:        toDocumentTagResponses(item.Tags),
		Source:      item.Source,
		Status:      item.Status,
		Confidence:  item.Confidence,
		WordCount:   item.WordCount,
		CoverURL:    item.CoverURL,
		AuthorID:    item.AuthorID,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		PublishedAt: item.PublishedAt,
	}
	if item.Category != nil {
		response.Category = &v1.DocumentCategoryResponse{
			ID:   item.Category.ID,
			Name: item.Category.Name,
			Slug: item.Category.Slug,
			Path: item.Category.Path,
		}
	}
	return response
}

func toDocumentDetailResponse(item internaldocument.Detail) v1.DocumentResponse {
	response := toDocumentResponse(item.Document)
	response.Content = item.Content
	return response
}

func toListDocumentsResponse(result internaldocument.ListResult) v1.ListDocumentsResponse {
	items := make([]v1.DocumentResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toDocumentResponse(item))
	}
	return v1.ListDocumentsResponse{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}
}

func toDocumentTagResponses(tags []internaldocument.TagSummary) []v1.DocumentTagResponse {
	items := make([]v1.DocumentTagResponse, 0, len(tags))
	for _, tag := range tags {
		items = append(items, v1.DocumentTagResponse{
			ID:   tag.ID,
			Name: tag.Name,
			Slug: tag.Slug,
		})
	}
	return items
}
