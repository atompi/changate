// Package openclaw provides an implementation of the agent.Client interface
// for connecting to OpenClaw Gateway servers.
package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/pkg/logger"
)

// Client is an agent.Client implementation for OpenClaw Gateway.
type Client struct {
	baseURL string
	apiPath string
	timeout time.Duration
	model   string
	token   string
	user    string
	client  *http.Client
}

// NewClient creates a new OpenClaw API client.
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
	Model    string                `json:"model"`
	Messages []chatMessageRequest  `json:"messages"`
	User     string               `json:"user,omitempty"`
	Stream   bool                  `json:"stream"`
}

type chatMessageRequest struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatCompletion sends a chat completion request to the OpenClaw Gateway.
func (c *Client) ChatCompletion(ctx context.Context, messages []agent.Message) (*agent.ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, c.apiPath)

	openclawMessages := make([]chatMessageRequest, len(messages))
	for i, m := range messages {
		openclawMessages[i] = chatMessageRequest{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	reqBody := chatCompletionRequest{
		Model:    c.model,
		Messages: openclawMessages,
		User:     c.user,
		Stream:   false,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debug("[openclaw request] url=%s body=%s", url, string(jsonBody))

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
	logger.Debug("[openclaw response] status=%d body=%s", resp.StatusCode, string(respBody))
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

// ChatCompletionWithContent sends a simple user message to the OpenClaw Gateway.
func (c *Client) ChatCompletionWithContent(ctx context.Context, userContent interface{}) (*agent.ChatCompletionResponse, error) {
	messages := []agent.Message{
		{
			Role:    "user",
			Content: userContent,
		},
	}
	return c.ChatCompletion(ctx, messages)
}

// GetTimeout returns the configured request timeout.
func (c *Client) GetTimeout() time.Duration {
	return c.timeout
}