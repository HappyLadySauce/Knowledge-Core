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

// Repository persists auth refresh tokens in PostgreSQL.
// Repository 将认证刷新令牌持久化到 PostgreSQL。
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
VALUES ($1, $2, $3, $4)`,
		userID, tokenHash, expiresAt.UTC(), time.Now().UTC())
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
		tokenID      int64
		userID       int64
		expiresAtOld time.Time
		revokedAt    sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `
SELECT id, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1`, oldHash).Scan(&tokenID, &userID, &expiresAtOld, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, apperrors.InvalidToken
		}
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if revokedAt.Valid {
		return user.User{}, apperrors.InvalidToken
	}
	if !expiresAtOld.After(time.Now().UTC()) {
		return user.User{}, apperrors.InvalidToken
	}

	currentUser, err := scanUser(tx.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), avatar, bio, role, status, token_version, created_at, updated_at
FROM users
WHERE id = $1`, userID))
	if err != nil {
		return user.User{}, err
	}
	if currentUser.Status != user.StatusActive {
		return user.User{}, apperrors.UserDisabled
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = $1
WHERE id = $2 AND revoked_at IS NULL`,
		now, tokenID)
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
VALUES ($1, $2, $3, $4)`,
		userID, newHash, expiresAt.UTC(), now); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return currentUser, nil
}

func (r *Repository) revokeRefreshToken(ctx context.Context, userID int64, tokenHash string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = $1
WHERE token_hash = $2 AND user_id = $3 AND revoked_at IS NULL`,
		time.Now().UTC(), tokenHash, userID)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected != 1 {
		return apperrors.InvalidToken
	}
	return nil
}

// LoginAttempt captures the current failure state for a user.
// LoginAttempt 记录用户当前的登录失败状态。
type LoginAttempt struct {
	FailedCount int
	LockedUntil sql.NullTime
}

// GetLoginAttempt returns the current login attempt state for a user.
// A zero-value LoginAttempt is returned when no record exists yet.
// GetLoginAttempt 返回用户当前的登录尝试状态。
// 不存在记录时返回零值 LoginAttempt。
func (r *Repository) GetLoginAttempt(ctx context.Context, userID int64) (LoginAttempt, error) {
	var (
		failedCount int
		lockedUntil sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
SELECT failed_count, locked_until FROM login_attempts WHERE user_id = $1`, userID).
		Scan(&failedCount, &lockedUntil)
	if err == sql.ErrNoRows {
		return LoginAttempt{}, nil
	}
	if err != nil {
		return LoginAttempt{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return LoginAttempt{FailedCount: failedCount, LockedUntil: lockedUntil}, nil
}

// RecordFailedLogin increments the failure counter and locks the account when
// the count reaches maxAttempts. Returns the updated attempt state.
// RecordFailedLogin 递增失败计数器，当计数达到 maxAttempts 时锁定账户。
// 返回更新后的尝试状态。
func (r *Repository) RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) (LoginAttempt, error) {
	now := time.Now().UTC()
	lockedUntil := sql.NullTime{}
	if maxAttempts > 0 {
		var failedCount int
		err := r.db.QueryRowContext(ctx, `
INSERT INTO login_attempts (user_id, failed_count, last_failed_at, locked_until)
VALUES ($1, 1, $2, NULL)
ON CONFLICT(user_id) DO UPDATE SET
    failed_count = login_attempts.failed_count + 1,
    last_failed_at = excluded.last_failed_at
RETURNING login_attempts.failed_count`, userID, now).Scan(&failedCount)
		if err != nil {
			return LoginAttempt{}, apperrors.Wrap(apperrors.InternalError, err)
		}
		if failedCount >= maxAttempts {
			lockTime := now.Add(lockDuration)
			if _, err := r.db.ExecContext(ctx, `
UPDATE login_attempts SET locked_until = $1 WHERE user_id = $2`, lockTime, userID); err != nil {
				return LoginAttempt{}, apperrors.Wrap(apperrors.InternalError, err)
			}
			lockedUntil = sql.NullTime{Time: lockTime, Valid: true}
			return LoginAttempt{FailedCount: failedCount, LockedUntil: lockedUntil}, nil
		}
		return LoginAttempt{FailedCount: failedCount, LockedUntil: lockedUntil}, nil
	}
	// maxAttempts == 0 means locking is disabled; just record the failure.
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO login_attempts (user_id, failed_count, last_failed_at, locked_until)
VALUES ($1, 1, $2, NULL)
ON CONFLICT(user_id) DO UPDATE SET
    failed_count = login_attempts.failed_count + 1,
    last_failed_at = excluded.last_failed_at`, userID, now); err != nil {
		return LoginAttempt{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return LoginAttempt{FailedCount: 1, LockedUntil: lockedUntil}, nil
}

// ResetLoginAttempt clears the failure state after a successful login.
// ResetLoginAttempt 在成功登录后清除失败状态。
func (r *Repository) ResetLoginAttempt(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM login_attempts WHERE user_id = $1`, userID); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (user.User, error) {
	var u user.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Avatar, &u.Bio, &u.Role, &u.Status, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, apperrors.NotFound
		}
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return u, nil
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		// Rollback failures after a failed transaction are not actionable here.
		// 事务失败后的回滚错误在此处不可恢复。
		return
	}
}
