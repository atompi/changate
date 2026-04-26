package handler

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/hermes"
	"github.com/atompi/changate/internal/model"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleCallback_AppNotFound(t *testing.T) {
	handler := NewCallbackHandler([]config.AppConfig{}, nil, 120)

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
	handler := NewCallbackHandler(apps, nil, 120)

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
	handler := NewCallbackHandler(apps, nil, 120)

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

func computeSignature(timestamp, body, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	handler := &CallbackHandler{}

	key := "test-encrypt-key"
	body := `{"test":"data"}`
	timestamp := "1608725989000"

	sig := computeSignature(timestamp, body, key)
	if !handler.verifySignature(timestamp, sig, body, key) {
		t.Error("verifySignature() should return true for valid signature")
	}

	if handler.verifySignature(timestamp, "invalid-sig", body, key) {
		t.Error("verifySignature() should return false for invalid signature")
	}

	if handler.verifySignature(timestamp, sig, body, "different-key") {
		t.Error("verifySignature() should return false for different key")
	}

	if !handler.verifySignature(timestamp, sig, body, "") {
		t.Error("verifySignature() should return true when key is empty")
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
	hermesClient := hermes.NewClient("http://localhost", "/v1/chat", "test-model", "test-token", "", 120)
	handler := NewCallbackHandler([]config.AppConfig{}, hermesClient, 120)

	body := []byte(`{"schema":"2.0","header":{"event_type":"im.message.receive_v1"},"event":{"sender":{"sender_id":{}},"message":{"message_id":"msg123","content":"{\"text\":\"hello\"}"}}}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/feishu/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.handleMessageEvent(c, &config.AppConfig{Name: "test"}, body)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNewCallbackHandler(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "app1", AppID: "id1"},
		{Name: "app2", AppID: "id2"},
	}

	hermesClient := hermes.NewClient("http://localhost", "/v1/chat", "test-model", "test-token", "", 120*time.Second)
	handler := NewCallbackHandler(apps, hermesClient, 120*time.Second)

	if handler.apps["app1"] == nil {
		t.Error("handler.apps['app1'] should not be nil")
	}
	if handler.apps["app2"] == nil {
		t.Error("handler.apps['app2'] should not be nil")
	}
	if handler.agentClient != hermesClient {
		t.Error("handler.agentClient should be set to hermesClient")
	}
	if handler.agentTimeout != 120*time.Second {
		t.Errorf("handler.agentTimeout = %v, want 120s", handler.agentTimeout)
	}
}
