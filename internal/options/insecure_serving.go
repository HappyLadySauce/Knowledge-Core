package options

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

type InsecureServingOptions struct {
	BindAddress string `json:"bind-address" mapstructure:"bind-address"`
	BindPort    int    `json:"bind-port"    mapstructure:"bind-port"`
	// TrustedProxies lists IP or CIDR prefixes trusted for client IP extraction (X-Forwarded-For).
	// TrustedProxies 为可信代理 IP 或 CIDR 前缀列表，用于从 X-Forwarded-For 解析客户端 IP。
	TrustedProxies []string `json:"trusted-proxies" mapstructure:"trusted-proxies"`
}

func NewInsecureServingOptions() *InsecureServingOptions {
	return &InsecureServingOptions{}
}

func (i *InsecureServingOptions) Validate() error {
	var err error
	if i.BindAddress == "" {
		err = errors.Join(err, fmt.Errorf("bind-address is required"))
	}
	if i.BindPort == 0 {
		err = errors.Join(err, fmt.Errorf("bind-port is required"))
	}
	return err
}

func (i *InsecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&i.BindAddress, "bind-address", "b", "127.0.0.1", "IP address on which to serve the --port, set to 0.0.0.0 for all interfaces")
	fs.IntVarP(&i.BindPort, "bind-port", "p", 8080, "port to listen to for incoming HTTPS requests")
	fs.StringSliceVar(&i.TrustedProxies, "trusted-proxies", nil,
		"Trusted proxy IPs or CIDRs for Forwarded headers (repeat flag or comma-separated); empty means trust no proxies")
}
