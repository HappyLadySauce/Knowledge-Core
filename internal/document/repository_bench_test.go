package document

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
	"github.com/HappyLadySauce/Knowledge-Core/internal/testutil"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

func newDocumentBenchDB(b testing.TB) *sql.DB {
	b.Helper()
	db := testutil.NewPostgresDB(b)
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO users (username, email, avatar, bio, password_hash, role, status, token_version, created_at, updated_at)
VALUES ('admin', '', '', '', '', 'admin', 'active', 0, $1, $2)`,
		time.Now().UTC(), time.Now().UTC()); err != nil {
		b.Fatalf("insert bench admin failed: %v", err)
	}
	return db
}

func BenchmarkDocumentCreate(b *testing.B) {
	ctx := context.Background()
	db := newDocumentBenchDB(b)
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		b.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.CreateAdmin(ctx, admin, CreateCommand{
			Slug:       fmt.Sprintf("bench-create-%d", i),
			Title:      "Bench Create",
			Content:    "benchmark body content",
			CategoryID: category.ID,
			Status:     StatusPublished,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDocumentGetByID(b *testing.B) {
	ctx := context.Background()
	db := newDocumentBenchDB(b)
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		b.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	created, err := service.CreateAdmin(ctx, admin, CreateCommand{
		Slug:       "bench-get",
		Title:      "Bench Get",
		Content:    "benchmark body content",
		CategoryID: category.ID,
		Status:     StatusPublished,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetAdmin(ctx, admin, created.ID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDocumentList(b *testing.B) {
	ctx := context.Background()
	db := newDocumentBenchDB(b)
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		b.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	for i := 0; i < 30; i++ {
		_, err := service.CreateAdmin(ctx, admin, CreateCommand{
			Slug:       fmt.Sprintf("bench-list-%d", i),
			Title:      "Bench List",
			Content:    "benchmark body content",
			CategoryID: category.ID,
			Status:     StatusPublished,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ListPublic(ctx, ListQuery{Page: 1, PageSize: 20})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDocumentApplyOps(b *testing.B) {
	ctx := context.Background()
	db := newDocumentBenchDB(b)
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(ctx, taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		b.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	admin := user.User{ID: 1, Role: user.RoleAdmin}
	created, err := service.CreateAdmin(ctx, admin, CreateCommand{
		Slug:       "bench-ops",
		Title:      "Bench Ops",
		Content:    "initial body",
		CategoryID: category.ID,
	})
	if err != nil {
		b.Fatal(err)
	}
	block := created.Blocks[0]
	blockVersion := block.Version
	docVersion := created.CurrentVersion
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := Operation{
			OpID:                 fmt.Sprintf("bench-op-%d", i),
			BaseDocumentVersion:  docVersion,
			BlockID:              block.BlockID,
			ExpectedBlockVersion: blockVersion,
			Type:                 OpTypeUpdateBlock,
			PayloadJSON:          `{"text_content":"updated"}`,
		}
		result, err := service.ApplyOpsAdmin(ctx, admin, created.ID, ApplyOpsCommand{Ops: []Operation{op}})
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Acks) > 0 {
			blockVersion = result.Acks[0].BlockVersion
			docVersion = result.Acks[0].DocumentVersion
		}
	}
}
