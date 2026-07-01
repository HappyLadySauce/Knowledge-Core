package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/options"
	"github.com/HappyLadySauce/Knowledge-Core/internal/testutil"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

func TestRefreshTokenIssueWritesAuditAndRedis(t *testing.T) {
	ctx := context.Background()
	db, service := newAuthTestService(t)
	currentUser := insertAuthTestUser(t, db, "session-user")

	if err := service.storeRefreshToken(ctx, currentUser, "hash-1", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("store refresh token failed: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, currentUser.ID).Scan(&count); err != nil {
		t.Fatalf("count refresh token audit failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("active refresh token audit count = %d, want 1", count)
	}
	if err := service.redis.Get(ctx, service.tokenKey("hash-1")).Err(); err != nil {
		t.Fatalf("redis refresh token key missing: %v", err)
	}
}

func TestRefreshTokenRotationIsSingleUseAndRepopulatesRedis(t *testing.T) {
	ctx := context.Background()
	db, service := newAuthTestService(t)
	currentUser := insertAuthTestUser(t, db, "rotate-user")
	if err := service.storeRefreshToken(ctx, currentUser, "old-hash", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("store old refresh token failed: %v", err)
	}
	if err := service.redis.Del(ctx, service.tokenKey("old-hash")).Err(); err != nil {
		t.Fatalf("delete redis token key failed: %v", err)
	}

	rotatedUser, err := service.rotateRefreshToken(ctx, "old-hash", "new-hash", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("rotate refresh token failed: %v", err)
	}
	if rotatedUser.ID != currentUser.ID {
		t.Fatalf("rotated user id = %d, want %d", rotatedUser.ID, currentUser.ID)
	}
	if _, err := service.rotateRefreshToken(ctx, "old-hash", "next-hash", time.Now().UTC().Add(time.Hour)); !errors.Is(err, apperrors.InvalidToken) {
		t.Fatalf("second rotate error = %v, want invalid token", err)
	}
	if err := service.redis.Get(ctx, service.tokenKey("new-hash")).Err(); err != nil {
		t.Fatalf("redis new refresh token key missing after fallback rotate: %v", err)
	}

	var revokedReason, rotatedTo string
	if err := db.QueryRowContext(ctx, `
SELECT revoked_reason, COALESCE(rotated_to_hash, '')
FROM refresh_tokens
WHERE token_hash = 'old-hash'`).Scan(&revokedReason, &rotatedTo); err != nil {
		t.Fatalf("read old refresh token audit failed: %v", err)
	}
	if revokedReason != reasonRotated || rotatedTo != "new-hash" {
		t.Fatalf("old token audit reason=%q rotated_to=%q", revokedReason, rotatedTo)
	}
}

func TestRevokeUserRefreshTokensDeletesRedisSessions(t *testing.T) {
	ctx := context.Background()
	db, service := newAuthTestService(t)
	currentUser := insertAuthTestUser(t, db, "revoke-user")
	for _, tokenHash := range []string{"hash-a", "hash-b"} {
		if err := service.storeRefreshToken(ctx, currentUser, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
			t.Fatalf("store refresh token %s failed: %v", tokenHash, err)
		}
	}

	if err := service.RevokeUserRefreshTokens(ctx, currentUser.ID, "user_disabled"); err != nil {
		t.Fatalf("revoke user refresh tokens failed: %v", err)
	}
	for _, tokenHash := range []string{"hash-a", "hash-b"} {
		if err := service.redis.Get(ctx, service.tokenKey(tokenHash)).Err(); err == nil {
			t.Fatalf("redis key %s still exists after user revoke", tokenHash)
		}
	}
	var active int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, currentUser.ID).Scan(&active); err != nil {
		t.Fatalf("count active refresh tokens failed: %v", err)
	}
	if active != 0 {
		t.Fatalf("active refresh token count = %d, want 0", active)
	}
}

func newAuthTestService(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db := testutil.NewDB(t)
	redisClient, prefix := testutil.NewCacheClient(t)
	return db, NewService(db, testJWTOptions(), redisClient, ServiceOptions{KeyPrefix: prefix})
}

func testJWTOptions() *options.JWTOptions {
	return &options.JWTOptions{
		Issuer:     "Knowledge-Core",
		Secret:     "Knowledge-Core-test-secret-32bytes",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
}

func insertAuthTestUser(t *testing.T, db *sql.DB, username string) user.User {
	t.Helper()
	now := time.Now().UTC()
	var id int64
	if err := db.QueryRowContext(context.Background(), `
INSERT INTO users (username, email, avatar, bio, password_hash, role, status, token_version, created_at, updated_at)
VALUES ($1, '', '', '', '', 'user', 'active', 0, $2, $3)
RETURNING id`, username, now, now).Scan(&id); err != nil {
		t.Fatalf("insert session test user failed: %v", err)
	}
	return user.User{
		ID:           id,
		Username:     username,
		Role:         user.RoleUser,
		Status:       user.StatusActive,
		TokenVersion: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
