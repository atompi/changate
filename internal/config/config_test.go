package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
server:
  host: "127.0.0.1"
  port: 9090
  read_timeout: 60s
  write_timeout: 60s
log_level: "debug"
etcd:
  endpoints:
    - "http://127.0.0.1:2379"
  timeout: 10s
  root_path: "/test"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if len(cfg.Etcd.Endpoints) != 1 {
		t.Errorf("len(Etcd.Endpoints) = %d, want 1", len(cfg.Etcd.Endpoints))
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	content := `
server:
  port: 0
log_level: ""
etcd:
  endpoints:
    - "http://127.0.0.1:2379"
  timeout: 0
  root_path: ""
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080 (default)", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q (default)", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 30s (default)", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 30s (default)", cfg.Server.WriteTimeout)
	}
	if cfg.Etcd.Timeout != 5*time.Second {
		t.Errorf("Etcd.Timeout = %v, want 5s (default)", cfg.Etcd.Timeout)
	}
	if cfg.Etcd.RootPath != "/changate" {
		t.Errorf("Etcd.RootPath = %q, want %q (default)", cfg.Etcd.RootPath, "/changate")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
}

func TestLoad_MissingEtcdEndpoints(t *testing.T) {
	content := `
server:
  port: 8080
etcd:
  endpoints: []
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("Load() should return error for missing etcd endpoints")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestServerAddress(t *testing.T) {
	cfg := &ServerConfig{
		Host: "192.168.1.1",
		Port: 8080,
	}

	addr := cfg.Address()
	if addr != "192.168.1.1:8080" {
		t.Errorf("Address() = %q, want %q", addr, "192.168.1.1:8080")
	}
}
