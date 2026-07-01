package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

const (
	reasonRotated = "rotated"
)

type redisRefreshToken struct {
	UserID       int64     `json:"user_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// storeRefreshToken records a new refresh token in PostgreSQL and best-effort
// writes the active session metadata to Redis.
// storeRefreshToken 将新 refresh token 写入 PostgreSQL，并尽力写入 Redis 活跃会话元数据。
func (s *Service) storeRefreshToken(ctx context.Context, currentUser user.User, tokenHash string, expiresAt time.Time) error {
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (user_id, token_hash, token_version, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5)`,
		currentUser.ID, tokenHash, currentUser.TokenVersion, expiresAt.UTC(), now); err != nil {
		return apperrors.Wrap(apperrors.InternalError, fmt.Errorf("store refresh token audit: %w", err))
	}
	s.storeRedisBestEffort(ctx, currentUser, tokenHash, expiresAt.UTC(), now)
	return nil
}

// rotateRefreshToken atomically revokes oldHash, inserts newHash, and returns
// the active user snapshot used for issuing the next access token.
// rotateRefreshToken 原子撤销 oldHash、插入 newHash，并返回用于签发新访问令牌的活跃用户快照。
func (s *Service) rotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (user.User, error) {
	oldHash = strings.TrimSpace(oldHash)
	newHash = strings.TrimSpace(newHash)
	if oldHash == "" || newHash == "" {
		return user.User{}, apperrors.InvalidToken
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rollbackTx(tx)

	var (
		tokenID      int64
		tokenUserID  int64
		tokenVersion int64
		expiresAtOld time.Time
		revokedAt    sql.NullTime
		currentUser  user.User
	)
	err = tx.QueryRowContext(ctx, `
SELECT rt.id, rt.user_id, rt.token_version, rt.expires_at, rt.revoked_at,
       u.id, u.username, COALESCE(u.email, ''), u.avatar, u.bio, u.role, u.status, u.token_version, u.created_at, u.updated_at
FROM refresh_tokens rt
JOIN users u ON u.id = rt.user_id
WHERE rt.token_hash = $1
FOR UPDATE OF rt`, oldHash).Scan(
		&tokenID, &tokenUserID, &tokenVersion, &expiresAtOld, &revokedAt,
		&currentUser.ID, &currentUser.Username, &currentUser.Email, &currentUser.Avatar, &currentUser.Bio,
		&currentUser.Role, &currentUser.Status, &currentUser.TokenVersion, &currentUser.CreatedAt, &currentUser.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, apperrors.InvalidToken
		}
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if tokenUserID != currentUser.ID || revokedAt.Valid || !expiresAtOld.After(time.Now().UTC()) {
		return user.User{}, apperrors.InvalidToken
	}
	if currentUser.Status != user.StatusActive || tokenVersion != currentUser.TokenVersion {
		return user.User{}, apperrors.InvalidToken
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = $1, last_used_at = $1, rotated_to_hash = $2, revoked_reason = $3
WHERE id = $4 AND revoked_at IS NULL`,
		now, newHash, reasonRotated, tokenID)
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
INSERT INTO refresh_tokens (user_id, token_hash, token_version, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5)`,
		currentUser.ID, newHash, currentUser.TokenVersion, expiresAt.UTC(), now); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}
	if err := tx.Commit(); err != nil {
		return user.User{}, apperrors.Wrap(apperrors.InternalError, err)
	}

	s.deleteRedisBestEffort(ctx, currentUser.ID, oldHash)
	s.storeRedisBestEffort(ctx, currentUser, newHash, expiresAt.UTC(), now)
	return currentUser, nil
}

