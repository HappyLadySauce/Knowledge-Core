package document

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

const (
	BlockTypeParagraph = "paragraph"
	OpTypeUpdateBlock  = "update_block"
	OpTypeMoveBlock    = "move_block"
)

type blockPayload struct {
	ParentID    *string `json:"parent_id,omitempty"`
	PositionKey *string `json:"position_key,omitempty"`
	Type        *string `json:"type,omitempty"`
	ContentJSON *string `json:"content_json,omitempty"`
	TextContent *string `json:"text_content,omitempty"`
}

// markdownToBlocks converts Markdown text into coarse paragraph blocks.
// markdownToBlocks 将 Markdown 文本转换为粗粒度段落块。
func markdownToBlocks(markdown string, actorID int64, now time.Time) []Block {
	parts := splitMarkdownBlocks(markdown)
	blocks := make([]Block, 0, len(parts))
	for i, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		blocks = append(blocks, Block{
			BlockID:     newBlockID(),
			PositionKey: fmt.Sprintf("%08d", i+1),
			Type:        BlockTypeParagraph,
			ContentJSON: paragraphContentJSON(text),
			TextContent: text,
			Version:     1,
			UpdatedBy:   actorID,
			UpdatedAt:   now,
		})
	}
	return blocks
}

func blockInputsToBlocks(inputs []BlockInput, actorID int64, now time.Time) []Block {
	blocks := make([]Block, 0, len(inputs))
	for i, input := range inputs {
		blockID := strings.TrimSpace(input.BlockID)
		if blockID == "" {
			blockID = newBlockID()
		}
		blockType := strings.TrimSpace(input.Type)
		if blockType == "" {
			blockType = BlockTypeParagraph
		}
		text := strings.TrimSpace(input.TextContent)
		contentJSON := strings.TrimSpace(input.ContentJSON)
		if contentJSON == "" {
			contentJSON = paragraphContentJSON(text)
		}
		positionKey := strings.TrimSpace(input.PositionKey)
		if positionKey == "" {
			positionKey = fmt.Sprintf("%08d", i+1)
		}
		blocks = append(blocks, Block{
			BlockID:     blockID,
			ParentID:    strings.TrimSpace(input.ParentID),
			PositionKey: positionKey,
			Type:        blockType,
			ContentJSON: contentJSON,
			TextContent: text,
			Version:     1,
			UpdatedBy:   actorID,
			UpdatedAt:   now,
		})
	}
	return blocks
}

func blocksToMarkdown(blocks []Block) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block.TextContent)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func splitMarkdownBlocks(markdown string) []string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	return strings.Split(normalized, "\n\n")
}

func paragraphContentJSON(text string) string {
	data, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return `{"text":""}`
	}
	return string(data)
}

func newBlockID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("block-%d", time.Now().UnixNano())
	}
	return "blk_" + hex.EncodeToString(b[:])
}

func encodeBlocksSnapshot(blocks []Block) (string, error) {
	type snapshotBlock struct {
		BlockID     string `json:"block_id"`
		ParentID    string `json:"parent_id"`
		PositionKey string `json:"position_key"`
		Type        string `json:"type"`
		ContentJSON string `json:"content_json"`
		TextContent string `json:"text_content"`
		Version     int64  `json:"version"`
	}
	items := make([]snapshotBlock, 0, len(blocks))
	for _, block := range blocks {
		items = append(items, snapshotBlock{
			BlockID:     block.BlockID,
			ParentID:    block.ParentID,
			PositionKey: block.PositionKey,
			Type:        block.Type,
			ContentJSON: block.ContentJSON,
			TextContent: block.TextContent,
			Version:     block.Version,
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeBlocksSnapshot(snapshot string) ([]Block, error) {
	type snapshotBlock struct {
		BlockID     string `json:"block_id"`
		ParentID    string `json:"parent_id"`
		PositionKey string `json:"position_key"`
		Type        string `json:"type"`
		ContentJSON string `json:"content_json"`
		TextContent string `json:"text_content"`
		Version     int64  `json:"version"`
	}
	var items []snapshotBlock
	if err := json.Unmarshal([]byte(snapshot), &items); err != nil {
		return nil, err
	}
	blocks := make([]Block, 0, len(items))
	for _, item := range items {
		blocks = append(blocks, Block{
			BlockID:     item.BlockID,
			ParentID:    item.ParentID,
			PositionKey: item.PositionKey,
			Type:        item.Type,
			ContentJSON: item.ContentJSON,
			TextContent: item.TextContent,
			Version:     item.Version,
		})
	}
	return blocks, nil
}

func applyBlockOperation(block Block, op Operation, actorID int64, now time.Time) (Block, error) {
	if strings.TrimSpace(op.OpID) == "" || strings.TrimSpace(op.BlockID) == "" {
		return Block{}, apperrors.InvalidRequest
	}
	var payload blockPayload
	if strings.TrimSpace(op.PayloadJSON) != "" {
		if err := json.Unmarshal([]byte(op.PayloadJSON), &payload); err != nil {
			return Block{}, apperrors.InvalidRequest
		}
	}
	next := block
	switch op.Type {
	case OpTypeUpdateBlock:
		if payload.Type != nil {
			next.Type = strings.TrimSpace(*payload.Type)
		}
		if payload.ContentJSON != nil {
			next.ContentJSON = strings.TrimSpace(*payload.ContentJSON)
		}
		if payload.TextContent != nil {
			next.TextContent = strings.TrimSpace(*payload.TextContent)
			if payload.ContentJSON == nil {
				next.ContentJSON = paragraphContentJSON(next.TextContent)
			}
		}
	case OpTypeMoveBlock:
		if payload.ParentID != nil {
			next.ParentID = strings.TrimSpace(*payload.ParentID)
		}
		if payload.PositionKey != nil {
			next.PositionKey = strings.TrimSpace(*payload.PositionKey)
		}
	default:
		return Block{}, apperrors.InvalidRequest
	}
	if next.Type == "" || next.PositionKey == "" {
		return Block{}, apperrors.InvalidRequest
	}
	next.Version++
	next.UpdatedBy = actorID
	next.UpdatedAt = now
	return next, nil
}
