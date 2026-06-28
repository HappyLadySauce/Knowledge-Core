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
	"strconv"
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
)

func TestRegisterLogsUserInAndRejectsAdminRoleInjection(t *testing.T) {
	harness := newAuthHarness(t)

	response := harness.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "Alice",
		"password": "StrongPass_123",
		"email":    "alice@example.com",
		"role":     internalauth.RoleAdmin,
	}, "")
	if response.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", response.Code, response.Body.String())
	}

	token := decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusCreated, apperrors.MessageOK)
	if token.AccessToken == "" || token.RefreshToken == "" || token.TokenType != internalauth.TokenTypeBearer {
		t.Fatalf("register should return OAuth2 token response in data: %+v", token)
	}
	if token.User.Role != internalauth.RoleUser || token.User.Status != internalauth.StatusActive {
		t.Fatalf("registered user should be active normal user: %+v", token.User)
	}
	if token.Scope != "role:user" {
		t.Fatalf("scope = %q", token.Scope)
	}
}

func TestDefaultAdminCanLogin(t *testing.T) {
	harness := newAuthHarness(t)

	response := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin",
		"password": "ChangeMe_123456!",
	}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, body = %s", response.Code, response.Body.String())
	}

	token := decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusOK, apperrors.MessageOK)
	if token.User.Role != internalauth.RoleAdmin || token.Scope != "role:admin" {
		t.Fatalf("admin login returned wrong user/scope: %+v", token)
	}
}

func TestRefreshRotatesTokenAndRejectsOldToken(t *testing.T) {
	harness := newAuthHarness(t)
	token := harness.registerUser(t, "refresh-user")

	firstRefresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": token.RefreshToken,
	}, "")
	if firstRefresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", firstRefresh.Code, firstRefresh.Body.String())
	}

	rotated := decodeEnvelopeData[v1.TokenResponse](t, firstRefresh, http.StatusOK, apperrors.MessageOK)
	if rotated.RefreshToken == "" || rotated.RefreshToken == token.RefreshToken {
		t.Fatalf("refresh token was not rotated")
	}

	oldRefresh := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": token.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, oldRefresh, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestAdminCanUpdateUserProfileStatusAndRole(t *testing.T) {
	harness := newAuthHarness(t)
	userToken := harness.registerUser(t, "managed-user")
	adminToken := harness.loginAdmin(t)

	updateResponse := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(userToken.User.ID), map[string]any{
		"username": "managed-user-renamed",
		"email":    "managed@example.com",
		"status":   internalauth.StatusActive,
		"role":     internalauth.RoleAdmin,
	}, adminToken.AccessToken)
	updated := decodeEnvelopeData[v1.UserResponse](t, updateResponse, http.StatusOK, apperrors.MessageOK)
	if updated.Username != "managed-user-renamed" || updated.Email != "managed@example.com" ||
		updated.Status != internalauth.StatusActive || updated.Role != internalauth.RoleAdmin {
		t.Fatalf("unexpected updated user: %+v", updated)
	}
}

func TestAdminDisableUserRevokesRefreshToken(t *testing.T) {
	harness := newAuthHarness(t)
	userToken := harness.registerUser(t, "status-user")
	adminToken := harness.loginAdmin(t)

	disableResponse := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(userToken.User.ID), map[string]any{
		"status": internalauth.StatusDisabled,
	}, adminToken.AccessToken)
	decodeEnvelopeData[v1.UserResponse](t, disableResponse, http.StatusOK, apperrors.MessageOK)

	loginDisabled := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "status-user",
		"password": "StrongPass_123",
	}, "")
	decodeEnvelopeData[any](t, loginDisabled, http.StatusUnauthorized, apperrors.MessageUnauthorized)

	refreshDisabled := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": userToken.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, refreshDisabled, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestAdminRoleChangeRevokesRefreshToken(t *testing.T) {
	harness := newAuthHarness(t)
	userToken := harness.registerUser(t, "role-user")
	adminToken := harness.loginAdmin(t)

	roleResponse := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(userToken.User.ID), map[string]any{
		"role": internalauth.RoleAdmin,
	}, adminToken.AccessToken)
	decodeEnvelopeData[v1.UserResponse](t, roleResponse, http.StatusOK, apperrors.MessageOK)

	refreshAfterRoleChange := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": userToken.RefreshToken,
	}, "")
	decodeEnvelopeData[any](t, refreshAfterRoleChange, http.StatusUnauthorized, apperrors.MessageUnauthorized)
}

