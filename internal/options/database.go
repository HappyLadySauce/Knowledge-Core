package options

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultDatabaseURL             = "postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable"
	defaultDatabaseMaxOpenConns    = 25
	defaultDatabaseMaxIdleConns    = 5
	defaultDatabaseConnMaxLifetime = 30 * time.Minute
)

// DatabaseOptions holds PostgreSQL connection pool settings.
// DatabaseOptions 保存 PostgreSQL 连接池配置。
type DatabaseOptions struct {
	// URL is the PostgreSQL connection string.
	// URL 为 PostgreSQL 连接字符串。
	URL string `json:"url" mapstructure:"url"`
	// MaxOpenConns is the maximum number of open database connections.
	// MaxOpenConns 为最大打开数据库连接数。
	MaxOpenConns int `json:"max-open-conns" mapstructure:"max-open-conns"`
	// MaxIdleConns is the maximum number of idle database connections.
	// MaxIdleConns 为最大空闲数据库连接数。
	MaxIdleConns int `json:"max-idle-conns" mapstructure:"max-idle-conns"`
	// ConnMaxLifetime is the maximum time a database connection may be reused.
	// ConnMaxLifetime 为数据库连接可复用的最长时间。
	ConnMaxLifetime time.Duration `json:"conn-max-lifetime" mapstructure:"conn-max-lifetime"`
}

// NewDatabaseOptions returns production-ready PostgreSQL defaults.
// NewDatabaseOptions 返回 PostgreSQL 默认配置。
func NewDatabaseOptions() *DatabaseOptions {
	return &DatabaseOptions{
		URL:             defaultDatabaseURL,
		MaxOpenConns:    defaultDatabaseMaxOpenConns,
		MaxIdleConns:    defaultDatabaseMaxIdleConns,
		ConnMaxLifetime: defaultDatabaseConnMaxLifetime,
	}
}

// Validate checks required PostgreSQL settings and normalizes pool values.
// Validate 校验必要 PostgreSQL 配置并规范化连接池参数。
func (d *DatabaseOptions) Validate() error {
	if strings.TrimSpace(d.URL) == "" {
		return fmt.Errorf("database url is required")
	}
	if d.MaxOpenConns <= 0 {
		d.MaxOpenConns = defaultDatabaseMaxOpenConns
	}
	if d.MaxIdleConns < 0 {
		d.MaxIdleConns = 0
	}
	if d.MaxIdleConns > d.MaxOpenConns {
		d.MaxIdleConns = d.MaxOpenConns
	}
	if d.ConnMaxLifetime <= 0 {
		d.ConnMaxLifetime = defaultDatabaseConnMaxLifetime
	}
	return nil
}

// AddFlags registers PostgreSQL flags on the supplied FlagSet.
// AddFlags 将 PostgreSQL 相关命令行标志注册到给定的 FlagSet。
func (d *DatabaseOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&d.URL, "database-url", defaultDatabaseURL, "PostgreSQL connection URL")
	fs.IntVar(&d.MaxOpenConns, "database-max-open-conns", defaultDatabaseMaxOpenConns, "Maximum open PostgreSQL connections")
	fs.IntVar(&d.MaxIdleConns, "database-max-idle-conns", defaultDatabaseMaxIdleConns, "Maximum idle PostgreSQL connections")
	fs.DurationVar(&d.ConnMaxLifetime, "database-conn-max-lifetime", defaultDatabaseConnMaxLifetime, "Maximum PostgreSQL connection lifetime")
}
