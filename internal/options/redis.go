package options

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultRedisURL          = "redis://localhost:6379/0"
	defaultRedisKeyPrefix    = "knowledge-core"
	defaultRedisPoolSize     = 10
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
)

// RedisOptions holds Redis connectivity and key namespace settings.
// RedisOptions 保存 Redis 连接与键命名空间配置。
type RedisOptions struct {
	// Enabled controls whether Redis is used for hot refresh-token sessions.
	// Enabled 控制是否使用 Redis 存储 refresh token 活跃会话。
	Enabled bool `json:"enabled" mapstructure:"enabled"`
	// Required controls startup behavior when Redis is unreachable.
	// Required 控制 Redis 不可达时的启动行为。
	Required bool `json:"required" mapstructure:"required"`
	// URL is the Redis connection URL.
	// URL 为 Redis 连接字符串。
	URL string `json:"url" mapstructure:"url"`
	// KeyPrefix namespaces Redis keys for this application instance.
	// KeyPrefix 为当前应用实例的 Redis key 添加命名空间。
	KeyPrefix string `json:"key-prefix" mapstructure:"key-prefix"`
	// PoolSize is the maximum Redis connection pool size.
	// PoolSize 为 Redis 连接池最大连接数。
	PoolSize int `json:"pool-size" mapstructure:"pool-size"`
	// DialTimeout is the timeout for creating Redis connections.
	// DialTimeout 为创建 Redis 连接的超时时间。
	DialTimeout time.Duration `json:"dial-timeout" mapstructure:"dial-timeout"`
	// ReadTimeout is the timeout for Redis read operations.
	// ReadTimeout 为 Redis 读操作超时时间。
	ReadTimeout time.Duration `json:"read-timeout" mapstructure:"read-timeout"`
	// WriteTimeout is the timeout for Redis write operations.
	// WriteTimeout 为 Redis 写操作超时时间。
	WriteTimeout time.Duration `json:"write-timeout" mapstructure:"write-timeout"`
}

// NewRedisOptions returns production-oriented Redis defaults.
// NewRedisOptions 返回生产可用的 Redis 默认配置。
func NewRedisOptions() *RedisOptions {
	return &RedisOptions{
		Enabled:      true,
		Required:     false,
		URL:          defaultRedisURL,
		KeyPrefix:    defaultRedisKeyPrefix,
		PoolSize:     defaultRedisPoolSize,
		DialTimeout:  defaultRedisDialTimeout,
		ReadTimeout:  defaultRedisReadTimeout,
		WriteTimeout: defaultRedisWriteTimeout,
	}
}

// Validate checks Redis settings and normalizes optional values.
// Validate 校验 Redis 配置并规范化可选值。
func (r *RedisOptions) Validate() error {
	if r == nil {
		return fmt.Errorf("redis config is nil")
	}
	if !r.Enabled {
		return nil
	}
	var err error
	if strings.TrimSpace(r.URL) == "" {
		err = errors.Join(err, fmt.Errorf("redis url is required when redis is enabled"))
	}
	r.KeyPrefix = strings.Trim(strings.TrimSpace(r.KeyPrefix), ":")
	if r.KeyPrefix == "" {
		r.KeyPrefix = defaultRedisKeyPrefix
	}
	if r.PoolSize <= 0 {
		r.PoolSize = defaultRedisPoolSize
	}
	if r.DialTimeout <= 0 {
		r.DialTimeout = defaultRedisDialTimeout
	}
	if r.ReadTimeout <= 0 {
		r.ReadTimeout = defaultRedisReadTimeout
	}
	if r.WriteTimeout <= 0 {
		r.WriteTimeout = defaultRedisWriteTimeout
	}
	return err
}

// AddFlags registers Redis flags on the supplied FlagSet.
// AddFlags 将 Redis 相关命令行标志注册到给定的 FlagSet。
func (r *RedisOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&r.Enabled, "redis-enabled", true, "Enable Redis-backed refresh token sessions")
	fs.BoolVar(&r.Required, "redis-required", false, "Fail startup when Redis is unreachable")
	fs.StringVar(&r.URL, "redis-url", defaultRedisURL, "Redis connection URL")
	fs.StringVar(&r.KeyPrefix, "redis-key-prefix", defaultRedisKeyPrefix, "Redis key prefix")
	fs.IntVar(&r.PoolSize, "redis-pool-size", defaultRedisPoolSize, "Redis connection pool size")
	fs.DurationVar(&r.DialTimeout, "redis-dial-timeout", defaultRedisDialTimeout, "Redis dial timeout")
	fs.DurationVar(&r.ReadTimeout, "redis-read-timeout", defaultRedisReadTimeout, "Redis read timeout")
	fs.DurationVar(&r.WriteTimeout, "redis-write-timeout", defaultRedisWriteTimeout, "Redis write timeout")
}
