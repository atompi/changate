package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleCallback_AppNotFound(t *testing.T) {
	handler := NewCallbackHandler([]config.AppConfig{}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/unknown", nil)

	handler.HandleCallback(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleCallback_URLVerification(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp", VerifyToken: "test-token"},
	}
	handler := NewCallbackHandler(apps, nil)

	body := `{"type":"url_verification","challenge":"test-challenge-123","token":"test-token"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp model.URLVerificationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Challenge != "test-challenge-123" {
		t.Errorf("challenge = %q, want %q", resp.Challenge, "test-challenge-123")
	}
}

func TestHandleCallback_TokenMismatch(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp", VerifyToken: "correct-token"},
	}
	handler := NewCallbackHandler(apps, nil)

	body := `{"type":"","token":"wrong-token"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func computeSignature(timestamp, nonce, body, key string) string {
	var b strings.Builder
	b.WriteString(timestamp)
	b.WriteString(nonce)
	b.WriteString(key)
	b.WriteString(body)
	hash := sha256.New()
	hash.Write([]byte(b.String()))
	return hex.EncodeToString(hash.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	handler := &CallbackHandler{}

	key := "test-encrypt-key"
	body := `{"test":"data"}`
	timestamp := "1608725989000"
	nonce := "test-nonce"

	sig := computeSignature(timestamp, nonce, body, key)
	if !handler.verifySignature(timestamp, nonce, sig, body, key) {
		t.Error("verifySignature() should return true for valid signature")
	}

	if handler.verifySignature(timestamp, nonce, "invalid-sig", body, key) {
		t.Error("verifySignature() should return false for invalid signature")
	}

	if handler.verifySignature(timestamp, nonce, sig, body, "different-key") {
		t.Error("verifySignature() should return false for different key")
	}

	if !handler.verifySignature("", "", "", "", "") {
		t.Error("verifySignature() should return true when all params are empty")
	}

	if !handler.verifySignature(timestamp, nonce, sig, body, "") {
		t.Error("verifySignature() should return true when key is empty (signature verification skipped)")
	}
}

func encryptForTest(key, plaintext string) string {
	keyBytes := sha256.Sum256([]byte(key))
	block, _ := aes.NewCipher(keyBytes[:])
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(padded, padded)
	return base64.StdEncoding.EncodeToString(append(iv, padded...))
}

func TestDecryptBody(t *testing.T) {
	handler := &CallbackHandler{}

	plaintext := `{"type":"test","data":"hello"}`
	encrypted := encryptForTest("test-key", plaintext)

	body := []byte(`{"encrypt":"` + encrypted + `"}`)

	decrypted, err := handler.decryptBody(body, "test-key")
	if err != nil {
		t.Fatalf("decryptBody() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(decrypted, &result); err != nil {
		t.Fatalf("failed to unmarshal decrypted data: %v", err)
	}
	if result["data"] != "hello" {
		t.Errorf("decrypted data = %v, want hello", result["data"])
	}
}

func TestDecryptBody_NoEncryption(t *testing.T) {
	handler := &CallbackHandler{}

	body := []byte(`{"type":"test","data":"hello"}`)

	decrypted, err := handler.decryptBody(body, "")
	if err != nil {
		t.Fatalf("decryptBody() error = %v", err)
	}

	if !bytes.Equal(decrypted, body) {
		t.Errorf("decryptBody() should return original body when encryptKey is empty")
	}
}

func TestDecryptBody_EmptyEncryptField(t *testing.T) {
	handler := &CallbackHandler{}

	body := []byte(`{"encrypt":""}`)

	decrypted, err := handler.decryptBody(body, "some-key")
	if err != nil {
		t.Fatalf("decryptBody() error = %v", err)
	}

	if !bytes.Equal(decrypted, body) {
		t.Errorf("decryptBody() should return original body when encrypt field is empty")
	}
}

func TestHandleMessageEvent_NonMessageEvent(t *testing.T) {
	handler := NewCallbackHandler([]config.AppConfig{}, nil)

	eventData := []byte(`{"sender":{"sender_id":{}},"message":{"message_id":"msg123","message_type":"text","content":"{\"text\":\"hello\"}"}}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/test", bytes.NewBuffer(eventData))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.handleMessageEvent(c, &config.AppConfig{Name: "test"}, eventData)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNewCallbackHandler(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "app1", AppID: "id1"},
		{Name: "app2", AppID: "id2"},
	}

	testClient := agent.NewClient("http://localhost", "/v1/responses", "test-model", "test-token", "", "", 120*time.Second, 3, 100*time.Millisecond)
	agentClients := map[string]agent.Client{
		"app1": testClient,
	}
	handler := NewCallbackHandler(apps, agentClients)

	if handler.apps["app1"] == nil {
		t.Error("handler.apps['app1'] should not be nil")
	}
	if handler.apps["app2"] == nil {
		t.Error("handler.apps['app2'] should not be nil")
	}
	if handler.agentClients["app1"] != testClient {
		t.Error("handler.agentClients['app1'] should be set to testClient")
	}
	if handler.agentClients["app2"] != nil {
		t.Error("handler.agentClients['app2'] should be nil")
	}
}

// MockAgentClient is a mock implementation of agent.Client for testing.
type MockAgentClient struct {
	ResponsesFunc       func(ctx context.Context, messages []model.Message) (*model.ResponsesResponse, error)
	ResponsesWithContentFunc func(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error)
	GetTimeoutFunc      func() time.Duration
}

func (m *MockAgentClient) Responses(ctx context.Context, messages []model.Message) (*model.ResponsesResponse, error) {
	if m.ResponsesFunc != nil {
		return m.ResponsesFunc(ctx, messages)
	}
	return nil, nil
}

func (m *MockAgentClient) ResponsesWithContent(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error) {
	if m.ResponsesWithContentFunc != nil {
		return m.ResponsesWithContentFunc(ctx, contentParts)
	}
	return nil, nil
}

func (m *MockAgentClient) GetTimeout() time.Duration {
	if m.GetTimeoutFunc != nil {
		return m.GetTimeoutFunc()
	}
	return 120 * time.Second
}

// MockFeishuClient is a mock Feishu client for testing.
type MockFeishuClient struct {
	SendTextMessageFunc    func(ctx context.Context, accessToken, messageID, text string) error
	SendFileMessageFunc   func(ctx context.Context, accessToken, messageID, imageURL string) error
	GetAppAccessTokenFunc  func(ctx context.Context) (string, error)
}

func (m *MockFeishuClient) SendTextMessage(ctx context.Context, accessToken, messageID, text string) error {
	if m.SendTextMessageFunc != nil {
		return m.SendTextMessageFunc(ctx, accessToken, messageID, text)
	}
	return nil
}

func (m *MockFeishuClient) SendFileMessage(ctx context.Context, accessToken, messageID, imageURL string) error {
	if m.SendFileMessageFunc != nil {
		return m.SendFileMessageFunc(ctx, accessToken, messageID, imageURL)
	}
	return nil
}

func (m *MockFeishuClient) GetAppAccessToken(ctx context.Context) (string, error) {
	if m.GetAppAccessTokenFunc != nil {
		return m.GetAppAccessTokenFunc(ctx)
	}
	return "mock-token", nil
}

func TestHandleMessageEvent_Returns200Immediately(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp"},
	}
	mockClient := &MockAgentClient{
		ResponsesWithContentFunc: func(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error) {
			return &model.ResponsesResponse{
				ID:      "resp-123",
				Object:  "response",
				Status:  "completed",
				Model:   "test-model",
				Output: []model.Output{
					{
						Type:   "message",
						Role:   "assistant",
						Content: []model.Content{
							{Type: "text", Text: "Test response"},
						},
					},
				},
			}, nil
		},
	}
	agentClients := map[string]agent.Client{
		"testapp": mockClient,
	}
	handler := NewCallbackHandler(apps, agentClients)

	eventData := []byte(`{"sender":{"sender_id":{}},"message":{"message_id":"msg123","message_type":"text","content":"{\"text\":\"hello\"}"}}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBuffer(eventData))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.handleMessageEvent(c, &apps[0], eventData)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["code"] != float64(0) {
		t.Errorf("response code = %v, want 0", resp["code"])
	}
}

func TestParseMessageContent_ImageType(t *testing.T) {
	content := `{"image_key":"img_v3_xxx"}`
	parts, err := model.ParseMessageContent(content, "image")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Type != "input_image" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_image")
	}
}

func TestParseMessageContent_PostType(t *testing.T) {
	content := `{"title":"","content":[[{"tag":"text","text":"hello"}]]}`
	parts, err := model.ParseMessageContent(content, "post")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Type != "input_text" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_text")
	}
}

func TestHandleMessageEvent_WrongSignature_Unauthorized(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp", EncryptKey: "test-encrypt-key"},
	}
	handler := NewCallbackHandler(apps, nil)

	body := []byte(`{"schema":"2.0","header":{"event_type":"im.message.receive_v1"},"event":{"sender":{"sender_id":{}},"message":{"message_id":"msg123","content":"{\"text\":\"hello\"}"}}}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Lark-Signature", "wrong-signature")
	c.Request.Header.Set("X-Lark-Request-Timestamp", "1608725989000")
	c.Request.Header.Set("X-Lark-Request-Nonce", "test-nonce")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleCallback_EmptyBody(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp"},
	}
	handler := NewCallbackHandler(apps, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBufferString(""))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleCallback_InvalidJSON(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "testapp"},
	}
	handler := NewCallbackHandler(apps, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBufferString("not valid json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleCallback_EncryptedBody(t *testing.T) {
	encryptKey := "test-encrypt-key-32bytes-long!!!!"
	plaintext := `{"schema":"2.0","header":{"event_type":"im.message.update_v1"},"event":{"sender":{"sender_id":{}},"message":{"message_id":"msg123","content":"{\"text\":\"hello\"}"}}}`
	encrypted := encryptForTest(encryptKey, plaintext)

	timestamp := "1608725989000"
	nonce := "test-nonce"
	body := []byte(`{"encrypt":"` + encrypted + `"}`)
	sig := computeSignature(timestamp, nonce, string(body), encryptKey)

	apps := []config.AppConfig{
		{Name: "testapp", EncryptKey: encryptKey},
	}
	handler := NewCallbackHandler(apps, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/testapp", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Lark-Signature", sig)
	c.Request.Header.Set("X-Lark-Request-Timestamp", timestamp)
	c.Request.Header.Set("X-Lark-Request-Nonce", nonce)
	c.Params = []gin.Param{{Key: "appName", Value: "testapp"}}

	handler.HandleCallback(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProcessMessageAsync_MockAgentClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"test_tenant_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/auth/v3/app_access_token/internal" {
			w.Write([]byte(`{"code":0,"msg":"success","app_access_token":"test_app_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/im/v1/messages/msg123/reply" {
			w.Write([]byte(`{"code":0,"msg":"success"}`))
		}
	}))
	defer server.Close()

	apps := []config.AppConfig{
		{
			Name:          "testapp",
			AppID:         "test-app-id",
			AppSecret:     "test-app-secret",
			FeishuBaseURL: server.URL,
		},
	}

	agentCalled := false

	mockAgentClient := &MockAgentClient{
		ResponsesWithContentFunc: func(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error) {
			agentCalled = true
			return &model.ResponsesResponse{
				ID:      "resp-123",
				Object:  "response",
				Status:  "completed",
				Model:   "test-model",
				Output: []model.Output{
					{
						Type:   "message",
						Role:   "assistant",
						Content: []model.Content{
							{Type: "text", Text: "This is a test response"},
						},
					},
				},
			}, nil
		},
	}

	agentClients := map[string]agent.Client{
		"testapp": mockAgentClient,
	}
	handler := NewCallbackHandler(apps, agentClients)

	contentParts := []model.ContentPart{
		{Type: "text", Text: "hello"},
	}

	handler.processMessageAsync("testapp", &apps[0], contentParts, "msg123")

	handler.WaitForCompletion()

	if !agentCalled {
		t.Error("agent client was not called")
	}
}

func TestDecryptBody_InvalidEncrypt(t *testing.T) {
	handler := &CallbackHandler{}

	body := []byte(`{"encrypt":"invalid-base64-!!!!"}`)

	_, err := handler.decryptBody(body, "test-key")
	if err == nil {
		t.Error("decryptBody() should return error for invalid base64")
	}
}
