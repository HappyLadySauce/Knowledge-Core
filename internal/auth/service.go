package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
)

const bcryptCost = 12

// Service implements auth use cases.
// Service 实现认证相关用例。
type Service struct {
	repo   *Repository
	tokens *tokenManager
	jwt    *options.JWTOptions
}

// NewService creates an auth service.
// NewService 创建认证服务。
func NewService(db *sql.DB, jwtOptions *options.JWTOptions) *Service {
	return &Service{
		repo:   NewRepository(db),
		tokens: newTokenManager(jwtOptions),
		jwt:    jwtOptions,
	}
}

// Register creates an active user account and immediately issues tokens.
// Register 创建 active 普通用户账号并立即签发令牌。
func (s *Service) Register(ctx context.Context, req RegisterCommand) (TokenResponse, error) {
	username := normalizeUsername(req.Username)
	password := strings.TrimSpace(req.Password)
	email := strings.TrimSpace(req.Email)
	if username == "" || password == "" {
		return TokenResponse{}, apperrors.InvalidRequest
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	user, err := s.repo.createUser(ctx, username, email, string(hash))
	if err != nil {
		return TokenResponse{}, err
	}
	return s.issueTokenResponse(ctx, user)
}

// Login verifies credentials and issues tokens.
// Login 校验凭据并签发令牌。
func (s *Service) Login(ctx context.Context, req LoginCommand) (TokenResponse, error) {
	record, err := s.repo.getUserRecordByUsername(ctx, normalizeUsername(req.Username))
	if err != nil {
		return TokenResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(req.Password)) != nil {
		return TokenResponse{}, apperrors.InvalidCredentials
	}
	if record.Status != StatusActive {
		return TokenResponse{}, apperrors.UserDisabled
	}
	return s.issueTokenResponse(ctx, record.User)
}

// Refresh rotates a refresh token and issues a new token response.
// Refresh 轮换刷新令牌并签发新的令牌响应。
func (s *Service) Refresh(ctx context.Context, req RefreshCommand) (TokenResponse, error) {
	plain := strings.TrimSpace(req.RefreshToken)
	if plain == "" {
		return TokenResponse{}, apperrors.InvalidToken
	}
	newPlain, newHash, err := newRefreshToken()
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	user, err := s.repo.rotateRefreshToken(ctx, refreshTokenHash(plain), newHash, time.Now().UTC().Add(s.jwt.RefreshTTL))
	if err != nil {
		return TokenResponse{}, err
	}
	accessToken, expiresIn, err := s.tokens.issueAccessToken(user)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return TokenResponse{
		AccessToken:  accessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    expiresIn,
		RefreshToken: newPlain,
		Scope:        scopeForUser(user),
		User:         user,
	}, nil
}

// CurrentUser returns the active user behind an access token.
// CurrentUser 返回访问令牌对应的 active 用户。
func (s *Service) CurrentUser(ctx context.Context, rawToken string) (User, error) {
	claims, err := s.tokens.parseAccessToken(rawToken)
	if err != nil {
		return User{}, err
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return User{}, apperrors.InvalidToken
	}
	user, err := s.repo.getUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if user.Status != StatusActive {
		return User{}, apperrors.UserDisabled
	}
	return user, nil
}

// UpdateUser changes user profile, role, or status under admin rules.
// UpdateUser 按 admin 规则修改用户资料、角色或状态。
func (s *Service) UpdateUser(ctx context.Context, actor User, targetID int64, cmd UpdateUserCommand) (User, error) {
	if actor.Role != RoleAdmin {
		return User{}, apperrors.Forbidden
	}
	cmd = normalizeUpdateUserCommand(cmd)
	if cmd.Username == nil && cmd.Email == nil && cmd.Status == nil && cmd.Role == nil {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Status != nil && *cmd.Status != StatusActive && *cmd.Status != StatusDisabled {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Role != nil && *cmd.Role != RoleAdmin && *cmd.Role != RoleUser {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Username != nil && *cmd.Username == "" {
		return User{}, apperrors.InvalidRequest
	}
	if actor.ID == targetID && (cmd.Status != nil || cmd.Role != nil) {
		return User{}, fmt.Errorf("%w: admin cannot change own role or status", apperrors.Forbidden)
	}

	current, err := s.repo.getUserByID(ctx, targetID)
	if err != nil {
		return User{}, err
	}
	if current.Role == RoleAdmin && current.Status == StatusActive {
		wouldLoseActiveAdmin := false
		if cmd.Status != nil && *cmd.Status == StatusDisabled {
			wouldLoseActiveAdmin = true
		}
		if cmd.Role != nil && *cmd.Role != RoleAdmin {
			wouldLoseActiveAdmin = true
		}
		if wouldLoseActiveAdmin {
			count, err := s.repo.countActiveAdmins(ctx)
			if err != nil {
				return User{}, err
			}
			if count <= 1 {
				return User{}, apperrors.Forbidden
			}
		}
	}
	return s.repo.updateUser(ctx, targetID, cmd)
}

func (s *Service) issueTokenResponse(ctx context.Context, user User) (TokenResponse, error) {
	accessToken, expiresIn, err := s.tokens.issueAccessToken(user)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	refreshPlain, refreshHash, err := newRefreshToken()
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := s.repo.storeRefreshToken(ctx, user.ID, refreshHash, time.Now().UTC().Add(s.jwt.RefreshTTL)); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken:  accessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshPlain,
		Scope:        scopeForUser(user),
		User:         user,
	}, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func normalizeUpdateUserCommand(cmd UpdateUserCommand) UpdateUserCommand {
	if cmd.Username != nil {
		username := normalizeUsername(*cmd.Username)
		cmd.Username = &username
	}
	if cmd.Email != nil {
		email := strings.TrimSpace(*cmd.Email)
		cmd.Email = &email
	}
	if cmd.Status != nil {
		status := strings.TrimSpace(*cmd.Status)
		cmd.Status = &status
	}
	if cmd.Role != nil {
		role := strings.TrimSpace(*cmd.Role)
		cmd.Role = &role
	}
	return cmd
}

func scopeForUser(user User) string {
	return "role:" + user.Role
}
