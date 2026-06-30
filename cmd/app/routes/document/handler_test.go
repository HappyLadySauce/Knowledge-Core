package document

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"

	authroute "github.com/HappyLadySauce/Knowledge-Core/cmd/app/routes/auth"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/common"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internalauth "github.com/HappyLadySauce/Knowledge-Core/internal/auth"
	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
	internaldocument "github.com/HappyLadySauce/Knowledge-Core/internal/document"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
	"github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
)

func TestDocumentHTTPPublishedVisibilityAndAdminAuth(t *testing.T) {
	harness := newDocumentHarness(t)
	adminToken := harness.loginAdmin(t)
	userToken := harness.registerUser(t, "doc-user")

	forbidden := harness.request(t, http.MethodGet, "/api/v1/admin/documents", nil, userToken.AccessToken)
	decodeEnvelopeData[any](t, forbidden, http.StatusForbidden, apperrors.MessageForbidden)

	create := harness.request(t, http.MethodPost, "/api/v1/admin/documents", map[string]any{
		"title":       "Draft Note",
		"summary":     "private",
		"content":     "draft body",
		"category_id": harness.categoryID,
		"tag_ids":     []int64{harness.tagID},
		"status":      internaldocument.StatusDraft,
	}, adminToken.AccessToken)
	created := decodeEnvelopeData[v1.DocumentResponse](t, create, http.StatusCreated, apperrors.MessageOK)

	publicList := harness.request(t, http.MethodGet, "/api/v1/documents", nil, "")
	list := decodeEnvelopeData[v1.ListDocumentsResponse](t, publicList, http.StatusOK, apperrors.MessageOK)
	if list.Total != 0 {
		t.Fatalf("public draft total = %d, want 0", list.Total)
	}

	publish := harness.request(t, http.MethodPatch, "/api/v1/admin/documents/"+itoa(created.ID), map[string]any{
		"status": internaldocument.StatusPublished,
	}, adminToken.AccessToken)
	published := decodeEnvelopeData[v1.DocumentResponse](t, publish, http.StatusOK, apperrors.MessageOK)
	if published.Status != internaldocument.StatusPublished {
		t.Fatalf("status = %s, want published", published.Status)
	}

	publicGet := harness.request(t, http.MethodGet, "/api/v1/documents/"+itoa(created.ID), nil, "")
	publicDoc := decodeEnvelopeData[v1.DocumentResponse](t, publicGet, http.StatusOK, apperrors.MessageOK)
	if publicDoc.Content != "draft body" || len(publicDoc.Tags) != 1 {
		t.Fatalf("unexpected public document: %+v", publicDoc)
	}
}

type documentHarness struct {
	router     *gin.Engine
	categoryID int64
	tagID      int64
}

func newDocumentHarness(t *testing.T) *documentHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := newDocumentRouteTestDB(t)
	libraryRoot := t.TempDir()
	jwtOptions := &options.JWTOptions{
		Issuer:     "Knowledge-Core",
		Secret:     "Knowledge-Core-test-secret-32bytes",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
	sc := &svc.ServiceContext{
		Config: &config.Config{
			JWT:     jwtOptions,
			Library: &options.LibraryOptions{Path: libraryRoot},
		},
		DB: db,
	}
	taxonomies := taxonomy.NewService(db)
	category, err := taxonomies.CreateCategory(context.Background(), taxonomy.CategoryCommand{Name: "Tech", Slug: "tech"})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	tag, err := taxonomies.CreateTag(context.Background(), taxonomy.TagCommand{Name: "Go", Slug: "go"})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	documentService, err := internaldocument.NewService(db, libraryRoot)
	if err != nil {
		t.Fatalf("create document service failed: %v", err)
	}

	router := gin.New()
	group := router.Group("/api/v1")
	authSvc := internalauth.NewService(db, jwtOptions)
	// Bootstrap admin so loginAdmin can authenticate.
	// 引导创建 admin 用户，使 loginAdmin 可认证。
	t.Setenv("KNOWLEDGE_CORE_ADMIN_PASSWORD", "ChangeMe_123456!")
	if err := authSvc.EnsureAdmin(context.Background()); err != nil {
		t.Fatalf("bootstrap admin failed: %v", err)
	}
	authroute.RegisterRoutes(group, authSvc, sc)
	RegisterRoutes(group, documentService, sc)
	return &documentHarness{router: router, categoryID: category.ID, tagID: tag.ID}
}

func (h *documentHarness) loginAdmin(t *testing.T) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin",
		"password": "ChangeMe_123456!",
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusOK, apperrors.MessageOK)
}

func (h *documentHarness) registerUser(t *testing.T, username string) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": username,
		"password": "StrongPass_123",
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusCreated, apperrors.MessageOK)
}

func (h *documentHarness) request(t *testing.T, method, path string, body any, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request failed: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response := httptest.NewRecorder()
	h.router.ServeHTTP(response, req)
	return response
}

func decodeEnvelopeData[T any](t *testing.T, response *httptest.ResponseRecorder, status int, message string) T {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var envelope common.Response[T]
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope failed: %v; body=%s", err, response.Body.String())
	}
	if envelope.Message != message {
		t.Fatalf("message = %s, want %s", envelope.Message, message)
	}
	return envelope.Data
}

func newDocumentRouteTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "document-route.db")))
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys failed: %v", err)
	}
	applyRouteMigrationFiles(t, db)
	return db
}

func applyRouteMigrationFiles(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationsDir := filepath.Join(findRouteRepoRoot(t), "sql", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations directory failed: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		path := filepath.Join(migrationsDir, entry.Name())
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s failed: %v", entry.Name(), err)
		}
		if _, err := db.ExecContext(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s failed: %v", entry.Name(), err)
		}
	}
}

func findRouteRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
