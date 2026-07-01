package auth

import (
	"context"
	"strings"

	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

const TokenTypeBearer = "Bearer"

const defaultKeyPrefix = "knowledge-core"

// TokenResponse carries issued OAuth2-compatible token fields.
// TokenResponse 携带已签发的 OAuth2 兼容令牌字段。
type TokenResponse struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	RefreshToken string
	Scope        string
	User         user.User
}

type RegisterCommand struct {
	Username string
	Password string
	Email    string
}

type LoginCommand struct {
	Username string
	Password string
}

type RefreshCommand struct {
	RefreshToken string
}

type LogoutCommand struct {
	RefreshToken string
	UserID       int64
}

type AuthService interface {
	Register(ctx context.Context, req RegisterCommand) (TokenResponse, error)
	Login(ctx context.Context, req LoginCommand) (TokenResponse, error)
	Refresh(ctx context.Context, req RefreshCommand) (TokenResponse, error)
	Logout(ctx context.Context, req LogoutCommand) error
	CurrentUser(ctx context.Context, rawToken string) (user.User, error)
}

// ServiceOptions controls auth service internals.
// ServiceOptions 控制认证服务内部选项。
type ServiceOptions struct {
	KeyPrefix string
}

func normalizeServiceOptions(opts ...ServiceOptions) ServiceOptions {
	options := ServiceOptions{KeyPrefix: defaultKeyPrefix}
	if len(opts) > 0 {
		options = opts[0]
	}
	options.KeyPrefix = strings.Trim(strings.TrimSpace(options.KeyPrefix), ":")
	if options.KeyPrefix == "" {
		options.KeyPrefix = defaultKeyPrefix
	}
	return options
}
