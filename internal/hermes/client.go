// Package hermes provides an implementation of the agent.Client interface
// for connecting to Hermes Agent API servers.
package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/model"
	"github.com/atompi/changate/pkg/logger"
)

// Client is an agent.Client implementation for Hermes API servers.
type Client struct {
	baseURL string
	apiPath string
	timeout time.Duration
	model   string
	token   string
	user    string
	client  *http.Client
}

// NewClient creates a new Hermes API client.
func NewClient(baseURL, apiPath, model, token, user string, timeout time.Duration) *Client {
	if apiPath == "" {
		apiPath = "/v1/chat/completions"
	}
	return &Client{
		baseURL: baseURL,
		apiPath: apiPath,
		timeout: timeout,
		model:   model,
		token:   token,
		user:    user,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type chatCompletionRequest struct {
	Model    string              `json:"model"`
	Messages []chatMessageRequest `json:"messages"`
	User     string             `json:"user,omitempty"`
	Stream   bool                `json:"stream"`
}

type chatMessageRequest struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatCompletion sends a chat completion request to the Hermes API.
func (c *Client) ChatCompletion(ctx context.Context, messages []agent.Message) (*agent.ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, c.apiPath)

	hermesMessages := make([]chatMessageRequest, len(messages))
	for i, m := range messages {
		hermesMessages[i] = chatMessageRequest{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	reqBody := chatCompletionRequest{
		Model:    c.model,
		Messages: hermesMessages,
		User:     c.user,
		Stream:   false,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debug("[hermes request] url=%s body=%s", url, string(jsonBody))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("[hermes response] status=%d body=%s", resp.StatusCode, string(respBody))
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var chatResp agent.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionWithContent sends a simple user message to the Hermes API.
func (c *Client) ChatCompletionWithContent(ctx context.Context, userContent interface{}) (*agent.ChatCompletionResponse, error) {
	content := convertContent(userContent)
	messages := []agent.Message{
		{
			Role:    "user",
			Content: content,
		},
	}
	return c.ChatCompletion(ctx, messages)
}

func convertContent(userContent interface{}) string {
	if contentStr, ok := userContent.(string); ok {
		return contentStr
	}
	if contentParts, ok := userContent.([]model.ContentPart); ok {
		var text string
		for _, part := range contentParts {
			text += part.Text
		}
		return text
	}
	return ""
}

// GetTimeout returns the configured request timeout.
func (c *Client) GetTimeout() time.Duration {
	return c.timeout
}