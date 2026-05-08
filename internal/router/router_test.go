package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atompi/changate/internal/config"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestConfig(apps []config.AppConfig) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Apps:     apps,
		LogLevel: "info",
	}
}

func TestSetup_EmptyAppsList(t *testing.T) {
	cfg := newTestConfig([]config.AppConfig{})

	result := Setup(cfg)

	if result == nil {
		t.Fatal("Setup() returned nil")
	}
	if result.Engine == nil {
		t.Error("result.Engine is nil")
	}
	if result.Handler == nil {
		t.Error("result.Handler is nil")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Body.Len() == 0 {
		t.Error("health endpoint body is empty")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/feishu/unknown", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("feishu unknown app status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetup_OneAppConfigured(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "testapp",
			AppID:         "cli_test_app_id",
			AppSecret:    "test_secret",
			VerifyToken:   "test_verify_token",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				Platform: "hermes",
				BaseURL:  "http://127.0.0.1:8642",
				APIPath:  "/v1/chat/completions",
				Timeout:  120 * time.Second,
				Model:    "hermes-agent",
				Token:    "test_token",
				User:     "test_user",
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	if result == nil {
		t.Fatal("Setup() returned nil")
	}
	if result.Engine == nil {
		t.Error("result.Engine is nil")
	}
	if result.Handler == nil {
		t.Error("result.Handler is nil")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint status = %d, want %d", w.Code, http.StatusOK)
	}

	w = httptest.NewRecorder()
	unknownReq := httptest.NewRequest(http.MethodPost, "/feishu/unknownapp", nil)
	result.Engine.ServeHTTP(w, unknownReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("feishu unknown app status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetup_RoutesRegisteredCorrectly(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "app1",
			AppID:         "cli_app1",
			AppSecret:    "secret1",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				Platform: "hermes",
				BaseURL:  "http://localhost:8642",
				APIPath:  "/v1/responses",
				Timeout:  120 * time.Second,
				Model:    "test-model",
				Token:    "token1",
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health status = %d, want %d", w.Code, http.StatusOK)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/feishu/nonexistent", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /feishu/nonexistent status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetup_AgentClientsCreated(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "app1",
			AppID:         "cli_app1",
			AppSecret:    "secret1",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				Platform: "hermes",
				BaseURL:  "http://127.0.0.1:8642",
				APIPath:  "/v1/responses",
				Timeout:  120 * time.Second,
				Model:    "hermes-agent",
				Token:    "token1",
				User:     "user1",
			},
		},
		{
			Name:          "app2",
			AppID:         "cli_app2",
			AppSecret:    "secret2",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				Platform: "openclaw",
				BaseURL:  "http://127.0.0.1:8080",
				APIPath:  "/v1/responses",
				Timeout:  120 * time.Second,
				Model:    "openclaw/default",
				Token:    "token2",
				User:     "user2",
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	if result == nil {
		t.Fatal("Setup() returned nil")
	}

	for _, appName := range []string{"app1", "app2"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/feishu/"+appName, nil)
		result.Engine.ServeHTTP(w, req)

		if w.Code == http.StatusNotFound {
			t.Errorf("POST /feishu/%s returned 404 (app not found), expected different status", appName)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feishu/unknown", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /feishu/unknown status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetup_ReturnsRouterResult(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "testapp",
			AppID:         "cli_test",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				BaseURL: "http://localhost:8642",
				APIPath: "/v1/responses",
				Model:   "test-model",
				Token:   "test-token",
				Timeout: 120 * time.Second,
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	if result == nil {
		t.Fatal("Setup() returned nil")
	}

	if result.Engine == nil {
		t.Error("RouterResult.Engine is nil")
	}
	if result.Handler == nil {
		t.Error("RouterResult.Handler is nil")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("engine failed to serve request: status = %d", w.Code)
	}
}

func TestSetup_MultipleApps(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "app1",
			AppID:         "cli_app1",
			AppSecret:    "secret1",
			VerifyToken:   "token1",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				BaseURL: "http://127.0.0.1:8642",
				APIPath: "/v1/responses",
				Model:   "model1",
				Token:   "token1",
				Timeout: 120 * time.Second,
			},
		},
		{
			Name:          "app2",
			AppID:         "cli_app2",
			AppSecret:    "secret2",
			VerifyToken:   "token2",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				BaseURL: "http://127.0.0.1:8080",
				APIPath: "/v1/responses",
				Model:   "model2",
				Token:   "token2",
				Timeout: 120 * time.Second,
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	if result == nil {
		t.Fatal("Setup() returned nil")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health endpoint status = %d, want %d", w.Code, http.StatusOK)
	}

	for _, appName := range []string{"app1", "app2"} {
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/feishu/"+appName, nil)
		result.Engine.ServeHTTP(w, req)

		if w.Code == http.StatusNotFound {
			t.Errorf("app %q returned 404, expected different status", appName)
		}
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/feishu/unknown", nil)
	result.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown app status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetup_URLVerificationRoute(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "myapp",
			AppID:         "cli_myapp",
			AppSecret:    "secret",
			VerifyToken:   "my-token",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				BaseURL: "http://localhost:8642",
				APIPath: "/v1/responses",
				Model:   "test-model",
				Token:   "test-token",
				Timeout: 120 * time.Second,
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	body := []byte(`{"type":"url_verification","challenge":"test-challenge-123","token":"my-token"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feishu/myapp", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	result.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("URL verification status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestSetup_RouteMethods(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name:          "app1",
			AppID:         "cli_app1",
			FeishuBaseURL: "https://open.feishu.cn",
			Agent: config.AgentConfig{
				BaseURL: "http://localhost:8642",
				APIPath: "/v1/responses",
				Model:   "model",
				Token:   "token",
				Timeout: 120 * time.Second,
			},
		},
	}
	cfg := newTestConfig(apps)

	result := Setup(cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	result.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /health failed: status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/feishu/app1", nil)
	result.Engine.ServeHTTP(w, req)
	if w.Code == http.StatusMethodNotAllowed {
		t.Error("/feishu/:appName should accept POST")
	}
}