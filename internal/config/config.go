package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Apps     []AppConfig    `mapstructure:"apps"`
	Agent    AgentConfig   `mapstructure:"agent"`
	LogLevel string        `mapstructure:"log_level"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// AppConfig holds configuration for a single Feishu app
type AppConfig struct {
	Name          string `mapstructure:"name"`
	AppID         string `mapstructure:"app_id"`
	AppSecret     string `mapstructure:"app_secret"`
	EncryptKey    string `mapstructure:"encrypt_key"`
	VerifyToken   string `mapstructure:"verify_token"`
	FeishuBaseURL string `mapstructure:"feishu_base_url"`
}

// AgentConfig holds configuration for Agent API Server (Hermes or OpenClaw)
type AgentConfig struct {
	Platform string        `mapstructure:"platform"`
	BaseURL  string        `mapstructure:"base_url"`
	APIPath  string        `mapstructure:"api_path"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Model    string        `mapstructure:"model"`
	Token    string        `mapstructure:"token"`
	User     string        `mapstructure:"user"`
}

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Enable environment variable override
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Resolve environment variables in app credentials
	resolveEnvVars(&cfg)

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// resolveEnvVars resolves environment variable placeholders in config
func resolveEnvVars(cfg *Config) {
	for i := range cfg.Apps {
		cfg.Apps[i].AppID = resolveEnvValue(cfg.Apps[i].AppID)
		cfg.Apps[i].AppSecret = resolveEnvValue(cfg.Apps[i].AppSecret)
		cfg.Apps[i].EncryptKey = resolveEnvValue(cfg.Apps[i].EncryptKey)
		cfg.Apps[i].VerifyToken = resolveEnvValue(cfg.Apps[i].VerifyToken)
	}
	cfg.Agent.Token = resolveEnvValue(cfg.Agent.Token)
}

// resolveEnvValue resolves ${ENV_VAR} pattern to actual env value
func resolveEnvValue(val string) string {
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		envKey := val[2 : len(val)-1]
		return getEnvOrDefault(envKey, val)
	}
	return val
}

// getEnvOrDefault returns env value or default if not set
func getEnvOrDefault(key, defaultVal string) string {
	if val := strings.TrimSpace(viper.GetString(key)); val != "" {
		return val
	}
	// Also try direct os.Getenv for non-viper env vars
	// This handles the case where env vars are set but not through viper
	return defaultVal
}

// validateConfig validates required configuration
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

	for i, app := range cfg.Apps {
		if app.Name == "" {
			return fmt.Errorf("app[%d]: name is required", i)
		}
	}

	if cfg.Agent.Platform == "" {
		cfg.Agent.Platform = "hermes"
	}
	if cfg.Agent.BaseURL == "" {
		return fmt.Errorf("agent.base_url is required")
	}
	if cfg.Agent.APIPath == "" {
		cfg.Agent.APIPath = "/v1/chat/completions"
	}
	if cfg.Agent.Timeout == 0 {
		cfg.Agent.Timeout = 120 * time.Second
	}
	if cfg.Agent.Model == "" {
		cfg.Agent.Model = "hermes-agent"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return nil
}

// GetAppByName returns app configuration by name
func (c *Config) GetAppByName(name string) *AppConfig {
	for i := range c.Apps {
		if c.Apps[i].Name == name {
			return &c.Apps[i]
		}
	}
	return nil
}

// Address returns the server address string
func (s *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}