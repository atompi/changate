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

type responsesRequest struct {
	Model        string          `json:"model"`
	Input        []model.Message `json:"input"`
	User         string          `json:"user,omitempty"`
	Conversation string          `json:"conversation,omitempty"`
	Stream       bool            `json:"stream"`
}

type responsesClient struct {
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

func NewClient(baseURL, apiPath, model, token, user, conversation string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration) Client {
	if apiPath == "" {
		apiPath = "/v1/responses"
	}
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &responsesClient{
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

func (c *responsesClient) Responses(ctx context.Context, messages []model.Message) (*model.ResponsesResponse, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, c.apiPath)

	reqBody := responsesRequest{
		Model:        c.model,
		Input:        messages,
		User:         c.user,
		Conversation: c.conversation,
		Stream:       false,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var respData model.ResponsesResponse

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

	rawOutput, _ := json.Marshal(respData.Output)
	logger.Debug("[agent response] raw_body=%s", string(rawOutput))

	return &respData, nil
}

func (c *responsesClient) ResponsesWithContent(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error) {
	reqInput := model.Message{
		Role:    "user",
		Content: contentParts,
	}

	messages := []model.Message{reqInput}
	return c.Responses(ctx, messages)
}

func (c *responsesClient) GetTimeout() time.Duration {
	return c.timeout
}
