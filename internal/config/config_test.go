package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Skip("Skipping integration test - requires config file")
	os.Setenv("FEISHU_APP_ID_1", "test_app_id")
	os.Setenv("FEISHU_APP_SECRET_1", "test_app_secret")
	defer func() {
		os.Unsetenv("FEISHU_APP_ID_1")
		os.Unsetenv("FEISHU_APP_SECRET_1")
	}()

	cfg, err := Load("config/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port == 0 {
		t.Error("Server.Port should not be 0")
	}
	if len(cfg.Apps) == 0 {
		t.Error("Apps should not be empty")
	}
}

func TestGetAppByName(t *testing.T) {
	cfg := &Config{
		Apps: []AppConfig{
			{Name: "app1"},
			{Name: "app2"},
		},
	}

	app := cfg.GetAppByName("app1")
	if app == nil {
		t.Fatal("GetAppByName() returned nil")
	}
	if app.Name != "app1" {
		t.Errorf("GetAppByName() = %v, want %v", app.Name, "app1")
	}

	app = cfg.GetAppByName("unknown")
	if app != nil {
		t.Errorf("GetAppByName() should return nil for unknown name")
	}
}

func TestServerAddress(t *testing.T) {
	s := &ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	if s.Address() != "localhost:8080" {
		t.Errorf("Address() = %v, want %v", s.Address(), "localhost:8080")
	}
}