package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

// Repository persists auth users and refresh tokens in SQLite.
// Repository 将认证用户与刷新令牌持久化到 SQLite。
type Repository struct {
	db *sql.DB
}

// NewRepository creates an auth repository backed by db.
// NewRepository 创建基于 db 的认证仓储。
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) createUser(ctx context.Context, username, email, passwordHash string) (User, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
INSERT INTO users (username, email, password_hash, role, status, created_at, updated_at)
VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		username, email, passwordHash, RoleUser, StatusActive, formatTime(now), formatTime(now))
	if err != nil {
		if isSQLiteConstraint(err) {
			return User{}, apperrors.Conflict
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.getUserByID(ctx, id)
}

func (r *Repository) getUserRecordByUsername(ctx context.Context, username string) (userRecord, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), password_hash, role, status, created_at, updated_at
FROM users
WHERE username = ?`, username)
	return scanUserRecord(row)
}

func (r *Repository) getUserByID(ctx context.Context, id int64) (User, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), role, status, created_at, updated_at
FROM users
WHERE id = ?`, id)
	return scanUser(row)
}

func (r *Repository) updateUser(ctx context.Context, id int64, cmd UpdateUserCommand) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	current, err := scanUser(tx.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), role, status, created_at, updated_at
FROM users
WHERE id = ?`, id))
	if err != nil {
		return User{}, err
	}

	nextUsername := current.Username
	nextEmail := current.Email
	nextStatus := current.Status
	nextRole := current.Role
	if cmd.Username != nil {
		nextUsername = *cmd.Username
	}
	if cmd.Email != nil {
		nextEmail = *cmd.Email
	}
	if cmd.Status != nil {
		nextStatus = *cmd.Status
	}
	if cmd.Role != nil {
		nextRole = *cmd.Role
	}
	if current.Role == RoleAdmin && current.Status == StatusActive {
		wouldLoseActiveAdmin := nextStatus == StatusDisabled || nextRole != RoleAdmin
		if wouldLoseActiveAdmin {
			count, err := countActiveAdminsTx(ctx, tx)
			if err != nil {
				return User{}, err
			}
			if count <= 1 {
				return User{}, apperrors.Forbidden
			}
		}
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE users
SET username = ?, email = NULLIF(?, ''), status = ?, role = ?, updated_at = ?
WHERE id = ?`,
		nextUsername, nextEmail, nextStatus, nextRole, formatTime(now), id)
	if err != nil {
		if isSQLiteConstraint(err) {
			return User{}, apperrors.Conflict
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return User{}, apperrors.NotFound
	}
	shouldRevokeTokens := nextStatus == StatusDisabled || nextRole != current.Role
	if shouldRevokeTokens {
		if _, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, ?)
WHERE user_id = ? AND revoked_at IS NULL`,
			formatTime(now), id); err != nil {
			return User{}, apperrors.Wrap(apperrors.InternalError, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.getUserByID(ctx, id)
}

func (r *Repository) countActiveAdmins(ctx context.Context) (int64, error) {
	return countActiveAdmins(ctx, r.db)
}

func countActiveAdminsTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	return countActiveAdmins(ctx, tx)
}

func countActiveAdmins(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (int64, error) {
	var count int64
	err := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM users
WHERE role = ? AND status = ?`, RoleAdmin, StatusActive).Scan(&count)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.InternalError, err)
	}
	return count, nil
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

func (r *Repository) rotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
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
			return User{}, apperrors.InvalidToken
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if revokedText.Valid {
		return User{}, apperrors.InvalidToken
	}
	expiresAtOld, err := parseTime(expiresText)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if !expiresAtOld.After(time.Now().UTC()) {
		return User{}, apperrors.InvalidToken
	}

	user, err := scanUser(tx.QueryRowContext(ctx, `
SELECT id, username, COALESCE(email, ''), role, status, created_at, updated_at
FROM users
WHERE id = ?`, userID))
	if err != nil {
		return User{}, err
	}
	if user.Status != StatusActive {
		return User{}, apperrors.UserDisabled
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = ?
WHERE id = ? AND revoked_at IS NULL`,
		formatTime(now), tokenID)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected != 1 {
		return User{}, apperrors.InvalidToken
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
VALUES (?, ?, ?, ?)`,
		userID, newHash, formatTime(expiresAt.UTC()), formatTime(now)); err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return user, nil
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (User, error) {
	var (
		u                        User
		createdText, updatedText string
	)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Status, &createdText, &updatedText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, apperrors.NotFound
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	createdAt, err := parseTime(createdText)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return u, nil
}

func scanUserRecord(row interface {
	Scan(dest ...any) error
}) (userRecord, error) {
	var (
		record                   userRecord
		createdText, updatedText string
	)
	err := row.Scan(&record.ID, &record.Username, &record.Email, &record.PasswordHash, &record.Role, &record.Status, &createdText, &updatedText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userRecord{}, apperrors.InvalidCredentials
		}
		return userRecord{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	createdAt, err := parseTime(createdText)
	if err != nil {
		return userRecord{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return userRecord{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	return record, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func isSQLiteConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint")
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		// Rollback failures after a failed transaction are not actionable here.
		// 事务失败后的回滚错误在此处不可恢复。
		return
	}
}
