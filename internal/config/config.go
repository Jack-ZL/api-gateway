package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 网关配置
type Config struct {
	Port             int                    `yaml:"port"`
	LogLevel         string                 `yaml:"log_level"`
	RateLimit        RateLimitConfig        `yaml:"rate_limit"`
	Auth             AuthConfig             `yaml:"auth"`
	ServiceDiscovery ServiceDiscoveryConfig `yaml:"service_discovery"` // 服务发现配置
	Jaeger           JaegerConfig           `yaml:"jaeger"`            // Jaeger 配置
	Routes           []RouteConfig          `yaml:"routes"`
}

// RateLimitConfig 限流配置 (与之前版本相同)
type RateLimitConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Requests int           `yaml:"requests"`
	Interval time.Duration `yaml:"interval"`
}

// AuthConfig 认证配置 (与之前版本相比，新增 OAuth2 配置)
type AuthConfig struct {
	Enabled bool          `yaml:"enabled"`
	Type    string        `yaml:"type"` // "jwt", "oauth2", "apikey", "none"
	JWT     JWTAuthConfig `yaml:"jwt"`
	OAuth2  OAuth2Config  `yaml:"oauth2"` // OAuth 2.0 配置
}

// JWTAuthConfig JWT 认证配置 (与之前版本相同)
type JWTAuthConfig struct {
	SecretKey string `yaml:"secret_key"`
}

// OAuth2Config OAuth 2.0 配置
type OAuth2Config struct {
	Enabled       bool   `yaml:"enabled"`
	TokenEndpoint string `yaml:"token_endpoint"`
	ClientID      string `yaml:"client_id"`
	ClientSecret  string `yaml:"client_secret"`
}

// ServiceDiscoveryConfig 服务发现配置
type ServiceDiscoveryConfig struct {
	Enabled bool         `yaml:"enabled"`
	Type    string       `yaml:"type"` // "consul", "eureka", "none"
	Consul  ConsulConfig `yaml:"consul"`
}

// ConsulConfig Consul 配置
type ConsulConfig struct {
	Address string `yaml:"address"`
}

// JaegerConfig Jaeger 配置
type JaegerConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ServiceName  string `yaml:"service_name"`
	AgentAddress string `yaml:"agent_address"`
}

// RouteConfig 路由配置 (与之前版本相比，新增 ServiceName 字段，target_url 变为可选)
type RouteConfig struct {
	Path        string `yaml:"path"`
	TargetURL   string `yaml:"target_url"`   //  静态目标 URL (可选，如果使用服务发现则不需要)
	ServiceName string `yaml:"service_name"` //  服务发现服务名 (可选，如果使用静态 TargetURL 则不需要)
	Timeout     string `yaml:"timeout"`
}

// LoadConfig 从 YAML 文件加载配置 (与之前版本相同)
func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
