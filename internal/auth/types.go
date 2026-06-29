package auth

import (
	"context"

	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

const TokenTypeBearer = "Bearer"

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

type AuthService interface {
	Register(ctx context.Context, req RegisterCommand) (TokenResponse, error)
	Login(ctx context.Context, req LoginCommand) (TokenResponse, error)
	Refresh(ctx context.Context, req RefreshCommand) (TokenResponse, error)
	CurrentUser(ctx context.Context, rawToken string) (user.User, error)
}
