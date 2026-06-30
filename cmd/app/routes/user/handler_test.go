package user

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"

	authroute "github.com/HappyLadySauce/Knowledge-Core/cmd/app/routes/auth"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internalauth "github.com/HappyLadySauce/Knowledge-Core/internal/auth"
	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
	internaluser "github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

func TestMeAndProfileUpdate(t *testing.T) {
	harness := newUserHarness(t)
	token := harness.registerUser(t, "profile-user")

	me := harness.request(t, http.MethodGet, "/api/v1/users/me", nil, token.AccessToken)
	current := decodeEnvelopeData[v1.UserResponse](t, me, http.StatusOK, apperrors.MessageOK)
	if current.Username != "profile-user" {
		t.Fatalf("unexpected current user: %+v", current)
	}

	update := harness.request(t, http.MethodPut, "/api/v1/users/me", map[string]any{
		"username": "Profile-Renamed",
		"email":    "profile@example.com",
		"avatar":   "https://example.com/avatar.png",
		"bio":      "hello",
	}, token.AccessToken)
	updated := decodeEnvelopeData[v1.UserResponse](t, update, http.StatusOK, apperrors.MessageOK)
	if updated.Username != "profile-renamed" || updated.Email != "profile@example.com" || updated.Avatar == "" || updated.Bio != "hello" {
		t.Fatalf("unexpected updated profile: %+v", updated)
	}
}

func TestChangePasswordRevokesRefreshToken(t *testing.T) {
	harness := newUserHarness(t)
	token := harness.registerUser(t, "password-user")

	badPassword := harness.request(t, http.MethodPut, "/api/v1/users/me/password", map[string]any{
		"old_password": "wrong-password",
		"new_password": "NewPass_123",
	}, token.AccessToken)
	decodeEnvelopeData[any](t, badPassword, http.StatusUnauthorized, apperrors.MessageUnauthorized)

	changed := harness.request(t, http.MethodPut, "/api/v1/users/me/password", map[string]any{
		"old_password": "StrongPass_123",
		"new_password": "NewPass_123",
	}, token.AccessToken)
	decodeEnvelopeData[any](t, changed, http.StatusOK, apperrors.MessageOK)

	refresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": token.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, refresh, http.StatusUnauthorized, apperrors.MessageUnauthorized)

	oldLogin := harness.login(t, "password-user", "StrongPass_123")
	decodeEnvelopeData[any](t, oldLogin, http.StatusUnauthorized, apperrors.MessageUnauthorized)
	newLogin := harness.login(t, "password-user", "NewPass_123")
	decodeEnvelopeData[v1.TokenResponse](t, newLogin, http.StatusOK, apperrors.MessageOK)
}

