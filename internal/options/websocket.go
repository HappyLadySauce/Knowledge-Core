package options

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

var defaultWebSocketAllowedOrigins = []string{
	"http://localhost:*",
	"http://127.0.0.1:*",
}

// WebSocketOptions holds realtime channel security settings.
// WebSocketOptions 保存实时通道安全配置。
type WebSocketOptions struct {
	// AllowedOrigins lists trusted browser origins for websocket upgrades.
	// AllowedOrigins 为允许发起 WebSocket 升级的可信浏览器来源。
	AllowedOrigins []string `json:"allowed-origins" mapstructure:"allowed-origins"`
}

// NewWebSocketOptions returns development-safe websocket defaults.
// NewWebSocketOptions 返回开发环境可用的 WebSocket 默认配置。
func NewWebSocketOptions() *WebSocketOptions {
	return &WebSocketOptions{AllowedOrigins: append([]string{}, defaultWebSocketAllowedOrigins...)}
}

// Validate normalizes allowed origins.
// Validate 规范化允许的来源配置。
func (w *WebSocketOptions) Validate() error {
	if len(w.AllowedOrigins) == 0 {
		w.AllowedOrigins = append([]string{}, defaultWebSocketAllowedOrigins...)
		return nil
	}
	if len(w.AllowedOrigins) == 1 {
		value := strings.TrimSpace(w.AllowedOrigins[0])
		if strings.HasPrefix(value, "[") {
			var origins []string
			if err := json.Unmarshal([]byte(value), &origins); err != nil {
				return fmt.Errorf("websocket allowed origins must be a JSON string array: %w", err)
			}
			w.AllowedOrigins = origins
		}
	}
	normalized := make([]string, 0, len(w.AllowedOrigins))
	for _, origin := range w.AllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			normalized = append(normalized, origin)
		}
	}
	if len(normalized) == 0 {
		w.AllowedOrigins = append([]string{}, defaultWebSocketAllowedOrigins...)
		return nil
	}
	w.AllowedOrigins = normalized
	return nil
}

// AddFlags registers websocket flags on the supplied FlagSet.
// AddFlags 将 WebSocket 相关命令行标志注册到给定的 FlagSet。
func (w *WebSocketOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&w.AllowedOrigins, "websocket-allowed-origins", defaultWebSocketAllowedOrigins,
		"Allowed websocket origins; supports exact origins and localhost/127.0.0.1 port wildcard")
}
