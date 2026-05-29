package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
	"github.com/atompi/changate/pkg/logger"
	"github.com/atompi/changate/pkg/retry"
)

type agentHTTPClient struct {
	baseURL      string
	apiPath      string
	model        string
	token        string
	user         string
	systemPrompt string
	tools        []config.MCPConfig
	httpClient   *http.Client
	builder      requestBuilder
	maxRetries   int
	retryBaseDelay time.Duration
}

type requestBuilder interface {
	defaultAPIPath() string
	buildRequest(client *agentHTTPClient, messages []model.Message) ([]byte, error)
	parseResponse(body []byte) (any, error)
	getAPIType() string
}

func newAgentHTTPClient(baseURL, apiPath, model, token, user, systemPrompt string, tools []config.MCPConfig, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration, builder requestBuilder) *agentHTTPClient {
	if apiPath == "" {
		apiPath = builder.defaultAPIPath()
	}
	if timeout == 0 {
		timeout = 3600 * time.Second
	}
	if maxRetries == 0 {
		maxRetries = 3
	}
	if retryBaseDelay == 0 {
		retryBaseDelay = 100 * time.Millisecond
	}

	return &agentHTTPClient{
		baseURL:      baseURL,
		apiPath:      apiPath,
		model:        model,
		token:        token,
		user:         user,
		systemPrompt: systemPrompt,
		tools:        tools,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		builder:        builder,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
	}
}

func (c *agentHTTPClient) GetTimeout() time.Duration {
	return c.httpClient.Timeout
}

