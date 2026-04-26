package openclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atompi/changate/internal/agent"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://example.com", "/v1/chat", "test-model", "test-token", "", 30*time.Second)

	if client.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://example.com")
	}
	if client.apiPath != "/v1/chat" {
		t.Errorf("apiPath = %q, want %q", client.apiPath, "/v1/chat")
	}
	if client.model != "test-model" {
		t.Errorf("model = %q, want %q", client.model, "test-model")
	}
	if client.token != "test-token" {
		t.Errorf("token = %q, want %q", client.token, "test-token")
	}
	if client.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want %v", client.timeout, 30*time.Second)
	}
}

func TestNewClient_DefaultAPIPath(t *testing.T) {
	client := NewClient("https://example.com", "", "test-model", "test-token", "", 30*time.Second)

	if client.apiPath != "/v1/chat/completions" {
		t.Errorf("apiPath = %q, want %q", client.apiPath, "/v1/chat/completions")
	}
}

func TestChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test-token")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(agent.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "openclaw/default",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.Message{
						Role:    "assistant",
						Content: "Hello, World!",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat", "openclaw/default", "test-token", "", 30*time.Second)
	resp, err := client.ChatCompletion(context.Background(), []agent.Message{
		{Role: "user", Content: "Hello"},
	})

	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.ID != "chatcmpl-123" {
		t.Errorf("resp.ID = %q, want %q", resp.ID, "chatcmpl-123")
	}
	if resp.Choices[0].Message.Content != "Hello, World!" {
		t.Errorf("resp.Choices[0].Message.Content = %q, want %q", resp.Choices[0].Message.Content, "Hello, World!")
	}
}

func TestChatCompletion_RequestError(t *testing.T) {
	client := NewClient("http://localhost:99999", "/v1/chat", "openclaw/default", "test-token", "", 1*time.Millisecond)
	_, err := client.ChatCompletion(context.Background(), []agent.Message{
		{Role: "user", Content: "Hello"},
	})

	if err == nil {
		t.Error("ChatCompletion() should return error for connection failure")
	}
}

func TestChatCompletionWithContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(agent.ChatCompletionResponse{
			ID:      "chatcmpl-456",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "openclaw/research",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.Message{
						Role:    "assistant",
						Content: "Response content",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat", "openclaw/research", "test-token", "", 30*time.Second)
	resp, err := client.ChatCompletionWithContent(context.Background(), "user message")

	if err != nil {
		t.Fatalf("ChatCompletionWithContent() error = %v", err)
	}
	if resp.ID != "chatcmpl-456" {
		t.Errorf("resp.ID = %q, want %q", resp.ID, "chatcmpl-456")
	}
}

func TestClient_ImplementsAgentClient(t *testing.T) {
	var _ agent.Client = (*Client)(nil)
}
