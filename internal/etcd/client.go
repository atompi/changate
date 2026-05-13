package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/atompi/changate/internal/config"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Client struct {
	cli      *clientv3.Client
	rootPath string
	mu       sync.RWMutex
}

func NewClient(cfg *config.EtcdConfig) (*Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certs")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.Timeout,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	rootPath := cfg.RootPath
	if rootPath == "" {
		rootPath = "/changate"
	}

	return &Client{
		cli:      cli,
		rootPath: rootPath,
	}, nil
}

func (c *Client) GetAppConfig(ctx context.Context, appName string) (*config.AppConfig, error) {
	key := fmt.Sprintf("%s/%s", c.rootPath, appName)

	resp, err := c.cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get app config: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("app config not found: %s", appName)
	}

	var appCfg config.AppConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &appCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app config: %w", err)
	}

	return &appCfg, nil
}

func (c *Client) GetUserConfig(ctx context.Context, appName, userID string) (*config.UserConfig, error) {
	key := fmt.Sprintf("%s/%s/%s", c.rootPath, appName, userID)

	resp, err := c.cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil // User config not found - use app default
	}

	var userCfg config.UserConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &userCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user config: %w", err)
	}

	return &userCfg, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}
