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
	nodes := make(map[int64]v1.CategoryResponse, len(items))
	childrenByParent := make(map[int64][]int64, len(items))
	order := make([]int64, 0, len(items))
	for _, item := range items {
		nodes[item.ID] = toCategoryResponse(item)
		order = append(order, item.ID)
	}
	rootIDs := make([]int64, 0)
	for _, id := range order {
		node := nodes[id]
		if node.ParentID == 0 {
			rootIDs = append(rootIDs, id)
			continue
		}
		if _, ok := nodes[node.ParentID]; !ok {
			rootIDs = append(rootIDs, id)
			continue
		}
		childrenByParent[node.ParentID] = append(childrenByParent[node.ParentID], id)
	}

	roots := make([]v1.CategoryResponse, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, buildCategoryBranch(id, nodes, childrenByParent))
	}
	return roots
}

func buildCategoryBranch(id int64, nodes map[int64]v1.CategoryResponse, childrenByParent map[int64][]int64) v1.CategoryResponse {
	node := nodes[id]
	for _, childID := range childrenByParent[id] {
		node.Children = append(node.Children, buildCategoryBranch(childID, nodes, childrenByParent))
	}
	return node
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