func (c *agentHTTPClient) OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error) {
	reqBody, err := c.builder.buildRequest(c, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	httpResp, err := c.doRequestWithRetry(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("responses HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Debugf("[responses] response body: %s", string(body))

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("responses API error: status=%d, body=%s", httpResp.StatusCode, string(body))
	}

	resp, err := c.builder.parseResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if openResp, ok := resp.(*model.OpenResponsesResponse); ok {
		return openResp, nil
	}
	return nil, fmt.Errorf("unexpected response type: %T", resp)
}

func (c *agentHTTPClient) OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error) {
	messages := []model.Message{{Role: "user", Content: contentParts}}
	return c.OpenResponses(ctx, messages)
}

func (c *agentHTTPClient) ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error) {
	reqBody, err := c.builder.buildRequest(c, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	httpResp, err := c.doRequestWithRetry(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("chat completions HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Debugf("[chatcompletions] response body: %s", string(body))

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat completions API error: status=%d, body=%s", httpResp.StatusCode, string(body))
	}

	resp, err := c.builder.parseResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp, ok := resp.(*model.ChatCompletionsResponse); ok {
		return chatResp, nil
	}
	return nil, fmt.Errorf("unexpected response type: %T", resp)
}

func (c *agentHTTPClient) ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error) {
	reqMessage := model.Message{
		Role:    "user",
		Content: contentParts,
	}
	messages := []model.Message{reqMessage}
	return c.ChatCompletions(ctx, messages)
}

func (c *agentHTTPClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	logger.Debugf("[%s] request body: %s", c.builder.getAPIType(), string(body))

	url := c.baseURL + c.apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

func (c *agentHTTPClient) doRequestWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	var lastResp *http.Response
	err := retry.Do(ctx, retry.Config{
		MaxRetries:  c.maxRetries,
		BaseDelay:   c.retryBaseDelay,
		BeforeRetry: func(attempt int, delay time.Duration) {
			logger.Warnf("[%s] retrying request (attempt %d, backoff %v)", c.builder.getAPIType(), attempt, delay)
		},
	}, func() error {
		resp, err := c.doRequest(ctx, body)
		if err != nil {
			lastResp = nil
			return fmt.Errorf("%w: %v", retry.ErrTransient, err)
		}
		lastResp = resp
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			resp.Body.Close()
			return fmt.Errorf("%w: status %d", retry.ErrTransient, resp.StatusCode)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lastResp, nil
}

type responsesBuilder struct{}

func (b *responsesBuilder) defaultAPIPath() string {
	return "/v1/responses"
}

func (b *responsesBuilder) getAPIType() string {
	return "responses"
}

func (b *responsesBuilder) buildRequest(client *agentHTTPClient, messages []model.Message) ([]byte, error) {
	input := []map[string]any{}

	if client.systemPrompt != "" {
		contentList := []map[string]any{
			{"type": "input_text", "text": client.systemPrompt},
		}
		input = append(input, map[string]any{
			"role":    "system",
			"content": contentList,
		})
	}

	for _, msg := range messages {
		item := b.convertModelMessageToInput(msg)
		if item != nil {
			input = append(input, item)
		}
	}

	reqBody := responsesRequest{
		Model:  client.model,
		Input:  input,
		Stream: false,
		User:   client.user,
		Tools:  client.tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

func (b *responsesBuilder) convertModelMessageToInput(msg model.Message) map[string]any {
	role := msg.Role

	var content any
	switch v := msg.Content.(type) {
	case string:
		content = []map[string]any{
			{"type": "input_text", "text": v},
		}
	case []model.OpenResponsesContentPart:
		content = b.convertContentPartsToInput(v)
	default:
		content = []map[string]any{
			{"type": "input_text", "text": fmt.Sprintf("%v", v)},
		}
	}

	return map[string]any{
		"role":    role,
		"content": content,
	}
}

func (b *responsesBuilder) convertContentPartsToInput(parts []model.OpenResponsesContentPart) []map[string]any {
	result := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		if part.Type == "input_text" {
			result = append(result, map[string]any{
				"type": "input_text",
				"text": part.Text,
			})
		} else if part.Type == "input_image" && part.ImageURL != "" {
			result = append(result, map[string]any{
				"type":      "input_image",
				"image_url": part.ImageURL,
			})
		}
	}
	return result
}

type responsesRequest struct {
	Model  string             `json:"model"`
	Input  []map[string]any   `json:"input"`
	Stream bool               `json:"stream"`
	User   string             `json:"user,omitempty"`
	Tools  []config.MCPConfig `json:"tools,omitempty"`
}

func (b *responsesBuilder) parseResponse(body []byte) (any, error) {
	var rawResp map[string]any
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	modelResp := &model.OpenResponsesResponse{}

	if id, ok := rawResp["id"].(string); ok {
		modelResp.ID = id
	}
	if status, ok := rawResp["status"].(string); ok {
		modelResp.Status = status
	}
	if createdAt, ok := rawResp["created_at"].(float64); ok {
		modelResp.CreatedAt = int64(createdAt)
	}
	if model, ok := rawResp["model"].(string); ok {
		modelResp.Model = model
	}

	if output, ok := rawResp["output"].([]any); ok {
		modelResp.Output = make([]model.Output, 0, len(output))
		for _, item := range output {
			outputItem := b.convertOutputItemToModel(item)
			if outputItem != nil {
				modelResp.Output = append(modelResp.Output, *outputItem)
			}
		}
	}

	if usageData, ok := rawResp["usage"].(map[string]any); ok {
		usage := model.Usage{}
		if inputTokens, ok := usageData["input_tokens"].(float64); ok {
			usage.InputTokens = int(inputTokens)
		}
		if outputTokens, ok := usageData["output_tokens"].(float64); ok {
			usage.OutputTokens = int(outputTokens)
		}
		if totalTokens, ok := usageData["total_tokens"].(float64); ok {
			usage.TotalTokens = int(totalTokens)
		}
		modelResp.Usage = usage
	}

	return modelResp, nil
}

func (b *responsesBuilder) convertOutputItemToModel(item any) *model.Output {
	itemMap, ok := item.(map[string]any)
	if !ok {
		return nil
	}

	itemType, _ := itemMap["type"].(string)
	role, _ := itemMap["role"].(string)

	if itemType != "message" || role != "assistant" {
		return nil
	}

	modelOutput := &model.Output{
		Type: itemType,
		Role: role,
	}

	if content, ok := itemMap["content"].([]any); ok {
		modelOutput.Content = make([]model.Content, 0, len(content))
		for _, c := range content {
			if cMap, ok := c.(map[string]any); ok {
				if cType, ok := cMap["type"].(string); ok && cType == "output_text" {
					if text, ok := cMap["text"].(string); ok {
						modelOutput.Content = append(modelOutput.Content, model.Content{
							Type: "text",
							Text: text,
						})
					}
				}
			}
		}
	}

	return modelOutput
}

type chatCompletionsBuilder struct{}

func (b *chatCompletionsBuilder) defaultAPIPath() string {
	return "/v1/chat/completions"
}

func (b *chatCompletionsBuilder) getAPIType() string {
	return "chatcompletions"
}

func (b *chatCompletionsBuilder) buildRequest(client *agentHTTPClient, messages []model.Message) ([]byte, error) {
	reqMessages := make([]model.Message, 0, len(messages))
	if client.systemPrompt != "" {
		reqMessages = append(reqMessages, model.Message{
			Role:    "system",
			Content: client.systemPrompt,
		})
	}
	reqMessages = append(reqMessages, messages...)

	reqBody := chatCompletionsRequest{
		Model:    client.model,
		Messages: reqMessages,
		Stream:   false,
		User:     client.user,
		Tools:    client.tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

type chatCompletionsRequest struct {
	Model    string             `json:"model"`
	Messages []model.Message    `json:"messages"`
	Stream   bool               `json:"stream"`
	User     string             `json:"user,omitempty"`
	Tools    []config.MCPConfig `json:"tools,omitempty"`
}

func (b *chatCompletionsBuilder) parseResponse(body []byte) (any, error) {
	var rawResp map[string]any
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	modelResp := &model.ChatCompletionsResponse{}

	if id, ok := rawResp["id"].(string); ok {
		modelResp.ID = id
	}
	if obj, ok := rawResp["object"].(string); ok {
		modelResp.Object = obj
	}
	if created, ok := rawResp["created"].(float64); ok {
		modelResp.Created = int64(created)
	}
	if model, ok := rawResp["model"].(string); ok {
		modelResp.Model = model
	}

	if choices, ok := rawResp["choices"].([]any); ok {
		modelResp.Choices = make([]model.Choice, 0, len(choices))
		for i, choiceData := range choices {
			choiceMap, ok := choiceData.(map[string]any)
			if !ok {
				continue
			}
			choice := model.Choice{
				Index: i,
			}
			if finishReason, ok := choiceMap["finish_reason"].(string); ok {
				choice.FinishReason = finishReason
			}
			if msgData, ok := choiceMap["message"].(map[string]any); ok {
				msg := model.Message{}
				if role, ok := msgData["role"].(string); ok {
					msg.Role = role
				}
				if content, ok := msgData["content"].(string); ok {
					msg.Content = content
				}
				choice.Message = msg
			}
			modelResp.Choices = append(modelResp.Choices, choice)
		}
	}

	if usageData, ok := rawResp["usage"].(map[string]any); ok {
		usage := model.Usage{}
		if promptTokens, ok := usageData["prompt_tokens"].(float64); ok {
			usage.InputTokens = int(promptTokens)
		}
		if completionTokens, ok := usageData["completion_tokens"].(float64); ok {
			usage.OutputTokens = int(completionTokens)
		}
		if totalTokens, ok := usageData["total_tokens"].(float64); ok {
			usage.TotalTokens = int(totalTokens)
		}
		modelResp.Usage = usage
	}

	return modelResp, nil
}