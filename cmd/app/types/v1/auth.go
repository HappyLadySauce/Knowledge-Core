package v1

import "time"

// UserResponse is the public user payload returned by HTTP APIs.
// UserResponse 是 HTTP API 返回的公开用户载荷。
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TokenResponse is the OAuth2-compatible token payload inside the common envelope.
// TokenResponse 是通用响应包装内的 OAuth2 兼容令牌载荷。
type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"`
	RefreshToken string       `json:"refresh_token"`
	Scope        string       `json:"scope"`
	User         UserResponse `json:"user"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UpdateUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Status   *string `json:"status"`
	Role     *string `json:"role"`
}
