package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internalauth "github.com/HappyLadySauce/Knowledge-Core/internal/auth"
	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
	internaluser "github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

func TestRegisterLoginAndRefreshReturnTokenEnvelope(t *testing.T) {
	harness := newAuthHarness(t)

	register := harness.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "Alice",
		"password": "StrongPass_123",
		"email":    "alice@example.com",
		"role":     internaluser.RoleAdmin,
	}, "")
	registered := decodeEnvelopeData[v1.TokenResponse](t, register, http.StatusCreated, apperrors.MessageOK)
	if registered.User.Role != internaluser.RoleUser || registered.User.Status != internaluser.StatusActive {
		t.Fatalf("registered user should be active normal user: %+v", registered.User)
	}
	if registered.User.Avatar != "" || registered.User.Bio != "" {
		t.Fatalf("new user should have empty profile fields: %+v", registered.User)
	}

	login := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "alice",
		"password": "StrongPass_123",
	}, "")
	loggedIn := decodeEnvelopeData[v1.TokenResponse](t, login, http.StatusOK, apperrors.MessageOK)
	if loggedIn.AccessToken == "" || loggedIn.RefreshToken == "" || loggedIn.TokenType != internalauth.TokenTypeBearer {
		t.Fatalf("login should return OAuth2 token response in data: %+v", loggedIn)
	}

	refresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": loggedIn.RefreshToken,
	}, "")
	rotated := decodeEnvelopeData[v1.TokenResponse](t, refresh, http.StatusOK, apperrors.MessageOK)
	if rotated.RefreshToken == "" || rotated.RefreshToken == loggedIn.RefreshToken {
		t.Fatalf("refresh token was not rotated")
	}

	oldRefresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": loggedIn.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, oldRefresh, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestDefaultAdminCanLogin(t *testing.T) {
	harness := newAuthHarness(t)

	response := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin",
		"password": "ChangeMe_123456!",
	}, "")
	token := decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusOK, apperrors.MessageOK)
	if token.User.Role != internaluser.RoleAdmin || token.Scope != "role:admin" {
		t.Fatalf("admin login returned wrong user/scope: %+v", token)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	harness := newAuthHarness(t)
	token := harness.registerUser(t, "logout-user")

	logout := harness.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]any{
		"refresh_token": token.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, logout, http.StatusOK, apperrors.MessageOK)

	refresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": token.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, refresh, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestBadAuthRequestsReturnEnvelope(t *testing.T) {
	harness := newAuthHarness(t)

	badRequest := harness.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "missing-password",
	}, "")
	decodeEnvelopeData[any](t, badRequest, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	_ = harness.registerUser(t, "duplicate-user")
	conflict := harness.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "duplicate-user",
		"password": "StrongPass_123",
	}, "")
	decodeEnvelopeData[any](t, conflict, http.StatusConflict, apperrors.MessageConflict)

	wrongPassword := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "duplicate-user",
		"password": "wrong-password",
	}, "")
	decodeEnvelopeData[any](t, wrongPassword, http.StatusUnauthorized, apperrors.MessageUnauthorized)

	badLogout := harness.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]any{}, "")
	decodeEnvelopeData[any](t, badLogout, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	invalidLogout := harness.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]any{
		"refresh_token": "invalid-refresh-token",
	}, "")
	decodeEnvelopeData[any](t, invalidLogout, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestAuthMeRouteIsRemoved(t *testing.T) {
	harness := newAuthHarness(t)

	response := harness.request(t, http.MethodGet, "/api/v1/auth/me", nil, "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/auth/me status = %d, want 404", response.Code)
	}
}

func TestJWTSecretValidationRejectsShortOrEmptySecret(t *testing.T) {
	for _, secret := range []string{"", "short"} {
		opts := options.NewJWTOptions()
		opts.Secret = secret
		opts.AccessTTL = time.Minute
		opts.RefreshTTL = time.Hour
		if err := opts.Validate(); err == nil {
			t.Fatalf("expected jwt secret %q to fail validation", secret)
		}
	}
}

type authHarness struct {
	router *gin.Engine
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, jwtOptions := newTestDB(t)
	sc := &svc.ServiceContext{
		Config: &config.Config{JWT: jwtOptions},
		DB:     db,
	}
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), internalauth.NewService(db, jwtOptions), sc)
	return &authHarness{router: router}
}

func (h *authHarness) registerUser(t *testing.T, username string) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": username,
		"password": "StrongPass_123",
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusCreated, apperrors.MessageOK)
}

func (h *authHarness) request(t *testing.T, method, path string, body any, accessToken string) *httptest.ResponseRecorder {
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

func newTestDB(t *testing.T) (*sql.DB, *options.JWTOptions) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "auth.db")))
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys failed: %v", err)
	}
	applyUsersMigration(t, db)
	return db, &options.JWTOptions{
		Issuer:     "Knowledge-Core",
		Secret:     "Knowledge-Core-test-secret-32bytes",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
}

func applyUsersMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	root := repoRootFromWorkingDir(t)
	body, err := os.ReadFile(filepath.Join(root, "sql", "migrations", "001_users.sql"))
	if err != nil {
		t.Fatalf("read users migration failed: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), string(body)); err != nil {
		t.Fatalf("apply users migration failed: %v", err)
	}
}

func repoRootFromWorkingDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}

type responseEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func decodeEnvelopeData[T any](t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantMessage string) T {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, wantStatus, response.Body.String())
	}
	var envelope responseEnvelope[T]
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response failed: %v; body = %s", err, response.Body.String())
	}
	if envelope.Code != wantStatus {
		t.Fatalf("envelope code = %d, want %d", envelope.Code, wantStatus)
	}
	if envelope.Message != wantMessage {
		t.Fatalf("envelope message = %q, want %q", envelope.Message, wantMessage)
	}
	return envelope.Data
}
