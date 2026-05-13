package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/atompi/changate/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.EtcdConfig{
		Endpoints: []string{"http://127.0.0.1:23790"},
		Timeout:   5 * time.Second,
		RootPath:  "/changate",
	}

	cli, err := NewClient(cfg)
	if err != nil {
		t.Skipf("etcd not available: %v", err)
	}
	defer cli.Close()

	if cli.rootPath != "/changate" {
		t.Errorf("expected rootPath /changate, got %s", cli.rootPath)
	}
}

func TestGetAppConfig_NotFound(t *testing.T) {
	cfg := &config.EtcdConfig{
		Endpoints: []string{"http://127.0.0.1:23790"},
		Timeout:   5 * time.Second,
		RootPath:  "/changate",
	}

	cli, err := NewClient(cfg)
	if err != nil {
		t.Skipf("etcd not available: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.GetAppConfig(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent app")
	}
}
