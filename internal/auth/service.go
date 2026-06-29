package auth

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

const bcryptCost = 12

// Service implements auth use cases.
// Service 实现认证服务。
type Service struct {
	users       *user.Repository
	refreshRepo *Repository
	tokens      *tokenManager
	jwt         *options.JWTOptions
}

// NewService creates an auth service.
// NewService 创建认证服务。
func NewService(db *sql.DB, jwtOptions *options.JWTOptions) *Service {
	return &Service{
		users:       user.NewRepository(db),
		refreshRepo: NewRepository(db),
		tokens:      newTokenManager(jwtOptions),
		jwt:         jwtOptions,
	}
}

// Register creates an active user account and immediately issues tokens.
// Register 创建 active 普通用户账号并立即签发令牌。
func (s *Service) Register(ctx context.Context, req RegisterCommand) (TokenResponse, error) {
	username := user.NormalizeUsername(req.Username)
	password := strings.TrimSpace(req.Password)
	email := strings.TrimSpace(req.Email)
	if username == "" || password == "" {
		return TokenResponse{}, apperrors.InvalidRequest
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	created, err := s.users.Create(ctx, username, email, string(hash))
	if err != nil {
		return TokenResponse{}, err
	}
	return s.issueTokenResponse(ctx, created)
}

// Login verifies credentials and issues tokens.
// Login 校验凭据并签发令牌。
func (s *Service) Login(ctx context.Context, req LoginCommand) (TokenResponse, error) {
	record, err := s.users.GetRecordByUsername(ctx, user.NormalizeUsername(req.Username))
	if err != nil {
		return TokenResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(req.Password)) != nil {
		return TokenResponse{}, apperrors.InvalidCredentials
	}
	if record.Status != user.StatusActive {
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
	currentUser, err := s.refreshRepo.rotateRefreshToken(ctx, refreshTokenHash(plain), newHash, time.Now().UTC().Add(s.jwt.RefreshTTL))
	if err != nil {
		return TokenResponse{}, err
	}
	accessToken, expiresIn, err := s.tokens.issueAccessToken(currentUser)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return TokenResponse{
		AccessToken:  accessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    expiresIn,
		RefreshToken: newPlain,
		Scope:        scopeForUser(currentUser),
		User:         currentUser,
	}, nil
}

// CurrentUser returns the active user behind an access token.
// CurrentUser 返回访问令牌对应的 active 用户。
func (s *Service) CurrentUser(ctx context.Context, rawToken string) (user.User, error) {
	claims, err := s.tokens.parseAccessToken(rawToken)
	if err != nil {
		return user.User{}, err
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return user.User{}, apperrors.InvalidToken
	}
	currentUser, err := s.users.GetByID(ctx, id)
	if err != nil {
		return user.User{}, err
	}
	if currentUser.Status != user.StatusActive {
		return user.User{}, apperrors.UserDisabled
	}
	return currentUser, nil
}

func (s *Service) issueTokenResponse(ctx context.Context, currentUser user.User) (TokenResponse, error) {
	accessToken, expiresIn, err := s.tokens.issueAccessToken(currentUser)
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	refreshPlain, refreshHash, err := newRefreshToken()
	if err != nil {
		return TokenResponse{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := s.refreshRepo.storeRefreshToken(ctx, currentUser.ID, refreshHash, time.Now().UTC().Add(s.jwt.RefreshTTL)); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken:  accessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshPlain,
		Scope:        scopeForUser(currentUser),
		User:         currentUser,
	}, nil
}

func scopeForUser(currentUser user.User) string {
	return "role:" + currentUser.Role
}
