package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atompi/changate/internal/model"
	"github.com/atompi/changate/pkg/logger"
	"github.com/atompi/changate/pkg/retry"
)

type chatCompletionsRequest struct {
	Model    string          `json:"model"`
	Messages []model.Message `json:"messages"`
	User     string          `json:"user,omitempty"`
	Stream   bool            `json:"stream"`
}

type chatCompletionsClient struct {
	baseURL        string
	apiPath        string
	timeout        time.Duration
	model          string
	token          string
	user           string
	conversation   string
	maxRetries     int
	retryBaseDelay time.Duration
	httpClient     *http.Client
}

func NewChatCompletionsClient(baseURL, apiPath, model, token, user, conversation string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration) *chatCompletionsClient {
	if apiPath == "" {
		apiPath = "/v1/chat/completions"
	}
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &chatCompletionsClient{
		baseURL:        baseURL,
		apiPath:        apiPath,
		timeout:        timeout,
		model:          model,
		token:          token,
		user:           user,
		conversation:   conversation,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *chatCompletionsClient) ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, c.apiPath)

	reqBody := chatCompletionsRequest{
		Model:    c.model,
		Messages: messages,
		User:     c.user,
		Stream:   false,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var respData model.ChatCompletionsResponse

	err = retry.Do(ctx, retry.Config{
		MaxRetries: c.maxRetries,
		BaseDelay:  c.retryBaseDelay,
		BeforeRetry: func(attempt int, delay time.Duration) {
			logger.Debug("[agent] retry attempt %d after delay %v", attempt, delay)
		},
	}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		logger.Debug("[agent request] url=%s body=%s", url, string(jsonBody))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			logger.Debug("[agent] request failed: %v", err)
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			logger.Debug("[agent] unexpected status code: %v", resp.StatusCode)
			if resp.StatusCode >= 500 {
				return fmt.Errorf("%w: status %d, body: %s", retry.ErrTransient, resp.StatusCode, string(respBody))
			}
			return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
		}

		if err := json.Unmarshal(respBody, &respData); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	rawOutput, _ := json.Marshal(respData.Choices)
	logger.Debug("[agent response] raw_body=%s", string(rawOutput))

	return &respData, nil
}

func (c *chatCompletionsClient) ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error) {
	reqMessage := model.Message{
		Role:    "user",
		Content: contentParts,
	}

	messages := []model.Message{reqMessage}
	return c.ChatCompletions(ctx, messages)
}

func (c *chatCompletionsClient) OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error) {
	return nil, fmt.Errorf("OpenResponses not supported for ChatCompletions client")
}

func (c *chatCompletionsClient) OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error) {
	return nil, fmt.Errorf("OpenResponses not supported for ChatCompletions client")
}

func (c *chatCompletionsClient) GetTimeout() time.Duration {
	return c.timeout
}
