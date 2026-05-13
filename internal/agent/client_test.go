package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atompi/changate/internal/model"
)

func TestNewClient_OpenResponses(t *testing.T) {
	client := NewClient("http://example.com", "", "test-model", "test-token", "test-user", "", 0, 3, 100*time.Millisecond, "OpenResponses")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if timeout := client.GetTimeout(); timeout != 120*time.Second {
		t.Errorf("default timeout = %v, want 120s", timeout)
	}
}

func TestNewClient_ChatCompletions(t *testing.T) {
	client := NewClient("http://example.com", "", "test-model", "test-token", "test-user", "", 0, 3, 100*time.Millisecond, "ChatCompletions")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if timeout := client.GetTimeout(); timeout != 120*time.Second {
		t.Errorf("default timeout = %v, want 120s", timeout)
	}
}

func TestNewClient_UnknownType(t *testing.T) {
	client := NewClient("http://example.com", "", "test-model", "test-token", "test-user", "", 0, 3, 100*time.Millisecond, "Unknown")

	if client == nil {
		t.Fatal("NewClient returned nil for unknown type")
	}

	if timeout := client.GetTimeout(); timeout != 120*time.Second {
		t.Errorf("default timeout = %v, want 120s", timeout)
	}
}

func TestNewClient_CustomTimeout(t *testing.T) {
	customTimeout := 60 * time.Second
	client := NewClient("http://example.com", "", "test-model", "test-token", "test-user", "", customTimeout, 3, 100*time.Millisecond, "OpenResponses")

	if timeout := client.GetTimeout(); timeout != customTimeout {
		t.Errorf("timeout = %v, want %v", timeout, customTimeout)
	}
}

func TestOpenResponsesClient_ChatCompletionsNotSupported(t *testing.T) {
	client := NewClient("http://example.com", "", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	_, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("ChatCompletions should return error for OpenResponses client")
	}

	_, err = client.ChatCompletionsWithContent(context.Background(), []model.ChatCompletionsContentPart{{Type: "text", Text: "test"}})
	if err == nil {
		t.Error("ChatCompletionsWithContent should return error for OpenResponses client")
	}
}

func TestChatCompletionsClient_OpenResponsesNotSupported(t *testing.T) {
	client := NewClient("http://example.com", "", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	_, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("OpenResponses should return error for ChatCompletions client")
	}

	_, err = client.OpenResponsesWithContent(context.Background(), []model.OpenResponsesContentPart{{Type: "text", Text: "test"}})
	if err == nil {
		t.Error("OpenResponsesWithContent should return error for ChatCompletions client")
	}
}

func TestOpenResponses_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}

		var req struct {
			Model string `json:"model"`
			Input []struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			} `json:"input"`
			User   string `json:"user"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("req.Model = %q, want %q", req.Model, "test-model")
		}

		if len(req.Input) != 1 {
			t.Errorf("len(req.Input) = %d, want 1", len(req.Input))
		}

		if req.User != "test-user" {
			t.Errorf("req.User = %q, want %q", req.User, "test-user")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_123",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]interface{}{{"type": "output_text", "text": "hello"}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "test-model", "test-token", "test-user", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	messages := []model.Message{{Role: "user", Content: "test message"}}
	resp, err := client.OpenResponses(context.Background(), messages)
	if err != nil {
		t.Fatalf("OpenResponses() error = %v", err)
	}

	if resp.Output[0].Content[0].Text != "hello" {
		t.Errorf("content = %q, want %q", resp.Output[0].Content[0].Text, "hello")
	}
}

func TestOpenResponses_ConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	_, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestOpenResponses_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	_, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestOpenResponses_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json{"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	_, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestOpenResponsesWithContent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model  string                   `json:"model"`
			Input  []map[string]interface{} `json:"input"`
			User   string                   `json:"user"`
			Stream bool                     `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(req.Input) != 1 {
			t.Fatalf("len(req.Input) = %d, want 1", len(req.Input))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_456",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]interface{}{{"type": "output_text", "text": "response content"}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "test-model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	contentParts := []model.OpenResponsesContentPart{{Type: "input_text", Text: "user message content"}}
	resp, err := client.OpenResponsesWithContent(context.Background(), contentParts)
	if err != nil {
		t.Fatalf("OpenResponsesWithContent() error = %v", err)
	}

	if resp.Output[0].Content[0].Text != "response content" {
		t.Errorf("resp.Output[0].Content[0].Text = %q, want %q", resp.Output[0].Content[0].Text, "response content")
	}
}

func TestChatCompletions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}

		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			} `json:"messages"`
			User   string `json:"user"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("req.Model = %q, want %q", req.Model, "test-model")
		}

		if len(req.Messages) != 1 {
			t.Errorf("len(req.Messages) = %d, want 1", len(req.Messages))
		}

		if req.User != "test-user" {
			t.Errorf("req.User = %q, want %q", req.User, "test-user")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1710000000,
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "hello",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "test-model", "test-token", "test-user", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	messages := []model.Message{{Role: "user", Content: "test message"}}
	resp, err := client.ChatCompletions(context.Background(), messages)
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}

	if strings.TrimSpace(resp.GetContent()) != "hello" {
		t.Errorf("content = %q, want %q", resp.GetContent(), "hello")
	}
}

func TestChatCompletions_ConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	_, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestChatCompletions_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	_, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestChatCompletions_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json{"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	_, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestChatCompletionsWithContent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string                   `json:"model"`
			Messages []map[string]interface{} `json:"messages"`
			User     string                   `json:"user"`
			Stream   bool                     `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(req.Messages) != 1 {
			t.Fatalf("len(req.Messages) = %d, want 1", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-456",
			"object":  "chat.completion",
			"created": 1710000000,
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "response content",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "test-model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	contentParts := []model.ChatCompletionsContentPart{{Type: "text", Text: "user message content"}}
	resp, err := client.ChatCompletionsWithContent(context.Background(), contentParts)
	if err != nil {
		t.Fatalf("ChatCompletionsWithContent() error = %v", err)
	}

	if strings.TrimSpace(resp.GetContent()) != "response content" {
		t.Errorf("resp.GetContent() = %q, want %q", resp.GetContent(), "response content")
	}
}

func TestOpenResponses_GetContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_123",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]interface{}{{"type": "output_text", "text": "test response"}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	resp, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("OpenResponses() error = %v", err)
	}

	content := resp.GetContent()
	if content != "test response" {
		t.Errorf("GetContent() = %q, want %q", content, "test response")
	}
}

func TestOpenResponses_GetFilePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_123",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]interface{}{{"type": "output_text", "text": "MEDIA:/path/to/file.png"}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "OpenResponses")

	resp, err := client.OpenResponses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("OpenResponses() error = %v", err)
	}

	path := resp.GetFilePath()
	if path != "/path/to/file.png" {
		t.Errorf("GetFilePath() = %q, want %q", path, "/path/to/file.png")
	}
}

func TestChatCompletions_GetContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1710000000,
			"model":   "model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "test response",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	resp, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}

	content := strings.TrimSpace(resp.GetContent())
	if content != "test response" {
		t.Errorf("GetContent() = %q, want %q", content, "test response")
	}
}

func TestChatCompletions_GetFilePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1710000000,
			"model":   "model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "MEDIA:/path/to/chatfile.png",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/chat/completions", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond, "ChatCompletions")

	resp, err := client.ChatCompletions(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}

	path := resp.GetFilePath()
	if path != "/path/to/chatfile.png" {
		t.Errorf("GetFilePath() = %q, want %q", path, "/path/to/chatfile.png")
	}
}