func TestAdminListGetUpdateDeleteAndResetPassword(t *testing.T) {
	harness := newUserHarness(t)
	userToken := harness.registerUserWithEmail(t, "managed-user", "managed@example.com")
	adminToken := harness.loginAdmin(t)

	list := harness.request(t, http.MethodGet, "/api/v1/admin/users?page=1&page_size=10&role=user&status=active&keyword=managed", nil, adminToken.AccessToken)
	listData := decodeEnvelopeData[v1.ListUsersResponse](t, list, http.StatusOK, apperrors.MessageOK)
	if listData.Total != 1 || len(listData.Items) != 1 || listData.Items[0].Username != "managed-user" {
		t.Fatalf("unexpected list result: %+v", listData)
	}

	get := harness.request(t, http.MethodGet, "/api/v1/admin/users/"+itoa(userToken.User.ID), nil, adminToken.AccessToken)
	got := decodeEnvelopeData[v1.UserResponse](t, get, http.StatusOK, apperrors.MessageOK)
	if got.ID != userToken.User.ID {
		t.Fatalf("unexpected fetched user: %+v", got)
	}

	update := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(userToken.User.ID), map[string]any{
		"username": "managed-renamed",
		"email":    "",
		"avatar":   "avatar",
		"bio":      "bio",
		"role":     internaluser.RoleAdmin,
		"status":   internaluser.StatusActive,
	}, adminToken.AccessToken)
	updated := decodeEnvelopeData[v1.UserResponse](t, update, http.StatusOK, apperrors.MessageOK)
	if updated.Username != "managed-renamed" || updated.Email != "" || updated.Role != internaluser.RoleAdmin || updated.Avatar != "avatar" || updated.Bio != "bio" {
		t.Fatalf("unexpected updated user: %+v", updated)
	}

	refreshAfterRoleChange := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": userToken.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, refreshAfterRoleChange, http.StatusUnauthorized, apperrors.MessageUnauthorized)

	reset := harness.request(t, http.MethodPut, "/api/v1/admin/users/"+itoa(userToken.User.ID)+"/password", map[string]any{
		"password": "ResetPass_123",
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, reset, http.StatusOK, apperrors.MessageOK)
	resetLogin := harness.login(t, "managed-renamed", "ResetPass_123")
	decodeEnvelopeData[v1.TokenResponse](t, resetLogin, http.StatusOK, apperrors.MessageOK)

	deleted := harness.request(t, http.MethodDelete, "/api/v1/admin/users/"+itoa(userToken.User.ID), nil, adminToken.AccessToken)
	decodeEnvelopeData[any](t, deleted, http.StatusOK, apperrors.MessageOK)
	disabledLogin := harness.login(t, "managed-renamed", "ResetPass_123")
	decodeEnvelopeData[any](t, disabledLogin, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestAdminUserManagementRejectsInvalidAndForbiddenCases(t *testing.T) {
	harness := newUserHarness(t)
	first := harness.registerUserWithEmail(t, "first-user", "first@example.com")
	second := harness.registerUserWithEmail(t, "second-user", "second@example.com")
	adminToken := harness.loginAdmin(t)

	forbidden := harness.request(t, http.MethodGet, "/api/v1/admin/users", nil, first.AccessToken)
	decodeEnvelopeData[any](t, forbidden, http.StatusForbidden, apperrors.MessageForbidden)

	empty := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, empty, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	badStatus := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{"status": "pending"}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, badStatus, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	badRole := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{"role": "owner"}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, badRole, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	duplicateEmail := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{"email": second.User.Email}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, duplicateEmail, http.StatusConflict, apperrors.MessageConflict)

	selfRole := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{"role": internaluser.RoleUser}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, selfRole, http.StatusForbidden, apperrors.MessageForbidden)

	selfDelete := harness.request(t, http.MethodDelete, "/api/v1/admin/users/"+itoa(adminToken.User.ID), nil, adminToken.AccessToken)
	decodeEnvelopeData[any](t, selfDelete, http.StatusForbidden, apperrors.MessageForbidden)

	selfReset := harness.request(t, http.MethodPut, "/api/v1/admin/users/"+itoa(adminToken.User.ID)+"/password", map[string]any{"password": "ResetPass_123"}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, selfReset, http.StatusForbidden, apperrors.MessageForbidden)
}

func TestRejectsRemovingLastActiveAdmin(t *testing.T) {
	harness := newUserHarness(t)
	adminToken := harness.loginAdmin(t)

	disableLastAdmin := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"status": internaluser.StatusDisabled,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, disableLastAdmin, http.StatusForbidden, apperrors.MessageForbidden)

	demoteLastAdmin := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"role": internaluser.RoleUser,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, demoteLastAdmin, http.StatusForbidden, apperrors.MessageForbidden)
}

type userHarness struct {
	router *gin.Engine
}

func newUserHarness(t *testing.T) *userHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, jwtOptions := newTestDB(t)
	sc := &svc.ServiceContext{
		Config: &config.Config{JWT: jwtOptions},
		DB:     db,
	}
	authSvc := internalauth.NewService(db, jwtOptions)
	// Bootstrap admin so loginAdmin can authenticate.
	// 引导创建 admin 用户，使 loginAdmin 可认证。
	t.Setenv("KNOWLEDGE_CORE_ADMIN_PASSWORD", "ChangeMe_123456!")
	if err := authSvc.EnsureAdmin(context.Background()); err != nil {
		t.Fatalf("bootstrap admin failed: %v", err)
	}
	router := gin.New()
	group := router.Group("/api/v1")
	authroute.RegisterRoutes(group, authSvc, sc)
	RegisterRoutes(group, internaluser.NewService(db), sc)
	return &userHarness{router: router}
}

func (h *userHarness) registerUser(t *testing.T, username string) v1.TokenResponse {
	t.Helper()
	return h.registerUserWithEmail(t, username, "")
}

func (h *userHarness) registerUserWithEmail(t *testing.T, username, email string) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": username,
		"password": "StrongPass_123",
		"email":    email,
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusCreated, apperrors.MessageOK)
}

func (h *userHarness) loginAdmin(t *testing.T) v1.TokenResponse {
	t.Helper()
	response := h.login(t, "admin", "ChangeMe_123456!")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusOK, apperrors.MessageOK)
}

func (h *userHarness) login(t *testing.T, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return h.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": password,
	}, "")
}

func (h *userHarness) request(t *testing.T, method, path string, body any, accessToken string) *httptest.ResponseRecorder {
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
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "user.db")))
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

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
