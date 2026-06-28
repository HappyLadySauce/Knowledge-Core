package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultJWTIssuer = "Knowledge-Core"
	jwtMinSecretLen  = 32
)

// JWTOptions holds JWT signing and token lifetime settings.
// JWTOptions 保存 JWT 签名与令牌生命周期配置。
type JWTOptions struct {
	// Issuer is written to and verified from JWT iss.
	// Issuer 会写入并校验 JWT 的 iss。
	Issuer string `json:"issuer" mapstructure:"issuer"`
	// Secret is the HS256 signing secret.
	// Secret 是 HS256 签名密钥。
	Secret string `json:"-" mapstructure:"secret"`
	// AccessTTL is the access token lifetime.
	// AccessTTL 是访问令牌有效期。
	AccessTTL time.Duration `json:"access-ttl" mapstructure:"access-ttl"`
	// RefreshTTL is the refresh token lifetime.
	// RefreshTTL 是刷新令牌有效期。
	RefreshTTL time.Duration `json:"refresh-ttl" mapstructure:"refresh-ttl"`
}

// NewJWTOptions returns JWT defaults with a hardcoded issuer.
// NewJWTOptions 返回使用硬编码 issuer 的 JWT 默认配置。
func NewJWTOptions() *JWTOptions {
	return &JWTOptions{Issuer: defaultJWTIssuer}
}

// Validate checks JWT issuer, secret strength, and token lifetimes.
// Validate 校验 JWT issuer、密钥强度与令牌有效期。
func (j *JWTOptions) Validate() error {
	var err error
	if j.Issuer == "" {
		j.Issuer = defaultJWTIssuer
	}
	if len(j.Secret) < jwtMinSecretLen {
		err = errors.Join(err, fmt.Errorf("jwt secret must be at least %d bytes, got %d", jwtMinSecretLen, len(j.Secret)))
	}
	if j.AccessTTL <= 0 {
		err = errors.Join(err, fmt.Errorf("jwt access-ttl must be > 0, got %s", j.AccessTTL))
	}
	if j.RefreshTTL <= 0 {
		err = errors.Join(err, fmt.Errorf("jwt refresh-ttl must be > 0, got %s", j.RefreshTTL))
	}
	if j.AccessTTL > 0 && j.RefreshTTL > 0 && j.RefreshTTL <= j.AccessTTL {
		err = errors.Join(err, fmt.Errorf("jwt refresh-ttl must be greater than access-ttl"))
	}
	return err
}

// AddFlags registers JWT flags on the supplied FlagSet.
// AddFlags 将 JWT 相关命令行标志注册到给定的 FlagSet。
func (j *JWTOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&j.Secret, "jwt-secret", "", "HS256 JWT signing secret")
	fs.DurationVar(&j.AccessTTL, "jwt-access-ttl", 15*time.Minute, "JWT access token lifetime")
	fs.DurationVar(&j.RefreshTTL, "jwt-refresh-ttl", 7*24*time.Hour, "JWT refresh token lifetime")
}