// revokeRefreshToken revokes one refresh token for a user.
// revokeRefreshToken 撤销用户的单个 refresh token。
func (s *Service) revokeRefreshToken(ctx context.Context, userID int64, tokenHash, reason string) error {
	tokenHash = strings.TrimSpace(tokenHash)
	if userID <= 0 || tokenHash == "" {
		return apperrors.InvalidToken
	}
	reason = normalizeReason(reason)
	var revokedHash string
	err := s.db.QueryRowContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = $1, revoked_reason = $2
WHERE user_id = $3 AND token_hash = $4 AND revoked_at IS NULL
RETURNING token_hash`,
		time.Now().UTC(), reason, userID, tokenHash).Scan(&revokedHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.InvalidToken
		}
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	s.deleteRedisBestEffort(ctx, userID, revokedHash)
	return nil
}

// RevokeUserRefreshTokens revokes all active refresh tokens for one user.
// RevokeUserRefreshTokens 撤销单个用户的所有活跃 refresh token。
func (s *Service) RevokeUserRefreshTokens(ctx context.Context, userID int64, reason string) error {
	if userID <= 0 {
		return apperrors.InvalidRequest
	}
	reason = normalizeReason(reason)
	rows, err := s.db.QueryContext(ctx, `
UPDATE refresh_tokens
SET revoked_at = $1, revoked_reason = $2
WHERE user_id = $3 AND revoked_at IS NULL
RETURNING token_hash`,
		time.Now().UTC(), reason, userID)
	if err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	defer rows.Close()

	hashes := make([]string, 0)
	for rows.Next() {
		var tokenHash string
		if err := rows.Scan(&tokenHash); err != nil {
			return apperrors.Wrap(apperrors.InternalError, err)
		}
		hashes = append(hashes, tokenHash)
	}
	if err := rows.Err(); err != nil {
		return apperrors.Wrap(apperrors.InternalError, err)
	}
	s.deleteUserRedisBestEffort(ctx, userID, hashes)
	return nil
}

func (s *Service) storeRedisBestEffort(ctx context.Context, currentUser user.User, tokenHash string, expiresAt, createdAt time.Time) {
	if s.redis == nil {
		return
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return
	}
	payload, err := json.Marshal(redisRefreshToken{
		UserID:       currentUser.ID,
		TokenVersion: currentUser.TokenVersion,
		ExpiresAt:    expiresAt,
		CreatedAt:    createdAt,
	})
	if err != nil {
		klog.ErrorS(err, "failed to encode redis refresh token payload")
		return
	}
	pipe := s.redis.Pipeline()
	pipe.Set(ctx, s.tokenKey(tokenHash), payload, ttl)
	pipe.SAdd(ctx, s.userKey(currentUser.ID), tokenHash)
	pipe.Expire(ctx, s.userKey(currentUser.ID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		klog.ErrorS(err, "failed to write refresh token session to redis")
	}
}

func (s *Service) deleteRedisBestEffort(ctx context.Context, userID int64, tokenHash string) {
	if s.redis == nil {
		return
	}
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, s.tokenKey(tokenHash))
	pipe.SRem(ctx, s.userKey(userID), tokenHash)
	if _, err := pipe.Exec(ctx); err != nil {
		klog.ErrorS(err, "failed to delete refresh token session from redis")
	}
}

func (s *Service) deleteUserRedisBestEffort(ctx context.Context, userID int64, tokenHashes []string) {
	if s.redis == nil {
		return
	}
	pipe := s.redis.Pipeline()
	for _, tokenHash := range tokenHashes {
		pipe.Del(ctx, s.tokenKey(tokenHash))
	}
	pipe.Del(ctx, s.userKey(userID))
	if _, err := pipe.Exec(ctx); err != nil {
		klog.ErrorS(err, "failed to delete user refresh token sessions from redis")
	}
}

func (s *Service) tokenKey(tokenHash string) string {
	return s.keyPrefix + ":auth:refresh:token:" + tokenHash
}

func (s *Service) userKey(userID int64) string {
	return fmt.Sprintf("%s:auth:refresh:user:%d", s.keyPrefix, userID)
}

func normalizeReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "unspecified"
	}
	return reason
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}
