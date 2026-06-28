package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// HS256 secret bound: at least one hash output (256 bits / 32 bytes) per RFC 7518 §3.2.
// HS256 密钥下限：至少一个哈希输出长度（256 位 / 32 字节），见 RFC 7518 §3.2。
const jwtMinSecretLen = 32

// JWTOptions holds CLI / config knobs for HMAC-SHA256 JWT issuance and verification.
// JWTOptions 保存基于 HMAC-SHA256 的 JWT 签发与验证相关的 CLI 与配置项。
type JWTOptions struct {
	// Issuer is the JWT iss claim and the verifier-side expected issuer.
	// Issuer 为 JWT 的 iss 声明，同时是验证侧期望的发行者。
	Issuer string `json:"issuer" mapstructure:"issuer"`
	// Secret is the HS256 signing key. JSON-tagged "-" to keep it out of debug dumps.
	// Secret 为 HS256 签名密钥；JSON tag "-" 避免出现在调试打印中。
	Secret string `json:"-" mapstructure:"secret"`
	// AccessTTL is the access token lifetime; short by design.
	// AccessTTL 为访问令牌存活时间，按设计应较短。
	AccessTTL time.Duration `json:"access-ttl" mapstructure:"access-ttl"`
	// RefreshTTL is the refresh token lifetime; must be strictly greater than AccessTTL.
	// RefreshTTL 为刷新令牌存活时间，必须严格大于 AccessTTL。
	RefreshTTL time.Duration `json:"refresh-ttl" mapstructure:"refresh-ttl"`
}

// NewJWTOptions returns an empty JWTOptions; defaults are applied via AddFlags.
// NewJWTOptions 返回空的 JWTOptions；默认值通过 AddFlags 写入。
func NewJWTOptions() *JWTOptions {
	return &JWTOptions{}
}

// Validate enforces a non-empty issuer, a sufficiently long secret, and ordered TTLs.
// Validate 强制 issuer 非空、密钥长度足够，以及 TTL 之间的顺序关系。
func (j *JWTOptions) Validate() error {
	var err error
	if j.Issuer == "" {
		err = errors.Join(err, fmt.Errorf("issuer is required"))
	}
	if j.Secret == "" {
		err = errors.Join(err, fmt.Errorf("secret is required"))
	} else if len(j.Secret) < jwtMinSecretLen {
		err = errors.Join(err, fmt.Errorf("secret must be at least %d bytes for HS256, got %d", jwtMinSecretLen, len(j.Secret)))
	}
	if j.AccessTTL <= 0 {
		err = errors.Join(err, fmt.Errorf("access-ttl must be > 0, got %s", j.AccessTTL))
	}
	if j.RefreshTTL <= 0 {
		err = errors.Join(err, fmt.Errorf("refresh-ttl must be > 0, got %s", j.RefreshTTL))
	}
	if j.AccessTTL > 0 && j.RefreshTTL > 0 && j.RefreshTTL <= j.AccessTTL {
		err = errors.Join(err, fmt.Errorf("refresh-ttl must be > access-ttl, got refresh=%s access=%s", j.RefreshTTL, j.AccessTTL))
	}
	return err
}

// AddFlags registers JWT flags on the supplied FlagSet.
// AddFlags 将 JWT 相关命令行标志注册到给定的 FlagSet。
func (j *JWTOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&j.Issuer, "jwt-issuer", "beehive-blog", "JWT iss claim and verifier expected issuer")
	fs.StringVar(&j.Secret, "jwt-secret", "", "HMAC-SHA256 signing secret (>= 32 bytes, required)")
	fs.DurationVar(&j.AccessTTL, "jwt-access-ttl", 15*time.Minute, "Access token lifetime")
	fs.DurationVar(&j.RefreshTTL, "jwt-refresh-ttl", 7*24*time.Hour, "Refresh token lifetime (must be > jwt-access-ttl)")
}
