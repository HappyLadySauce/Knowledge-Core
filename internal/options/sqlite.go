package options

import (
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultSQLiteDBPath      = ".knowledge-core/index.db"
	defaultSQLiteBusyTimeout = 5 * time.Second
)

// SQLiteOptions holds local SQLite index database settings.
// SQLiteOptions 保存本地 SQLite 索引数据库配置。
type SQLiteOptions struct {
	// Path is the local SQLite database file path.
	// Path 为本地 SQLite 数据库文件路径。
	Path string `json:"path" mapstructure:"path"`
	// BusyTimeout is the maximum time SQLite waits for a locked database.
	// BusyTimeout 为 SQLite 等待数据库锁释放的最长时间。
	BusyTimeout time.Duration `json:"busy-timeout" mapstructure:"busy-timeout"`
}

// NewSQLiteOptions returns hardcoded SQLite defaults for the local app database.
// NewSQLiteOptions 返回本地应用数据库的硬编码 SQLite 默认配置。
func NewSQLiteOptions() *SQLiteOptions {
	return &SQLiteOptions{
		Path:        defaultSQLiteDBPath,
		BusyTimeout: defaultSQLiteBusyTimeout,
	}
}

// Validate checks required SQLite settings and normalizes the database path.
// Validate 校验必要 SQLite 配置并规范化数据库路径。
func (s *SQLiteOptions) Validate() error {
	var err error
	if strings.TrimSpace(s.Path) == "" {
		s.Path = defaultSQLiteDBPath
	}
	if s.BusyTimeout <= 0 {
		s.BusyTimeout = defaultSQLiteBusyTimeout
	}
	return err
}

// AddFlags registers SQLite flags on the supplied FlagSet.
// AddFlags 将 SQLite 相关命令行标志注册到给定的 FlagSet。
func (s *SQLiteOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Path, "sqlite-path", defaultSQLiteDBPath, "SQLite index database path")
	fs.DurationVar(&s.BusyTimeout, "sqlite-busy-timeout", defaultSQLiteBusyTimeout, "SQLite busy timeout")
}
