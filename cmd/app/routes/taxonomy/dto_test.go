package taxonomy

import (
	"testing"
	"time"

	internaltaxonomy "github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
)

func TestBuildCategoryTreeKeepsNestedChildren(t *testing.T) {
	now := time.Now().UTC()
	items := []internaltaxonomy.Category{
		{ID: 1, Name: "Tech", Slug: "tech", Path: "tech", CreatedAt: now, UpdatedAt: now},
		{ID: 2, Name: "AI", Slug: "ai", Path: "tech/ai", ParentID: 1, CreatedAt: now, UpdatedAt: now},
		{ID: 3, Name: "LLM", Slug: "llm", Path: "tech/ai/llm", ParentID: 2, CreatedAt: now, UpdatedAt: now},
	}

	tree := buildCategoryTree(items)
	if len(tree) != 1 {
		t.Fatalf("root count = %d, want 1", len(tree))
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("child count = %d, want 1", len(tree[0].Children))
	}
	if len(tree[0].Children[0].Children) != 1 {
		t.Fatalf("grandchild count = %d, want 1", len(tree[0].Children[0].Children))
	}
	if tree[0].Children[0].Children[0].Path != "tech/ai/llm" {
		t.Fatalf("grandchild path = %s, want tech/ai/llm", tree[0].Children[0].Children[0].Path)
	}
}
