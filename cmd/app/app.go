package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/options"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/router"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/routes/auth"
	userroute "github.com/HappyLadySauce/Knowledge-Core/cmd/app/routes/user"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
)

func NewAPICommand(ctx context.Context, basename string) *cobra.Command {
	opts := options.NewOptions(basename)
	cmd := &cobra.Command{
		Use:   basename,
		Short: basename + " is a web server",
		Long:  basename + " is a web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bind command-line flags to Viper (CLI values override the config file).
			// 将命令行标志绑定到 Viper（命令行参数覆盖配置文件）。
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			if err := viper.Unmarshal(opts); err != nil {
				return err
			}

			// Initialize logging after flags are parsed and configuration is loaded.
			// 在解析完标志并加载配置后初始化日志。
			logs.InitLogs()
			defer logs.FlushLogs()

			// Validate options after flags and configuration are fully populated.
			// 在标志与配置全部就绪后校验选项。
			if err := opts.Validate(); err != nil {
				return err
			}
			return run(ctx, opts)
		},
	}

	nfs := opts.AddFlags(cmd.Flags())
	flag.SetUsageAndHelpFunc(cmd, *nfs, 80)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	cfg := &config.Config{
		InsecureServing: opts.InsecureServing,
		SQLite:          opts.SQLite,
		JWT:             opts.JWT,
	}
	config.Init(cfg)

	if err := router.ConfigureTrustedProxies(opts.InsecureServing.TrustedProxies); err != nil {
		return fmt.Errorf("configure trusted proxies: %w", err)
	}

	sc, err := svc.NewServiceContext(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sc.Close(); closeErr != nil {
			klog.ErrorS(closeErr, "failed to close service context")
		}
	}()

	// Initialize HTTP route handlers after the service context is ready.
	// 在服务上下文就绪后初始化 HTTP 路由处理器。
	if err := routesInit(ctx, sc); err != nil {
		return err
	}

	serve(opts)
	<-ctx.Done()
	return nil
}

func serve(opts *options.Options) {
	insecureAddress := fmt.Sprintf("%s:%d", opts.InsecureServing.BindAddress, opts.InsecureServing.BindPort)
	klog.V(2).InfoS("Listening and serving on", "address", insecureAddress)
	go func() {
		klog.Fatal(router.Router().Run(insecureAddress))
	}()
}

// Initialize HTTP route handlers after the service context is ready.
// 在服务上下文就绪后初始化 HTTP 路由处理器。
func routesInit(ctx context.Context, sc *svc.ServiceContext) error {
	auth.Init(ctx, sc)
	userroute.Init(ctx, sc)
	return nil
}
