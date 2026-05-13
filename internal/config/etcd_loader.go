package config

import (
	"context"
	"fmt"
	"time"
)

type EtcdConfigLoader struct {
	client   EtcdClient
	rootPath string
}

type EtcdClient interface {
	GetAppConfig(ctx context.Context, appName string) (*AppConfig, error)
	GetUserConfig(ctx context.Context, appName, userID string) (*UserConfig, error)
}

func NewEtcdConfigLoader(client EtcdClient, rootPath string) *EtcdConfigLoader {
	if rootPath == "" {
		rootPath = "/changate"
	}
	return &EtcdConfigLoader{
		client:   client,
		rootPath: rootPath,
	}
}

func (l *EtcdConfigLoader) GetAppConfigOnly(ctx context.Context, appName string) (*AppConfig, error) {
	appCfg, err := l.client.GetAppConfig(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("app config lookup failed: %w", err)
	}

	if !appCfg.Enabled {
		return nil, fmt.Errorf("app is disabled")
	}

	return &AppConfig{
		AppID:         appCfg.AppID,
		AppSecret:     appCfg.AppSecret,
		EncryptKey:    appCfg.EncryptKey,
		VerifyToken:   appCfg.VerifyToken,
		FeishuBaseURL: appCfg.FeishuBaseURL,
		MaxConcurrent: appCfg.MaxConcurrent,
		Timeout:       appCfg.Timeout * time.Second,
		Agent:         appCfg.Agent,
	}, nil
}

func (l *EtcdConfigLoader) GetResolvedConfig(ctx context.Context, appName, userID string) (*AppConfig, error) {
	appCfg, err := l.client.GetAppConfig(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("app config lookup failed: %w", err)
	}

	if !appCfg.Enabled {
		return nil, fmt.Errorf("app is disabled")
	}

	resolved := &AppConfig{
		AppID:         appCfg.AppID,
		AppSecret:     appCfg.AppSecret,
		EncryptKey:    appCfg.EncryptKey,
		VerifyToken:   appCfg.VerifyToken,
		FeishuBaseURL: appCfg.FeishuBaseURL,
		MaxConcurrent: appCfg.MaxConcurrent,
		Timeout:       appCfg.Timeout * time.Second,
		Agent:         appCfg.Agent,
	}

	if userID != "" {
		userCfg, err := l.client.GetUserConfig(ctx, appName, userID)
		if err != nil {
			return nil, fmt.Errorf("user config lookup failed: %w", err)
		}

		if userCfg != nil {
			if !userCfg.Enabled {
				return nil, fmt.Errorf("user is disabled")
			}
			if userCfg.Agent.BaseURL != "" {
				resolved.Agent = userCfg.Agent
			}
		}
	}

	if resolved.Agent.Type == "" {
		return nil, fmt.Errorf("no agent configured for user")
	}
	if resolved.Agent.Type != "ChatCompletions" && resolved.Agent.Type != "OpenResponses" {
		return nil, fmt.Errorf("invalid agent type: %s", resolved.Agent.Type)
	}

	resolved.Agent.Timeout = resolved.Agent.Timeout * time.Second
	resolved.Agent.RetryBaseDelay = resolved.Agent.RetryBaseDelay * time.Millisecond

	return resolved, nil
}
