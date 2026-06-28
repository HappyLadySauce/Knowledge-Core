package options

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/HappyLadySauce/Knowledge-Core/pkg/utils/homedir"
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

// NewSQLiteOptions returns empty SQLite options; defaults are provided by flags.
// NewSQLiteOptions 返回空 SQLite 配置；默认值由命令行标志提供。
func NewSQLiteOptions() *SQLiteOptions {
	return &SQLiteOptions{}
}

// Validate checks required SQLite settings and normalizes the database path.
// Validate 校验必要 SQLite 配置并规范化数据库路径。
func (s *SQLiteOptions) Validate() error {
	var err error
	if strings.TrimSpace(s.Path) == "" {
		err = errors.Join(err, fmt.Errorf("sqlite path is required"))
	}
	if s.BusyTimeout <= 0 {
		err = errors.Join(err, fmt.Errorf("sqlite busy-timeout must be > 0, got %s", s.BusyTimeout))
	}
	return err
}

// AddFlags registers SQLite flags on the supplied FlagSet.
// AddFlags 将 SQLite 相关命令行标志注册到给定的 FlagSet。
func (s *SQLiteOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Path, "sqlite-path", defaultSQLitePath(), "SQLite index database path")
	fs.DurationVar(&s.BusyTimeout, "sqlite-busy-timeout", 5*time.Second, "SQLite busy timeout")
}

func defaultSQLitePath() string {
	return filepath.Join(homedir.HomeDir(), ".knowledge-core", "index.db")
}