func TestNonAdminCannotUpdateUserAndAdminCannotChangeOwnRoleOrStatus(t *testing.T) {
	harness := newAuthHarness(t)
	userToken := harness.registerUser(t, "normal-user")
	adminToken := harness.loginAdmin(t)

	forbidden := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(userToken.User.ID), map[string]any{
		"status": internalauth.StatusDisabled,
	}, userToken.AccessToken)
	decodeEnvelopeData[any](t, forbidden, http.StatusForbidden, apperrors.MessageForbidden)

	selfDisable := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"status": internalauth.StatusDisabled,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, selfDisable, http.StatusForbidden, apperrors.MessageForbidden)

	selfRole := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"role": internalauth.RoleUser,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, selfRole, http.StatusForbidden, apperrors.MessageForbidden)
}

func TestAdminCanUpdateOwnProfile(t *testing.T) {
	harness := newAuthHarness(t)
	adminToken := harness.loginAdmin(t)

	response := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"username": "admin-renamed",
		"email":    "admin@example.com",
	}, adminToken.AccessToken)
	updated := decodeEnvelopeData[v1.UserResponse](t, response, http.StatusOK, apperrors.MessageOK)
	if updated.Username != "admin-renamed" || updated.Email != "admin@example.com" {
		t.Fatalf("unexpected self profile update: %+v", updated)
	}
}

func TestBadRequestsAndDuplicateRegisterReturnEnvelope(t *testing.T) {
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
}

func TestUpdateUserRejectsEmptyBodyInvalidEnumsAndDuplicateFields(t *testing.T) {
	harness := newAuthHarness(t)
	first := harness.registerUser(t, "first-user")
	second := harness.registerUserWithEmail(t, "second-user", "second@example.com")
	adminToken := harness.loginAdmin(t)

	empty := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, empty, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	badStatus := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{
		"status": "pending",
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, badStatus, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	badRole := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{
		"role": "owner",
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, badRole, http.StatusBadRequest, apperrors.MessageInvalidRequest)

	duplicateUsername := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{
		"username": second.User.Username,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, duplicateUsername, http.StatusConflict, apperrors.MessageConflict)

	duplicateEmail := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(first.User.ID), map[string]any{
		"email": "second@example.com",
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, duplicateEmail, http.StatusConflict, apperrors.MessageConflict)
}

func TestRejectsDisablingOrDemotingLastActiveAdmin(t *testing.T) {
	harness := newAuthHarness(t)
	adminToken := harness.loginAdmin(t)

	disableLastAdmin := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"status": internalauth.StatusDisabled,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, disableLastAdmin, http.StatusForbidden, apperrors.MessageForbidden)

	demoteLastAdmin := harness.request(t, http.MethodPatch, "/api/v1/admin/users/"+itoa(adminToken.User.ID), map[string]any{
		"role": internalauth.RoleUser,
	}, adminToken.AccessToken)
	decodeEnvelopeData[any](t, demoteLastAdmin, http.StatusForbidden, apperrors.MessageForbidden)
}

func TestJWTSecretValidationRejectsShortSecret(t *testing.T) {
	opts := options.NewJWTOptions()
	opts.Issuer = "Knowledge-Core"
	opts.Secret = "short"
	opts.AccessTTL = time.Minute
	opts.RefreshTTL = time.Hour

	if err := opts.Validate(); err == nil {
		t.Fatalf("expected short jwt secret to fail validation")
	}
}

func TestJWTSecretValidationRejectsEmptySecret(t *testing.T) {
	opts := options.NewJWTOptions()
	opts.Issuer = "Knowledge-Core"
	opts.AccessTTL = time.Minute
	opts.RefreshTTL = time.Hour

	if err := opts.Validate(); err == nil {
		t.Fatalf("expected empty jwt secret to fail validation")
	}
}

type authHarness struct {
	router *gin.Engine
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

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
	applyAuthMigration(t, db)

	jwtOptions := &options.JWTOptions{
		Issuer:     "Knowledge-Core",
		Secret:     "Knowledge-Core-test-secret-32bytes",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
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

func (h *authHarness) registerUserWithEmail(t *testing.T, username, email string) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": username,
		"password": "StrongPass_123",
		"email":    email,
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusCreated, apperrors.MessageOK)
}

func (h *authHarness) loginAdmin(t *testing.T) v1.TokenResponse {
	t.Helper()
	response := h.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin",
		"password": "ChangeMe_123456!",
	}, "")
	return decodeEnvelopeData[v1.TokenResponse](t, response, http.StatusOK, apperrors.MessageOK)
}

func (h *authHarness) request(t *testing.T, method, path string, body any, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
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

func applyAuthMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	root := repoRootFromWorkingDir(t)
	body, err := os.ReadFile(filepath.Join(root, "sql", "migrations", "002_auth_users.sql"))
	if err != nil {
		t.Fatalf("read auth migration failed: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), string(body)); err != nil {
		t.Fatalf("apply auth migration failed: %v", err)
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
