package auth

import (
	"context"
	"database/sql"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

// Repository persists login-attempt state in PostgreSQL.
// Repository 将登录尝试状态持久化到 PostgreSQL。
type Repository struct {
	db *sql.DB
}

// NewRepository creates an auth repository backed by db.
// NewRepository 创建基于 db 的认证仓储。
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
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
