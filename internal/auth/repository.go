package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

// Repository persists auth refresh tokens in SQLite.
// Repository 将认证刷新令牌持久化到 SQLite。
type Repository struct {
	db *sql.DB
}

// NewRepository creates an auth repository backed by db.
// NewRepository 创建基于 db 的认证仓储。
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) storeRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
VALUES (?, ?, ?, ?)`,
		userID, tokenHash, formatTime(expiresAt.UTC()), formatTime(time.Now().UTC()))
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, fmt.Errorf("store refresh token: %w", err))
	}
	return nil
}

func (r *Repository) rotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (user.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	var (
		tokenID     int64
		userID      int64
		expiresText string
		revokedText sql.NullString
	)
	err = tx.QueryRowContext(ctx, `
SELECT id, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = ?`, oldHash).Scan(&tokenID, &userID, &expiresText, &revokedText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, apperrors.InvalidToken
		}
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if revokedText.Valid {
		return user.User{}, apperrors.InvalidToken
	}
	expiresAtOld, err := parseTime(expiresText)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if !expiresAtOld.After(time.Now().UTC()) {
		return user.User{}, apperrors.InvalidToken
	}

	currentUser, err := scanUser(tx.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), avatar, bio, role, status, created_at, updated_at
FROM users
WHERE id = ?`, userID))
	if err != nil {
		return user.User{}, err
	}
	if currentUser.Status != user.StatusActive {
		return user.User{}, apperrors.UserDisabled
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = ?
WHERE id = ? AND revoked_at IS NULL`,
		formatTime(now), tokenID)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected != 1 {
		return user.User{}, apperrors.InvalidToken
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
VALUES (?, ?, ?, ?)`,
		userID, newHash, formatTime(expiresAt.UTC()), formatTime(now)); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return currentUser, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (user.User, error) {
	var (
		u                        user.User
		createdText, updatedText string
	)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Avatar, &u.Bio, &u.Role, &u.Status, &createdText, &updatedText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, apperrors.NotFound
		}
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	createdAt, err := parseTime(createdText)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return u, nil
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		// Rollback failures after a failed transaction are not actionable here.
		// 事务失败后的回滚错误在此处不可恢复。
		return
	}
}
