package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
)

func TestNewClient_RequiresBaseURL(t *testing.T) {
	_, err := NewClient(Config{Model: "m"})
	if err == nil {
		t.Fatal("expected error for empty BaseURL, got nil")
	}
	if !strings.Contains(err.Error(), "BaseURL") {
		t.Errorf("expected error to mention BaseURL, got: %v", err)
	}
}

func TestNewClient_AppliesDefaults(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "http://example", Model: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc := c.(*agentHTTPClient)
	if got := hc.httpClient.Timeout; got != defaultHTTPTimeout {
		t.Errorf("Timeout default: got %v, want %v", got, defaultHTTPTimeout)
	}
	if hc.maxRetries != 3 {
		t.Errorf("MaxRetries default: got %d, want 3", hc.maxRetries)
	}
	if hc.retryBaseDelay != 100*time.Millisecond {
		t.Errorf("RetryBaseDelay default: got %v, want 100ms", hc.retryBaseDelay)
	}
}

func TestNewClient_RejectsUnknownType(t *testing.T) {
	_, err := NewClient(Config{BaseURL: "http://example", Model: "m", AgentType: "Bogus"})
	if err == nil {
		t.Fatal("expected error for unknown AgentType, got nil")
	}
}

func TestNewClient_DispatchesChatCompletions(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "http://example", Model: "m", AgentType: TypeChatCompletions})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc, ok := c.(*agentHTTPClient)
	if !ok {
		t.Fatalf("expected *agentHTTPClient, got %T", c)
	}
	if hc.chatBld == nil {
		t.Error("chatBld should be set for ChatCompletions")
	}
	if hc.responsesBld != nil {
		t.Error("responsesBld should NOT be set for ChatCompletions")
	}
	if hc.apiPath != defaultAPIPathChatCompletions {
		t.Errorf("apiPath: got %q, want %q", hc.apiPath, defaultAPIPathChatCompletions)
	}
	if hc.apiName != "chatcompletions" {
		t.Errorf("apiName: got %q, want chatcompletions", hc.apiName)
	}
}

func TestNewClient_DispatchesOpenResponses(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "http://example", Model: "m", AgentType: TypeOpenResponses})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc := c.(*agentHTTPClient)
	if hc.responsesBld == nil {
		t.Error("responsesBld should be set for OpenResponses")
	}
	if hc.chatBld != nil {
		t.Error("chatBld should NOT be set for OpenResponses")
	}
	if hc.apiPath != defaultAPIPathResponses {
		t.Errorf("apiPath: got %q, want %q", hc.apiPath, defaultAPIPathResponses)
	}
	if hc.apiName != "responses" {
		t.Errorf("apiName: got %q, want responses", hc.apiName)
	}
}

