package user

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, username, email, passwordHash string) (User, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
INSERT INTO users (username, email, avatar, bio, password_hash, role, status, created_at, updated_at)
VALUES (?, NULLIF(?, ''), '', '', ?, ?, ?, ?, ?)`,
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
	return r.GetByID(ctx, id)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (User, error) {
	row := r.db.QueryRowContext(ctx, userSelectSQL+` WHERE id = ?`, id)
	return scanUser(row)
}

func (r *Repository) GetRecordByID(ctx context.Context, id int64) (Record, error) {
	row := r.db.QueryRowContext(ctx, recordSelectSQL+` WHERE id = ?`, id)
	return scanRecord(row, apperrors.NotFound)
}

func (r *Repository) GetRecordByUsername(ctx context.Context, username string) (Record, error) {
	row := r.db.QueryRowContext(ctx, recordSelectSQL+` WHERE username = ?`, username)
	return scanRecord(row, apperrors.InvalidCredentials)
}

func (r *Repository) List(ctx context.Context, query ListQuery) (ListResult, error) {
	where, args := listWhere(query)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`+where, args...).Scan(&total); err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}

	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, userSelectSQL+where+`
ORDER BY id ASC
LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return ListResult{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, id int64, cmd UpdateProfileCommand) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	current, err := scanUser(tx.QueryRowContext(ctx, userSelectSQL+` WHERE id = ?`, id))
	if err != nil {
		return User{}, err
	}
	nextUsername, nextEmail, nextAvatar, nextBio := profileValues(current, cmd)
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET username = ?, email = NULLIF(?, ''), avatar = ?, bio = ?, updated_at = ?
WHERE id = ?`,
		nextUsername, nextEmail, nextAvatar, nextBio, formatTime(now), id); err != nil {
		if isSQLiteConstraint(err) {
			return User{}, apperrors.Conflict
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) AdminUpdate(ctx context.Context, id int64, cmd AdminUpdateCommand) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	current, err := scanUser(tx.QueryRowContext(ctx, userSelectSQL+` WHERE id = ?`, id))
	if err != nil {
		return User{}, err
	}
	nextUsername, nextEmail, nextAvatar, nextBio := adminProfileValues(current, cmd)
	nextStatus := current.Status
	nextRole := current.Role
	if cmd.Status != nil {
		nextStatus = *cmd.Status
	}
	if cmd.Role != nil {
		nextRole = *cmd.Role
	}
	if err := protectLastActiveAdmin(ctx, tx, current, nextStatus, nextRole); err != nil {
		return User{}, err
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET username = ?, email = NULLIF(?, ''), avatar = ?, bio = ?, status = ?, role = ?, updated_at = ?
WHERE id = ?`,
		nextUsername, nextEmail, nextAvatar, nextBio, nextStatus, nextRole, formatTime(now), id); err != nil {
		if isSQLiteConstraint(err) {
			return User{}, apperrors.Conflict
		}
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if nextStatus == StatusDisabled || nextRole != current.Role {
		if err := revokeRefreshTokensTx(ctx, tx, id, now); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Disable(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	current, err := scanUser(tx.QueryRowContext(ctx, userSelectSQL+` WHERE id = ?`, id))
	if err != nil {
		return err
	}
	if err := protectLastActiveAdmin(ctx, tx, current, StatusDisabled, current.Role); err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET status = ?, updated_at = ?
WHERE id = ?`, StatusDisabled, formatTime(now), id); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := revokeRefreshTokensTx(ctx, tx, id, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, id int64, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE users
SET password_hash = ?, updated_at = ?
WHERE id = ?`, passwordHash, formatTime(now), id)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	if affected == 0 {
		return apperrors.NotFound
	}
	if err := revokeRefreshTokensTx(ctx, tx, id, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func (r *Repository) RevokeRefreshTokens(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)
	if err := revokeRefreshTokensTx(ctx, tx, id, time.Now().UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

func listWhere(query ListQuery) (string, []any) {
	parts := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.Role != "" {
		parts = append(parts, "role = ?")
		args = append(args, query.Role)
	}
	if query.Status != "" {
		parts = append(parts, "status = ?")
		args = append(args, query.Status)
	}
	if query.Keyword != "" {
		parts = append(parts, "(username LIKE ? OR email LIKE ?)")
		like := "%" + query.Keyword + "%"
		args = append(args, like, like)
	}
	if len(parts) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func profileValues(current User, cmd UpdateProfileCommand) (string, string, string, string) {
	username, email, avatar, bio := current.Username, current.Email, current.Avatar, current.Bio
	if cmd.Username != nil {
		username = *cmd.Username
	}
	if cmd.Email != nil {
		email = *cmd.Email
	}
	if cmd.Avatar != nil {
		avatar = *cmd.Avatar
	}
	if cmd.Bio != nil {
		bio = *cmd.Bio
	}
	return username, email, avatar, bio
}

func adminProfileValues(current User, cmd AdminUpdateCommand) (string, string, string, string) {
	return profileValues(current, UpdateProfileCommand{
		Username: cmd.Username,
		Email:    cmd.Email,
		Avatar:   cmd.Avatar,
		Bio:      cmd.Bio,
	})
}

func protectLastActiveAdmin(ctx context.Context, tx *sql.Tx, current User, nextStatus, nextRole string) error {
	if current.Role != RoleAdmin || current.Status != StatusActive {
		return nil
	}
	if nextStatus == StatusActive && nextRole == RoleAdmin {
		return nil
	}
	count, err := countActiveAdmins(ctx, tx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return apperrors.Forbidden
	}
	return nil
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

func revokeRefreshTokensTx(ctx context.Context, tx *sql.Tx, id int64, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, ?)
WHERE user_id = ? AND revoked_at IS NULL`, formatTime(now), id)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	return nil
}

const userSelectSQL = `
SELECT id, username, COALESCE(email, ''), avatar, bio, role, status, created_at, updated_at
FROM users`

const recordSelectSQL = `
SELECT id, username, COALESCE(email, ''), avatar, bio, password_hash, role, status, created_at, updated_at
FROM users`

func scanUser(row interface {
	Scan(dest ...any) error
}) (User, error) {
	var (
		u                        User
		createdText, updatedText string
	)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Avatar, &u.Bio, &u.Role, &u.Status, &createdText, &updatedText)
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

func scanRecord(row interface {
	Scan(dest ...any) error
}, missing error) (Record, error) {
	var (
		record                   Record
		createdText, updatedText string
	)
	err := row.Scan(&record.ID, &record.Username, &record.Email, &record.Avatar, &record.Bio, &record.PasswordHash, &record.Role, &record.Status, &createdText, &updatedText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, missing
		}
		return Record{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	createdAt, err := parseTime(createdText)
	if err != nil {
		return Record{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	updatedAt, err := parseTime(updatedText)
	if err != nil {
		return Record{}, apperrors.Wrap(apperrors.InternalError, err)
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
