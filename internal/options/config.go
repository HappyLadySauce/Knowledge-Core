package options

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/HappyLadySauce/Knowledge-Core/pkg/utils/homedir"
)

const (
	configFlagName = "config"
)

var cfgFile string

func init() {
	pflag.StringVarP(&cfgFile, "config", "f", cfgFile, "Read configuration from specified `FILE`, "+
		"support JSON, TOML, YAML, HCL, or Java properties formats.")
}

// AddConfigFlag registers the shared --config flag and wires Viper loading for basename.
// AddConfigFlag 注册共用的 --config 标志，并按 basename 接入 Viper 配置加载。
func AddConfigFlag(fs *pflag.FlagSet, basename string) {
	fs.AddFlag(pflag.Lookup(configFlagName))

	viper.AutomaticEnv()
	viper.SetEnvPrefix(strings.Replace(strings.ToUpper(basename), "-", "_", -1))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	cobra.OnInitialize(func() {
		if cfgFile != "" {
			// Support ${ENV_VAR} expansion inside config files so values can be injected via the environment (e.g. from Make).
			// 支持配置文件内的 ${ENV_VAR} 展开，以便通过环境变量注入配置（例如由 Makefile 传入）。
			b, err := os.ReadFile(cfgFile)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: failed to read configuration file(%s): %v\n", cfgFile, err)
				os.Exit(1)
			}

			expanded := os.ExpandEnv(string(b))
			ext := strings.TrimPrefix(filepath.Ext(cfgFile), ".")
			if ext != "" {
				viper.SetConfigType(ext)
			}
			if err := viper.ReadConfig(strings.NewReader(expanded)); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: failed to read configuration file(%s): %v\n", cfgFile, err)
				os.Exit(1)
			}
			return
		} else {
			viper.AddConfigPath(".")

			if names := strings.Split(basename, "-"); len(names) > 1 {
				viper.AddConfigPath(filepath.Join(homedir.HomeDir(), "."+names[0]))
				viper.AddConfigPath(filepath.Join("/etc", names[0]))
			}

			viper.SetConfigName(basename)
		}

		if err := viper.ReadInConfig(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: failed to read configuration file(%s): %v\n", cfgFile, err)
			os.Exit(1)
		}
	})
}
