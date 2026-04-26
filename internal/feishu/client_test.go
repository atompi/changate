package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

		var req ReplyMessageReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.MsgType != "text" {
			t.Errorf("MsgType = %q, want %q", req.MsgType, "text")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ReplyMessageResp{
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
		json.NewEncoder(w).Encode(ReplyMessageResp{
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
		var req ReplyMessageReq
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
		json.NewEncoder(w).Encode(ReplyMessageResp{Code: 0, Msg: "success"})
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

		var req GetAccessTokenReq
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
		json.NewEncoder(w).Encode(GetAccessTokenResp{
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
		json.NewEncoder(w).Encode(GetAccessTokenResp{
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