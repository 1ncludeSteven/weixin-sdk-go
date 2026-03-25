// Package config provides configuration management.
package config

// AccountConfig represents per-account configuration.
type AccountConfig struct {
	Name       string `json:"name,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
	CDNBaseURL string `json:"cdnBaseUrl,omitempty"`
	RouteTag   string `json:"routeTag,omitempty"`
}

// ChannelConfig represents channel-level configuration.
type ChannelConfig struct {
	Accounts      map[string]*AccountConfig `json:"accounts,omitempty"`
	Enabled       bool                      `json:"enabled,omitempty"`
	LogUploadURL  string                    `json:"logUploadUrl,omitempty"`
	RouteTag      string                    `json:"routeTag,omitempty"`
}

// DefaultCDNBaseURL is the default CDN base URL.
const DefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"

// DefaultBaseURL is the default API base URL.
const DefaultBaseURL = "https://ilinkai.weixin.qq.com"
