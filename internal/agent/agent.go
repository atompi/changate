// Package agent provides the interface and data types for AI agent clients.
// It defines a common Client interface that both Hermes and OpenClaw implementations satisfy.
package agent

import (
	"context"
	"time"
)

// Message represents a chat message in the agent API.
type Message struct {
	Role    string
	Content interface{}
}

// ChatCompletionResponse represents the response from a chat completion API call.
type ChatCompletionResponse struct {
	ID      string
	Object  string
	Created int64
	Model   string
	Choices []Choice
	Usage   Usage
}

// Choice represents a single choice in a chat completion response.
type Choice struct {
	Index        int
	Message      Message
	FinishReason string
}

// Usage represents token usage statistics from the API.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ContentPart represents a content block in a multimodal message.
// It can be either text or image_url type.
type ContentPart struct {
	Type     string
	Text     string
	ImageURL *ImageURL
}

// ImageURL represents an image URL with optional detail setting.
type ImageURL struct {
	URL    string
	Detail string
}

// Client is the interface for interacting with AI agent APIs.
// Implementations must provide chat completion functionality.
type Client interface {
	ChatCompletion(ctx context.Context, messages []Message) (*ChatCompletionResponse, error)
	ChatCompletionWithContent(ctx context.Context, userContent interface{}) (*ChatCompletionResponse, error)
	GetTimeout() time.Duration
}

// GetContent extracts plain text content from a ChatCompletionResponse.
// It handles both simple string content and content parts arrays.
func GetContent(r *ChatCompletionResponse) string {
	if len(r.Choices) == 0 {
		return ""
	}
	switch content := r.Choices[0].Message.Content.(type) {
	case string:
		return content
	case []ContentPart:
		var text string
		for _, part := range content {
			text += part.Text
		}
		return text
	}
	return ""
}

// ClientOption is a functional option for configuring AgentConfig.
type ClientOption func(AgentConfig) AgentConfig

// AgentConfig holds configuration for connecting to an agent API server.
type AgentConfig struct {
	BaseURL  string
	APIPath  string
	Timeout  time.Duration
	Model    string
	Token    string
	Platform string
	User     string
}

// WithPlatform sets the platform type (e.g., "hermes" or "openclaw").
func WithPlatform(platform string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.Platform = platform
		return cfg
	}
}

// WithBaseURL sets the agent API server base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.BaseURL = baseURL
		return cfg
	}
}

// WithAPIPath sets the API path for chat completions.
func WithAPIPath(apiPath string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.APIPath = apiPath
		return cfg
	}
}

// WithTimeout sets the request timeout duration.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.Timeout = timeout
		return cfg
	}
}

// WithModel sets the model name to use.
func WithModel(model string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.Model = model
		return cfg
	}
}

// WithToken sets the authentication token.
func WithToken(token string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.Token = token
		return cfg
	}
}

// WithUser sets the user identifier for session persistence.
func WithUser(user string) ClientOption {
	return func(cfg AgentConfig) AgentConfig {
		cfg.User = user
		return cfg
	}
}