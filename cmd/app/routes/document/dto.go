package document

import (
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internaldocument "github.com/HappyLadySauce/Knowledge-Core/internal/document"
)

func toDocumentResponse(item internaldocument.Document) v1.DocumentResponse {
	response := v1.DocumentResponse{
		ID:             item.ID,
		Slug:           item.Slug,
		Title:          item.Title,
		Summary:        item.Summary,
		CategoryID:     item.CategoryID,
		Tags:           toDocumentTagResponses(item.Tags),
		Source:         item.Source,
		Status:         item.Status,
		Confidence:     item.Confidence,
		WordCount:      item.WordCount,
		CoverURL:       item.CoverURL,
		AuthorID:       item.AuthorID,
		CurrentVersion: item.CurrentVersion,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		PublishedAt:    item.PublishedAt,
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
	response.Blocks = toDocumentBlockResponses(item.Blocks)
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

func toDocumentBlockResponses(blocks []internaldocument.Block) []v1.DocumentBlockResponse {
	items := make([]v1.DocumentBlockResponse, 0, len(blocks))
	for _, block := range blocks {
		items = append(items, v1.DocumentBlockResponse{
			BlockID:     block.BlockID,
			ParentID:    block.ParentID,
			PositionKey: block.PositionKey,
			Type:        block.Type,
			ContentJSON: block.ContentJSON,
			TextContent: block.TextContent,
			Version:     block.Version,
			UpdatedBy:   block.UpdatedBy,
			UpdatedAt:   block.UpdatedAt,
		})
	}
	return items
}

func toBlockInputs(items []v1.DocumentBlockInput) []internaldocument.BlockInput {
	blocks := make([]internaldocument.BlockInput, 0, len(items))
	for _, item := range items {
		blocks = append(blocks, internaldocument.BlockInput{
			BlockID:     item.BlockID,
			ParentID:    item.ParentID,
			PositionKey: item.PositionKey,
			Type:        item.Type,
			ContentJSON: item.ContentJSON,
			TextContent: item.TextContent,
		})
	}
	return blocks
}

func toOperations(items []v1.DocumentOperationRequest) []internaldocument.Operation {
	ops := make([]internaldocument.Operation, 0, len(items))
	for _, item := range items {
		ops = append(ops, internaldocument.Operation{
			OpID:                 item.OpID,
			BaseDocumentVersion:  item.BaseDocumentVersion,
			BlockID:              item.BlockID,
			ExpectedBlockVersion: item.ExpectedBlockVersion,
			Type:                 item.Type,
			PayloadJSON:          item.PayloadJSON,
		})
	}
	return ops
}

func toApplyOpsResponse(result internaldocument.ApplyOpsResult) v1.ApplyDocumentOpsResponse {
	acks := make([]v1.DocumentOperationAckResponse, 0, len(result.Acks))
	for _, ack := range result.Acks {
		acks = append(acks, v1.DocumentOperationAckResponse{
			OpID:            ack.OpID,
			DocumentID:      ack.DocumentID,
			DocumentVersion: ack.DocumentVersion,
			BlockID:         ack.BlockID,
			BlockVersion:    ack.BlockVersion,
		})
	}
	conflicts := make([]v1.DocumentOperationConflictResponse, 0, len(result.Conflicts))
	for _, conflict := range result.Conflicts {
		conflicts = append(conflicts, v1.DocumentOperationConflictResponse{
			OpID:            conflict.OpID,
			DocumentID:      conflict.DocumentID,
			DocumentVersion: conflict.DocumentVersion,
			Block:           toDocumentBlockResponses([]internaldocument.Block{conflict.Block})[0],
		})
	}
	document := toDocumentResponse(result.Document)
	document.Blocks = toDocumentBlockResponses(result.Blocks)
	return v1.ApplyDocumentOpsResponse{
		Acks:           acks,
		Conflicts:      conflicts,
		Document:       document,
		CurrentVersion: result.Document.CurrentVersion,
	}
}
