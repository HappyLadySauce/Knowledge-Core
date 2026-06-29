package user

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

const (
	bcryptCost        = 12
	minPasswordLength = 8
	defaultPage       = 1
	defaultPageSize   = 20
	maxPageSize       = 100
)

type Service struct {
	repo *Repository
}

func NewService(db *sql.DB) UserService {
	return &Service{repo: NewRepository(db)}
}

func (s *Service) GetMe(ctx context.Context, actor User) (User, error) {
	return s.repo.GetByID(ctx, actor.ID)
}

func (s *Service) UpdateMe(ctx context.Context, actor User, cmd UpdateProfileCommand) (User, error) {
	cmd = normalizeProfileCommand(cmd)
	if cmd.Username == nil && cmd.Email == nil && cmd.Avatar == nil && cmd.Bio == nil {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Username != nil && *cmd.Username == "" {
		return User{}, apperrors.InvalidRequest
	}
	return s.repo.UpdateProfile(ctx, actor.ID, cmd)
}

func (s *Service) ChangePassword(ctx context.Context, actor User, cmd ChangePasswordCommand) error {
	oldPassword := strings.TrimSpace(cmd.OldPassword)
	newPassword := strings.TrimSpace(cmd.NewPassword)
	if oldPassword == "" || len(newPassword) < minPasswordLength {
		return apperrors.InvalidRequest
	}
	record, err := s.repo.GetRecordByID(ctx, actor.ID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(oldPassword)) != nil {
		return apperrors.InvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return s.repo.UpdatePasswordHash(ctx, actor.ID, string(hash))
}

func (s *Service) ListUsers(ctx context.Context, actor User, query ListQuery) (ListResult, error) {
	if actor.Role != RoleAdmin {
		return ListResult{}, apperrors.Forbidden
	}
	query = normalizeListQuery(query)
	if query.Role != "" && query.Role != RoleAdmin && query.Role != RoleUser {
		return ListResult{}, apperrors.InvalidRequest
	}
	if query.Status != "" && query.Status != StatusActive && query.Status != StatusDisabled {
		return ListResult{}, apperrors.InvalidRequest
	}
	return s.repo.List(ctx, query)
}

func (s *Service) GetUser(ctx context.Context, actor User, id int64) (User, error) {
	if actor.Role != RoleAdmin {
		return User{}, apperrors.Forbidden
	}
	if id <= 0 {
		return User{}, apperrors.InvalidRequest
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, actor User, id int64, cmd AdminUpdateCommand) (User, error) {
	if actor.Role != RoleAdmin {
		return User{}, apperrors.Forbidden
	}
	if id <= 0 {
		return User{}, apperrors.InvalidRequest
	}
	cmd = normalizeAdminCommand(cmd)
	if cmd.Username == nil && cmd.Email == nil && cmd.Avatar == nil && cmd.Bio == nil && cmd.Status == nil && cmd.Role == nil {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Username != nil && *cmd.Username == "" {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Status != nil && *cmd.Status != StatusActive && *cmd.Status != StatusDisabled {
		return User{}, apperrors.InvalidRequest
	}
	if cmd.Role != nil && *cmd.Role != RoleAdmin && *cmd.Role != RoleUser {
		return User{}, apperrors.InvalidRequest
	}
	if actor.ID == id && (cmd.Status != nil || cmd.Role != nil) {
		return User{}, fmt.Errorf("%w: admin cannot change own role or status", apperrors.Forbidden)
	}
	return s.repo.AdminUpdate(ctx, id, cmd)
}

func (s *Service) DeleteUser(ctx context.Context, actor User, id int64) error {
	if actor.Role != RoleAdmin {
		return apperrors.Forbidden
	}
	if id <= 0 {
		return apperrors.InvalidRequest
	}
	if actor.ID == id {
		return fmt.Errorf("%w: admin cannot delete self", apperrors.Forbidden)
	}
	return s.repo.Disable(ctx, id)
}

func (s *Service) ResetPassword(ctx context.Context, actor User, id int64, password string) error {
	if actor.Role != RoleAdmin {
		return apperrors.Forbidden
	}
	if id <= 0 {
		return apperrors.InvalidRequest
	}
	if actor.ID == id {
		return fmt.Errorf("%w: admin cannot reset own password", apperrors.Forbidden)
	}
	password = strings.TrimSpace(password)
	if len(password) < minPasswordLength {
		return apperrors.InvalidRequest
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return s.repo.UpdatePasswordHash(ctx, id, string(hash))
}

func normalizeProfileCommand(cmd UpdateProfileCommand) UpdateProfileCommand {
	if cmd.Username != nil {
		username := NormalizeUsername(*cmd.Username)
		cmd.Username = &username
	}
	if cmd.Email != nil {
		email := strings.TrimSpace(*cmd.Email)
		cmd.Email = &email
	}
	if cmd.Avatar != nil {
		avatar := strings.TrimSpace(*cmd.Avatar)
		cmd.Avatar = &avatar
	}
	if cmd.Bio != nil {
		bio := strings.TrimSpace(*cmd.Bio)
		cmd.Bio = &bio
	}
	return cmd
}

func normalizeAdminCommand(cmd AdminUpdateCommand) AdminUpdateCommand {
	profile := normalizeProfileCommand(UpdateProfileCommand{
		Username: cmd.Username,
		Email:    cmd.Email,
		Avatar:   cmd.Avatar,
		Bio:      cmd.Bio,
	})
	cmd.Username = profile.Username
	cmd.Email = profile.Email
	cmd.Avatar = profile.Avatar
	cmd.Bio = profile.Bio
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

func normalizeListQuery(query ListQuery) ListQuery {
	if query.Page <= 0 {
		query.Page = defaultPage
	}
	if query.PageSize <= 0 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	query.Role = strings.TrimSpace(query.Role)
	query.Status = strings.TrimSpace(query.Status)
	query.Keyword = strings.TrimSpace(query.Keyword)
	return query
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
