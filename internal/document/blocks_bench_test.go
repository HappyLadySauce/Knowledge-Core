package document

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func makeBenchBlocks(n int) []Block {
	blocks := make([]Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = Block{
			BlockID:     fmt.Sprintf("blk_bench_%d", i),
			PositionKey: fmt.Sprintf("%08d", i+1),
			Type:        BlockTypeParagraph,
			ContentJSON: paragraphContentJSON(fmt.Sprintf("benchmark paragraph content number %d", i)),
			TextContent: fmt.Sprintf("benchmark paragraph content number %d", i),
			Version:     int64(i),
			UpdatedBy:   1,
			UpdatedAt:   time.Now().UTC(),
		}
	}
	return blocks
}

func BenchmarkMarkdownToBlocks(b *testing.B) {
	markdown := strings.Repeat("paragraph content line\n\n", 50)
	now := time.Now().UTC()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = markdownToBlocks(markdown, 1, now)
	}
}

func BenchmarkBlocksToMarkdown(b *testing.B) {
	blocks := makeBenchBlocks(50)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = blocksToMarkdown(blocks)
	}
}

func BenchmarkEncodeBlocksSnapshot(b *testing.B) {
	blocks := makeBenchBlocks(50)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = encodeBlocksSnapshot(blocks)
	}
}

func BenchmarkDecodeBlocksSnapshot(b *testing.B) {
	blocks := makeBenchBlocks(50)
	snapshot, err := encodeBlocksSnapshot(blocks)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(snapshot)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = decodeBlocksSnapshot(snapshot)
	}
}

func BenchmarkApplyBlockOperation(b *testing.B) {
	now := time.Now().UTC()
	block := Block{
		BlockID:     "blk_bench",
		PositionKey: "00000001",
		Type:        BlockTypeParagraph,
		ContentJSON: paragraphContentJSON("initial"),
		TextContent: "initial",
		Version:     1,
	}
	payload, _ := json.Marshal(map[string]string{"text_content": "updated content"})
	op := Operation{
		OpID:                 "op-bench",
		BlockID:              "blk_bench",
		ExpectedBlockVersion: 1,
		Type:                 OpTypeUpdateBlock,
		PayloadJSON:          string(payload),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyBlockOperation(block, op, 1, now)
	}
}
