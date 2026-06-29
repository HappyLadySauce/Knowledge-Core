package options

import (
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	"github.com/HappyLadySauce/Knowledge-Core/pkg/utils/homedir"
)

const defaultLibraryDirName = "knowledge-core"

// LibraryOptions holds local Markdown library settings.
// LibraryOptions 保存本地 Markdown 知识库配置。
type LibraryOptions struct {
	// Path is the root directory used to store Markdown documents.
	// Path 是用于保存 Markdown 文档的根目录。
	Path string `json:"path" mapstructure:"path"`
}

// NewLibraryOptions returns the default local Markdown library settings.
// NewLibraryOptions 返回默认本地 Markdown 知识库配置。
func NewLibraryOptions() *LibraryOptions {
	return &LibraryOptions{Path: defaultLibraryPath()}
}

// Validate checks required library settings and normalizes the path.
// Validate 校验必要知识库配置并规范化路径。
func (o *LibraryOptions) Validate() error {
	if strings.TrimSpace(o.Path) == "" {
		o.Path = defaultLibraryPath()
	}
	o.Path = filepath.Clean(o.Path)
	return nil
}

// AddFlags registers library flags on the supplied FlagSet.
// AddFlags 将知识库相关命令行标志注册到给定的 FlagSet。
func (o *LibraryOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Path, "library-path", defaultLibraryPath(), "Markdown document library root path")
}

func defaultLibraryPath() string {
	home := homedir.HomeDir()
	if strings.TrimSpace(home) == "" {
		return defaultLibraryDirName
	}
	return filepath.Join(home, defaultLibraryDirName)
}
