package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig `mapstructure:"server"`
	Etcd     EtcdConfig   `mapstructure:"etcd"`
	LogLevel string       `mapstructure:"log_level"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type EtcdConfig struct {
	Endpoints          []string      `mapstructure:"endpoints"`
	Timeout            time.Duration `mapstructure:"timeout"`
	CAFile             string        `mapstructure:"ca_file"`
	CertFile           string        `mapstructure:"cert_file"`
	KeyFile            string        `mapstructure:"key_file"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
	RootPath           string        `mapstructure:"root_path"`
}

type AgentConfig struct {
	Type           string        `json:"type"`
	BaseURL        string        `json:"base_url"`
	APIPath        string        `json:"api_path"`
	Timeout        time.Duration `json:"timeout"`
	Model          string        `json:"model"`
	Token          string        `json:"token"`
	User           string        `json:"user"`
	Conversation   string        `json:"conversation"`
	MaxRetries     int           `json:"max_retries"`
	RetryBaseDelay time.Duration `json:"retry_base_delay"`
}

// AppConfig is the configuration stored at /changate/<app_name>
type AppConfig struct {
	Enabled       bool          `json:"enabled"`
	AppID         string        `json:"app_id"`
	AppSecret     string        `json:"app_secret"`
	EncryptKey    string        `json:"encrypt_key"`
	VerifyToken   string        `json:"verify_token"`
	FeishuBaseURL string        `json:"feishu_base_url"`
	MaxConcurrent int           `json:"max_concurrent"`
	Timeout       time.Duration `json:"timeout"`
	Agent         AgentConfig   `json:"agent"`
}

// UserConfig is the configuration stored at /changate/<app_name>/<user_id>
type UserConfig struct {
	Enabled bool        `json:"enabled"`
	Agent   AgentConfig `json:"agent"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}

	// ETCD-based config - apps are loaded from etcd
	if len(cfg.Etcd.Endpoints) == 0 {
		return fmt.Errorf("etcd.endpoints is required")
	}
	if cfg.Etcd.Timeout == 0 {
		cfg.Etcd.Timeout = 5 * time.Second
	}
	if cfg.Etcd.RootPath == "" {
		cfg.Etcd.RootPath = "/changate"
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return nil
}

func (s *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
