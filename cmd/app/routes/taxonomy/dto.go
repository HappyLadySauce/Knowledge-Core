package taxonomy

import (
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internaltaxonomy "github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
)

func toCategoryResponse(item internaltaxonomy.Category) v1.CategoryResponse {
	return v1.CategoryResponse{
		ID:            item.ID,
		Name:          item.Name,
		Slug:          item.Slug,
		Path:          item.Path,
		ParentID:      item.ParentID,
		Sort:          item.Sort,
		DocumentCount: item.DocumentCount,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toListCategoriesResponse(items []internaltaxonomy.Category) v1.ListCategoriesResponse {
	return v1.ListCategoriesResponse{Items: buildCategoryTree(items)}
}

func buildCategoryTree(items []internaltaxonomy.Category) []v1.CategoryResponse {
	nodes := make(map[int64]*v1.CategoryResponse, len(items))
	order := make([]int64, 0, len(items))
	for _, item := range items {
		response := toCategoryResponse(item)
		nodes[item.ID] = &response
		order = append(order, item.ID)
	}
	roots := make([]v1.CategoryResponse, 0)
	for _, id := range order {
		node := nodes[id]
		if node.ParentID == 0 {
			roots = append(roots, *node)
			continue
		}
		parent, ok := nodes[node.ParentID]
		if !ok {
			roots = append(roots, *node)
			continue
		}
		parent.Children = append(parent.Children, *node)
	}
	return roots
}

func toTagResponse(item internaltaxonomy.Tag) v1.TagResponse {
	return v1.TagResponse{
		ID:            item.ID,
		Name:          item.Name,
		Slug:          item.Slug,
		DocumentCount: item.DocumentCount,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toListTagsResponse(items []internaltaxonomy.Tag) v1.ListTagsResponse {
	responses := make([]v1.TagResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toTagResponse(item))
	}
	return v1.ListTagsResponse{Items: responses}
}