func TestNewClient_DefaultAgentTypeIsOpenResponses(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "http://example", Model: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc := c.(*agentHTTPClient)
	if hc.responsesBld == nil {
		t.Error("responsesBld should be set when AgentType is empty (default)")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"first wins", []string{"a", "b"}, "a"},
		{"skips empty", []string{"", "b", "c"}, "b"},
		{"all empty", []string{"", ""}, ""},
		{"empty", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstNonEmpty(tc.in...); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadBoundedResponse(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		body, err := readBoundedResponse(strings.NewReader("hello"), 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != "hello" {
			t.Errorf("body: got %q, want %q", string(body), "hello")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		_, err := readBoundedResponse(strings.NewReader("hello world"), 5)
		if err == nil {
			t.Fatal("expected error for body exceeding limit, got nil")
		}
	})

	t.Run("at limit", func(t *testing.T) {
		body, err := readBoundedResponse(strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != "hello" {
			t.Errorf("body: got %q, want %q", string(body), "hello")
		}
	})
}

func TestTruncateForLog(t *testing.T) {
	t.Run("small body unchanged", func(t *testing.T) {
		got := truncateForLog([]byte("short"))
		if got != "short" {
			t.Errorf("got %q, want %q", got, "short")
		}
	})

	t.Run("large body truncated", func(t *testing.T) {
		big := strings.Repeat("a", logBodyByteLimit+100)
		got := truncateForLog([]byte(big))
		if !strings.HasPrefix(got, strings.Repeat("a", logBodyByteLimit)) {
			t.Errorf("expected %d leading 'a's, got prefix len %d", logBodyByteLimit, len(got))
		}
		if !strings.Contains(got, "truncated") {
			t.Errorf("expected 'truncated' marker, got %q", got)
		}
	})
}

func TestRawString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"nil map", nil, "x", ""},
		{"missing key", map[string]any{}, "x", ""},
		{"string value", map[string]any{"x": "y"}, "x", "y"},
		{"wrong type", map[string]any{"x": 42}, "x", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rawString(tc.m, tc.key); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRawInt(t *testing.T) {
	if got := rawInt(map[string]any{"n": float64(42)}, "n"); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if got := rawInt(nil, "n"); got != 0 {
		t.Errorf("nil map: got %d, want 0", got)
	}
}

func TestRawInt64(t *testing.T) {
	if got := rawInt64(map[string]any{"n": float64(1234567890)}, "n"); got != 1234567890 {
		t.Errorf("got %d, want 1234567890", got)
	}
	if got := rawInt64(nil, "n"); got != 0 {
		t.Errorf("nil map: got %d, want 0", got)
	}
}

func TestRawMap(t *testing.T) {
	want := map[string]any{"a": "b"}
	if got := rawMap(map[string]any{"m": want}, "m"); got == nil || got["a"] != "b" {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := rawMap(nil, "m"); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestResponsesBuilder_ParseResponse(t *testing.T) {
	body := []byte(`{
		"id": "resp_123",
		"status": "completed",
		"created_at": 1700000000,
		"model": "m",
		"output": [
			{"type": "message", "role": "assistant", "content": [
				{"type": "output_text", "text": "Hello!"}
			]},
			{"type": "function_call", "name": "foo"},
			{"type": "reasoning", "summary": "thinking..."}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}
	}`)

	r, err := (&responsesBuilder{}).parseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "resp_123" {
		t.Errorf("ID: got %q, want %q", r.ID, "resp_123")
	}
	if r.Status != "completed" {
		t.Errorf("Status: got %q, want %q", r.Status, "completed")
	}
	if r.CreatedAt != 1700000000 {
		t.Errorf("CreatedAt: got %d, want %d", r.CreatedAt, 1700000000)
	}
	if len(r.Output) != 1 {
		t.Fatalf("Output length: got %d, want 1 (function_call and reasoning should be dropped)", len(r.Output))
	}
	if got := r.GetContent(); got != "Hello!" {
		t.Errorf("GetContent: got %q, want %q", got, "Hello!")
	}
	if r.Usage.InputTokens != 10 || r.Usage.OutputTokens != 5 || r.Usage.TotalTokens != 15 {
		t.Errorf("Usage: got %+v", r.Usage)
	}
}

func TestResponsesBuilder_ParseResponse_Malformed(t *testing.T) {
	if _, err := (&responsesBuilder{}).parseResponse([]byte("not json")); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestChatCompletionsBuilder_ParseResponse(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "m",
		"choices": [
			{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "Hi"}}
		],
		"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
	}`)

	r, err := (&chatCompletionsBuilder{}).parseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "chatcmpl-123" {
		t.Errorf("ID: got %q, want %q", r.ID, "chatcmpl-123")
	}
	if len(r.Choices) != 1 {
		t.Fatalf("Choices length: got %d, want 1", len(r.Choices))
	}
	if got := r.GetContent(); got != "\nHi" {
		t.Errorf("GetContent: got %q, want %q", got, "\nHi")
	}
	if r.Usage.InputTokens != 3 || r.Usage.OutputTokens != 2 || r.Usage.TotalTokens != 5 {
		t.Errorf("Usage: got %+v", r.Usage)
	}
}

func TestResponsesBuilder_BuildRequest_StringContent(t *testing.T) {
	c := &agentHTTPClient{model: "m", user: "u", systemPrompt: "be brief"}
	body, err := (&responsesBuilder{}).buildRequest(c, []model.Message{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"system"`) {
		t.Errorf("expected system message, got: %s", string(body))
	}
	if !strings.Contains(string(body), `"input_text"`) {
		t.Errorf("expected input_text type, got: %s", string(body))
	}
}

func TestResponsesBuilder_BuildRequest_RejectsUnknownContent(t *testing.T) {
	c := &agentHTTPClient{model: "m"}
	_, err := (&responsesBuilder{}).buildRequest(c, []model.Message{
		{Role: "user", Content: []model.ChatCompletionsContentPart{}},
	})
	if err == nil {
		t.Fatal("expected error for unsupported content type, got nil")
	}
}

func TestResponsesBuilder_BuildRequest_IncludesToolChoice(t *testing.T) {
	c := &agentHTTPClient{model: "m", toolChoice: "required"}
	body, err := (&responsesBuilder{}).buildRequest(c, []model.Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"tool_choice":"required"`) {
		t.Errorf("expected tool_choice in body, got: %s", string(body))
	}
}

func TestConvertContentPartsToInput_DropsUnknown(t *testing.T) {
	parts := []model.OpenResponsesContentPart{
		{Type: "input_text", Text: "hi"},
		{Type: "input_audio"},
		{Type: "input_image", ImageData: "data:..."},
	}
	out := (&responsesBuilder{}).convertContentPartsToInput(parts)
	if len(out) != 2 {
		t.Errorf("got %d parts, want 2 (text + image, audio dropped)", len(out))
	}
}

func TestExecuteWithRetry_RetriesOn5xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL:    srv.URL,
		Model:      "m",
		AgentType:  TypeOpenResponses,
		MaxRetries: 3,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := c.OpenResponsesWithContent(context.Background(), []model.OpenResponsesContentPart{{Type: "input_text", Text: "hi"}})
	if err != nil {
		t.Fatalf("OpenResponses: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if resp.GetContent() != "ok" {
		t.Errorf("content: got %q, want %q", resp.GetContent(), "ok")
	}
}

func TestExecuteWithRetry_GivesUpAfterMaxRetries(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL:        srv.URL,
		Model:          "m",
		AgentType:      TypeOpenResponses,
		MaxRetries:     2,
		RetryBaseDelay: 1 * time.Millisecond,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.OpenResponsesWithContent(context.Background(), []model.OpenResponsesContentPart{{Type: "input_text", Text: "hi"}})
	if err == nil {
		t.Fatal("expected error after max retries exhausted, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", attempts)
	}
}

func TestExecuteWithRetry_DoesNotRetryOn4xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL:        srv.URL,
		Model:          "m",
		AgentType:      TypeOpenResponses,
		MaxRetries:     3,
		RetryBaseDelay: 1 * time.Millisecond,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.OpenResponsesWithContent(context.Background(), []model.OpenResponsesContentPart{{Type: "input_text", Text: "hi"}})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}

func TestReadBoundedResponse_RejectsOversized(t *testing.T) {
	big := strings.Repeat("x", maxResponseBodyBytes+1)
	_, err := readBoundedResponse(strings.NewReader(big), maxResponseBodyBytes)
	if err == nil {
		t.Fatal("expected error for body exceeding maxResponseBodyBytes, got nil")
	}
}

func TestBuildRequest_NilTools(t *testing.T) {
	c := &agentHTTPClient{model: "m", tools: nil}
	body, err := (&responsesBuilder{}).buildRequest(c, []model.Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"model":"m"`) {
		t.Errorf("missing model in body: %s", string(body))
	}
}

func TestBuildRequest_Tools(t *testing.T) {
	c := &agentHTTPClient{
		model: "m",
		tools: []config.MCPConfig{
			{Type: "mcp", ServerURL: "u", ServerLabel: "l", RequireApproval: "never"},
		},
	}
	body, err := (&responsesBuilder{}).buildRequest(c, []model.Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"server_url":"u"`) {
		t.Errorf("tools not serialized: %s", string(body))
	}
}
