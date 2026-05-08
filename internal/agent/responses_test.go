package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atompi/changate/internal/model"
)

func TestNewClient_Defaults(t *testing.T) {
	client := NewClient("http://example.com", "", "test-model", "test-token", "test-user", "", 0, 3, 100*time.Millisecond)

	rc, ok := client.(*responsesClient)
	if !ok {
		t.Fatalf("expected *responsesClient, got %T", client)
	}

	if rc.apiPath != "/v1/responses" {
		t.Errorf("apiPath = %q, want %q", rc.apiPath, "/v1/responses")
	}

	if rc.timeout != 120*time.Second {
		t.Errorf("timeout = %v, want %v", rc.timeout, 120*time.Second)
	}

	if rc.model != "test-model" {
		t.Errorf("model = %q, want %q", rc.model, "test-model")
	}

	if rc.token != "test-token" {
		t.Errorf("token = %q, want %q", rc.token, "test-token")
	}

	if rc.user != "test-user" {
		t.Errorf("user = %q, want %q", rc.user, "test-user")
	}
}

func TestNewClient_CustomValues(t *testing.T) {
	customTimeout := 60 * time.Second
	customAPIPath := "/v1/custom"
	client := NewClient("http://example.com", customAPIPath, "custom-model", "custom-token", "custom-user", "", customTimeout, 3, 100*time.Millisecond)

	rc, ok := client.(*responsesClient)
	if !ok {
		t.Fatalf("expected *responsesClient, got %T", client)
	}

	if rc.apiPath != customAPIPath {
		t.Errorf("apiPath = %q, want %q", rc.apiPath, customAPIPath)
	}

	if rc.timeout != customTimeout {
		t.Errorf("timeout = %v, want %v", rc.timeout, customTimeout)
	}
}

func TestResponses_Success(t *testing.T) {
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

		var req responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("req.Model = %q, want %q", req.Model, "test-model")
		}

		if len(req.Input) != 1 || req.Input[0].Role != "user" {
			t.Errorf("req.Input = %v, want single user message", req.Input)
		}

		if req.User != "test-user" {
			t.Errorf("req.User = %q, want %q", req.User, "test-user")
		}

		if req.Stream != false {
			t.Errorf("req.Stream = %v, want false", req.Stream)
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

	client := NewClient(server.URL, "/v1/responses", "test-model", "test-token", "test-user", "", 30*time.Second, 3, 100*time.Millisecond)

	messages := []model.Message{{Role: "user", Content: "test message"}}
	resp, err := client.Responses(context.Background(), messages)
	if err != nil {
		t.Fatalf("Responses() error = %v", err)
	}

	if len(resp.Output) != 1 {
		t.Fatalf("len(resp.Output) = %d, want 1", len(resp.Output))
	}

	if resp.Output[0].Role != "assistant" {
		t.Errorf("resp.Output[0].Role = %q, want %q", resp.Output[0].Role, "assistant")
	}

	if len(resp.Output[0].Content) == 0 {
		t.Fatal("resp.Output[0].Content is empty")
	}
	content := resp.Output[0].Content[0].Text
	if content != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
}

func TestResponsesWithContent_Success(t *testing.T) {
	var receivedReq responsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Fatalf("failed to decode request: %v", err)
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

	client := NewClient(server.URL, "/v1/responses", "test-model", "", "", "", 30*time.Second, 3, 100*time.Millisecond)

	contentParts := []model.ContentPart{
		{Type: "text", Text: "user message content"},
	}
	resp, err := client.ResponsesWithContent(context.Background(), contentParts)
	if err != nil {
		t.Fatalf("ResponsesWithContent() error = %v", err)
	}

	if len(receivedReq.Input) != 1 {
		t.Fatalf("len(receivedReq.Input) = %d, want 1", len(receivedReq.Input))
	}

	if receivedReq.Input[0].Role != "user" {
		t.Errorf("receivedReq.Input[0].Role = %q, want %q", receivedReq.Input[0].Role, "user")
	}

	content, ok := receivedReq.Input[0].Content.([]interface{})
	if !ok {
		t.Fatalf("receivedReq.Input[0].Content = %v, want []interface{}", receivedReq.Input[0].Content)
	}
	if len(content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(content))
	}
	part, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("content[0] = %v, want map[string]interface{}", content[0])
	}
	if part["type"] != "text" {
		t.Errorf("part[\"type\"] = %q, want %q", part["type"], "text")
	}
	if part["text"] != "user message content" {
		t.Errorf("part[\"text\"] = %q, want %q", part["text"], "user message content")
	}

	if resp.Output[0].Content[0].Text != "response content" {
		t.Errorf("resp.Output[0].Content[0].Text = %q, want %q", resp.Output[0].Content[0].Text, "response content")
	}
}

func TestGetTimeout(t *testing.T) {
	customTimeout := 45 * time.Second
	client := NewClient("http://example.com", "", "model", "", "", "", customTimeout, 3, 100*time.Millisecond)

	if timeout := client.GetTimeout(); timeout != customTimeout {
		t.Errorf("GetTimeout() = %v, want %v", timeout, customTimeout)
	}
}

func TestResponses_ConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond)

	_, err := client.Responses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("Responses() expected error for connection failure, got nil")
	}
}

func TestResponses_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond)

	_, err := client.Responses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("Responses() expected error for non-200 status, got nil")
	}
}

func TestResponses_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json{"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond)

	_, err := client.Responses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("Responses() expected error for invalid JSON, got nil")
	}
}

func TestResponses_WithOutputImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_789",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": "Here is the image"},
					},
				},
				{
					"type": "output_image",
					"output_image": map[string]interface{}{
						"url": "https://example.com/generated-image.png",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "test-model", "test-token", "", "", 30*time.Second, 3, 100*time.Millisecond)

	resp, err := client.Responses(context.Background(), []model.Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("Responses() error = %v", err)
	}

	content := resp.GetContent()
	if content != "Here is the image" {
		t.Errorf("GetContent() = %q, want %q", content, "Here is the image")
	}
}

func TestResponses_TextOnlyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "resp_text_only",
			"object": "response",
			"status": "completed",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": "Plain text response"},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/v1/responses", "model", "", "", "", 30*time.Second, 3, 100*time.Millisecond)

	resp, err := client.Responses(context.Background(), []model.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Responses() error = %v", err)
	}

	if resp.GetContent() != "Plain text response" {
		t.Errorf("GetContent() = %q, want %q", resp.GetContent(), "Plain text response")
	}
}