package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	"github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

type Claims struct {
	Role         string `json:"role"`
	TokenVersion int64  `json:"ver"`
	jwt.RegisteredClaims
}

func (s *Service) issueAccessToken(currentUser user.User) (string, int64, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.jwt.AccessTTL)
	claims := Claims{
		Role:         currentUser.Role,
		TokenVersion: currentUser.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwt.Issuer,
			Subject:   fmt.Sprintf("%d", currentUser.ID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        randomID(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwt.Secret))
	if err != nil {
		return "", 0, err
	}
	return signed, int64(s.jwt.AccessTTL.Seconds()), nil
}

func (s *Service) parseAccessToken(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, apperrors.InvalidToken
		}
		return []byte(s.jwt.Secret), nil
	}, jwt.WithIssuer(s.jwt.Issuer))
	if err != nil {
		return nil, apperrors.InvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, apperrors.InvalidToken
	}
	return claims, nil
}

func newRefreshToken() (plain string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, refreshTokenHash(plain), nil
}

func refreshTokenHash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
