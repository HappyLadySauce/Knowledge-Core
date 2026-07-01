package options

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

const defaultRedisURL = "redis://localhost:6379/0"

// RedisOptions holds Redis connectivity settings.
// RedisOptions 保存 Redis 连接配置。
type RedisOptions struct {
	// URL is the Redis connection URL.
	// URL 为 Redis 连接字符串。
	URL string `json:"url" mapstructure:"url"`
}

// NewRedisOptions returns Redis defaults.
// NewRedisOptions 返回 Redis 默认配置。
func NewRedisOptions() *RedisOptions {
	return &RedisOptions{
		URL: defaultRedisURL,
	}
}

// Validate checks Redis settings.
// Validate 校验 Redis 配置。
func (r *RedisOptions) Validate() error {
	if r == nil {
		return fmt.Errorf("redis config is nil")
	}
	r.URL = strings.TrimSpace(r.URL)
	if r.URL == "" {
		return fmt.Errorf("redis url is required")
	}
	return nil
}

// AddFlags registers Redis flags on the supplied FlagSet.
// AddFlags 将 Redis 相关命令行标志注册到给定的 FlagSet。
func (r *RedisOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.URL, "redis-url", defaultRedisURL, "Redis connection URL")
}
