package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig `mapstructure:"server"`
	Apps     []AppConfig  `mapstructure:"apps"`
	LogLevel string       `mapstructure:"log_level"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type AppConfig struct {
	Name          string      `mapstructure:"name"`
	AppID         string      `mapstructure:"app_id"`
	AppSecret     string      `mapstructure:"app_secret"`
	EncryptKey    string      `mapstructure:"encrypt_key"`
	VerifyToken   string      `mapstructure:"verify_token"`
	FeishuBaseURL string      `mapstructure:"feishu_base_url"`
	Agent         AgentConfig `mapstructure:"agent"`
	MaxConcurrent int         `mapstructure:"max_concurrent"`
}

type AgentConfig struct {
	Platform       string        `mapstructure:"platform"`
	BaseURL        string        `mapstructure:"base_url"`
	APIPath        string        `mapstructure:"api_path"`
	Timeout        time.Duration `mapstructure:"timeout"`
	Model          string        `mapstructure:"model"`
	Token          string        `mapstructure:"token"`
	User           string        `mapstructure:"user"`
	Conversation   string        `mapstructure:"conversation"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryBaseDelay time.Duration `mapstructure:"retry_base_delay"`
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

	resolveEnvVars(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func resolveEnvVars(cfg *Config) {
	for i := range cfg.Apps {
		cfg.Apps[i].AppID = resolveEnvValue(cfg.Apps[i].AppID)
		cfg.Apps[i].AppSecret = resolveEnvValue(cfg.Apps[i].AppSecret)
		cfg.Apps[i].EncryptKey = resolveEnvValue(cfg.Apps[i].EncryptKey)
		cfg.Apps[i].VerifyToken = resolveEnvValue(cfg.Apps[i].VerifyToken)
		cfg.Apps[i].Agent.Token = resolveEnvValue(cfg.Apps[i].Agent.Token)
	}
}

func resolveEnvValue(val string) string {
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		envKey := val[2 : len(val)-1]
		return getEnvOrDefault(envKey, val)
	}
	return val
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := strings.TrimSpace(viper.GetString(key)); val != "" {
		return val
	}
	return defaultVal
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

	if len(cfg.Apps) == 0 {
		return fmt.Errorf("at least one app configuration is required")
	}

	for i := range cfg.Apps {
		app := &cfg.Apps[i]
		if app.Name == "" {
			return fmt.Errorf("app[%d]: name is required", i)
		}
		if app.MaxConcurrent == 0 {
			app.MaxConcurrent = 100
		}
		if app.Agent.Platform == "" {
			app.Agent.Platform = "hermes"
		}
		if app.Agent.BaseURL == "" {
			return fmt.Errorf("app[%d].agent.base_url is required", i)
		}
		if app.Agent.APIPath == "" {
			app.Agent.APIPath = "/v1/responses"
		}
		if app.Agent.Timeout == 0 {
			app.Agent.Timeout = 120 * time.Second
		}
		if app.Agent.Model == "" {
			app.Agent.Model = "hermes-agent"
		}
		if app.Agent.MaxRetries == 0 {
			app.Agent.MaxRetries = 3
		}
		if app.Agent.RetryBaseDelay == 0 {
			app.Agent.RetryBaseDelay = 100 * time.Millisecond
		}
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return nil
}

func (c *Config) GetAppByName(name string) *AppConfig {
	for i := range c.Apps {
		if c.Apps[i].Name == name {
			return &c.Apps[i]
		}
	}
	return nil
}

func (s *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
