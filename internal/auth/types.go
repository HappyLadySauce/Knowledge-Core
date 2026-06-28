package auth

import "time"

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusActive   = "active"
	StatusDisabled = "disabled"

	TokenTypeBearer = "Bearer"
)

// User is the domain account model.
// User 是领域层账户模型。
type User struct {
	ID        int64
	Username  string
	Email     string
	Role      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type userRecord struct {
	User
	PasswordHash string
}

// TokenResponse carries issued OAuth2-compatible token fields.
// TokenResponse 携带已签发的 OAuth2 兼容令牌字段。
type TokenResponse struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	RefreshToken string
	Scope        string
	User         User
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

type UpdateUserCommand struct {
	Username *string
	Email    *string
	Status   *string
	Role     *string
}
