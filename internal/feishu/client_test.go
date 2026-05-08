package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("app_id", "app_secret", "")

	if client.appID != "app_id" {
		t.Errorf("appID = %q, want %q", client.appID, "app_id")
	}
	if client.appSecret != "app_secret" {
		t.Errorf("appSecret = %q, want %q", client.appSecret, "app_secret")
	}
	if client.baseURL != "https://open.feishu.cn" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://open.feishu.cn")
	}
}

func TestReplyMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test_token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test_token")
		}

		var req replyMessageReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.MsgType != "text" {
			t.Errorf("MsgType = %q, want %q", req.MsgType, "text")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(replyMessageResp{
			Code: 0,
			Msg:  "success",
		})
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	err := client.ReplyMessage(context.Background(), "test_token", "msg_id_123", "text", `{"text":"Hello"}`)
	if err != nil {
		t.Fatalf("ReplyMessage() error = %v", err)
	}
}

func TestReplyMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(replyMessageResp{
			Code: 10003,
			Msg:  "invalid parameter",
		})
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	err := client.ReplyMessage(context.Background(), "test_token", "msg_id_123", "text", `{"text":"Hello"}`)
	if err == nil {
		t.Error("ReplyMessage() should return error when API returns error code")
	}
}

func TestSendTextMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req replyMessageReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.MsgType != "text" {
			t.Errorf("MsgType = %q, want %q", req.MsgType, "text")
		}

		var content map[string]string
		if err := json.Unmarshal([]byte(req.Content), &content); err != nil {
			t.Fatalf("failed to unmarshal content: %v", err)
		}
		if content["text"] != "Hello, World!" {
			t.Errorf("content text = %q, want %q", content["text"], "Hello, World!")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(replyMessageResp{Code: 0, Msg: "success"})
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	err := client.SendTextMessage(context.Background(), "test_token", "msg_id_123", "Hello, World!")
	if err != nil {
		t.Fatalf("SendTextMessage() error = %v", err)
	}
}

func TestGetAppAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}

		var req getAccessTokenReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.AppID != "test_app_id" {
			t.Errorf("AppID = %q, want %q", req.AppID, "test_app_id")
		}
		if req.AppSecret != "test_app_secret" {
			t.Errorf("AppSecret = %q, want %q", req.AppSecret, "test_app_secret")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(getAccessTokenResp{
			Code:           0,
			Msg:            "success",
			AppAccessToken: "test_access_token_12345",
			Expire:         7200,
		})
	}))
	defer server.Close()

	client := NewClient("test_app_id", "test_app_secret", server.URL)

	token, err := client.GetAppAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAppAccessToken() error = %v", err)
	}
	if token != "test_access_token_12345" {
		t.Errorf("token = %q, want %q", token, "test_access_token_12345")
	}
}

func TestGetAppAccessToken_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(getAccessTokenResp{
			Code: 99999,
			Msg:  "internal error",
		})
	}))
	defer server.Close()

	client := NewClient("test_app_id", "test_app_secret", server.URL)

	_, err := client.GetAppAccessToken(context.Background())
	if err == nil {
		t.Error("GetAppAccessToken() should return error when API returns error code")
	}
}

func TestUploadImage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test_token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test_token")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,"msg":"success","data":{"file_key":"file_xxxx"}}`))
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	imageKey, err := client.UploadMessageResource(context.Background(), "test_token", imageData, "test.png", "stream")
	if err != nil {
		t.Fatalf("UploadMessageResource() error = %v", err)
	}
	if imageKey != "file_xxxx" {
		t.Errorf("imageKey = %q, want %q", imageKey, "file_xxxx")
	}
}

func TestUploadImage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":10003,"msg":"invalid parameter"}`))
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	_, err := client.UploadMessageResource(context.Background(), "test_token", []byte("fake image data"), "test.png", "stream")
	if err == nil {
		t.Error("UploadMessageResource() should return error when API returns error code")
	}
}

func TestUploadImage_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	_, err := client.UploadMessageResource(context.Background(), "test_token", []byte("image"), "test.png", "stream")
	if err == nil {
		t.Error("UploadMessageResource() should return error for non-200 status")
	}
}

func TestSendFileMessage_Success(t *testing.T) {
	var uploadCalled bool
	var replyCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"test_tenant_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/im/v1/files" {
			uploadCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","data":{"file_key":"file_xxxx"}}`))
		} else if r.URL.Path == "/open-apis/im/v1/messages/msg123/reply" {
			replyCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success"}`))
		}
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	tmpFile, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte("fake image data"))
	tmpFile.Close()

	err = client.SendFileMessage(context.Background(), "test_token", "msg123", tmpFile.Name())
	if err != nil {
		t.Fatalf("SendFileMessage() error = %v", err)
	}
	if !uploadCalled {
		t.Error("UploadMessageResource was not called")
	}
	if !replyCalled {
		t.Error("ReplyMessage was not called")
	}
}

func TestSendFileMessage_UploadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"test_tenant_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/im/v1/files" {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	tmpFile, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte("fake image data"))
	tmpFile.Close()

	err = client.SendFileMessage(context.Background(), "test_token", "msg123", tmpFile.Name())
	if err == nil {
		t.Error("SendFileMessage() should return error when upload fails")
	}
}

func TestSendFileMessage_FileReadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"test_tenant_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/im/v1/files" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","data":{"file_key":"file_xxxx"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	err := client.SendFileMessage(context.Background(), "test_token", "msg123", "/nonexistent/path/file.png")
	if err == nil {
		t.Error("SendFileMessage() should return error when file read fails")
	}
}

func TestSendFileMessage_ReplyFailure(t *testing.T) {
	uploaded := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"test_tenant_token","expire":7200}`))
		} else if r.URL.Path == "/open-apis/im/v1/files" {
			uploaded = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":0,"msg":"success","data":{"file_key":"file_xxxx"}}`))
		} else if r.URL.Path == "/open-apis/im/v1/messages/msg123/reply" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":10003,"msg":"invalid parameter"}`))
		}
	}))
	defer server.Close()

	client := NewClient("app_id", "app_secret", server.URL)

	tmpFile, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte("fake image data"))
	tmpFile.Close()

	err = client.SendFileMessage(context.Background(), "test_token", "msg123", tmpFile.Name())
	if err == nil {
		t.Error("SendFileMessage() should return error when reply fails")
	}
	if !uploaded {
		t.Error("UploadMessageResource should have been called before reply failure")
	}
}
