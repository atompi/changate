package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
	"github.com/atompi/changate/pkg/retry"
)

const (
	maxResponseBodyBytes = 10 * 1024 * 1024
	logBodyByteLimit     = 2048

	defaultAPIPathResponses       = "/v1/responses"
	defaultAPIPathChatCompletions = "/v1/chat/completions"

	defaultHTTPTimeout = 120 * time.Second
)

type agentHTTPClient struct {
	baseURL        string
	apiPath        string
	apiName        string
	model          string
	token          string
	user           string
	systemPrompt   string
	tools          []config.MCPConfig
	toolChoice     string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration

	responsesBld *responsesBuilder
	chatBld      *chatCompletionsBuilder
}

var _ Client = (*agentHTTPClient)(nil)

func newAgentHTTPClient(cfg Config) (*agentHTTPClient, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultHTTPTimeout
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBaseDelay == 0 {
		cfg.RetryBaseDelay = 100 * time.Millisecond
	}

	c := &agentHTTPClient{
		baseURL:        cfg.BaseURL,
		model:          cfg.Model,
		token:          cfg.Token,
		user:           cfg.User,
		systemPrompt:   cfg.SystemPrompt,
		tools:          cfg.Tools,
		toolChoice:     cfg.ToolChoice,
		httpClient:     &http.Client{Timeout: cfg.Timeout},
		maxRetries:     cfg.MaxRetries,
		retryBaseDelay: cfg.RetryBaseDelay,
	}

	switch cfg.AgentType {
	case TypeChatCompletions:
		c.chatBld = &chatCompletionsBuilder{}
		c.apiName = "chatcompletions"
		c.apiPath = firstNonEmpty(cfg.APIPath, defaultAPIPathChatCompletions)
	case TypeOpenResponses, "":
		c.responsesBld = &responsesBuilder{}
		c.apiName = "responses"
		c.apiPath = firstNonEmpty(cfg.APIPath, defaultAPIPathResponses)
	default:
		return nil, fmt.Errorf("unknown agent type %q (want %q or %q)", cfg.AgentType, TypeOpenResponses, TypeChatCompletions)
	}

	return c, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (c *agentHTTPClient) OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error) {
	return doCall(c, ctx, []model.Message{{Role: "user", Content: contentParts}}, c.responsesBld.parseResponse)
}

func (c *agentHTTPClient) ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error) {
	return doCall(c, ctx, []model.Message{{
		Role:    "user",
		Content: contentParts,
	}}, c.chatBld.parseResponse)
}

type responseParser[T any] func(body []byte) (T, error)

func doCall[T any](c *agentHTTPClient, ctx context.Context, messages []model.Message, parse responseParser[T]) (T, error) {
	var zero T

	reqBody, err := c.buildRequest(messages)
	if err != nil {
		return zero, fmt.Errorf("failed to build request: %w", err)
	}

	httpResp, err := c.executeWithRetry(ctx, reqBody)
	if err != nil {
		return zero, fmt.Errorf("%s HTTP request failed: %w", c.apiName, err)
	}
	defer httpResp.Body.Close()

	body, err := readBoundedResponse(httpResp.Body, maxResponseBodyBytes)
	if err != nil {
		return zero, fmt.Errorf("failed to read response body: %w", err)
	}

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.Debug("agent response body",
			slog.String("api", c.apiName),
			slog.String("body", truncateForLog(body)),
		)
	}

	if httpResp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("%s API error: status=%d", c.apiName, httpResp.StatusCode)
	}

	return parse(body)
}

func (c *agentHTTPClient) buildRequest(messages []model.Message) ([]byte, error) {
	switch {
	case c.responsesBld != nil:
		return c.responsesBld.buildRequest(c, messages)
	case c.chatBld != nil:
		return c.chatBld.buildRequest(c, messages)
	}
	return nil, fmt.Errorf("no builder configured")
}

// executeWithRetry runs the HTTP request with retry on transient failures.
// On every error path, the response body is explicitly closed; only the
// successful response is returned to the caller (whose defer closes it).
func (c *agentHTTPClient) executeWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	var successResp *http.Response

	err := retry.Do(ctx, retry.Config{
		MaxRetries: c.maxRetries,
		BaseDelay:  c.retryBaseDelay,
		BeforeRetry: func(attempt int, delay time.Duration) {
			slog.Warn("agent request retrying",
				slog.String("api", c.apiName),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
			)
		},
	}, func() error {
		resp, doErr := c.doHTTP(ctx, body)
		if doErr != nil {
			return fmt.Errorf("%w: %v", retry.ErrTransient, doErr)
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			_ = resp.Body.Close()
			return fmt.Errorf("%w: status %d", retry.ErrTransient, resp.StatusCode)
		}
		successResp = resp
		return nil
	})
	if err != nil {
		if successResp != nil {
			_ = successResp.Body.Close()
		}
		return nil, err
	}
	return successResp, nil
}

func (c *agentHTTPClient) doHTTP(ctx context.Context, body []byte) (*http.Response, error) {
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.Debug("agent request body",
			slog.String("api", c.apiName),
			slog.String("body", truncateForLog(body)),
		)
	}

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

func readBoundedResponse(body io.Reader, max int64) ([]byte, error) {
	limited := io.LimitReader(body, max+1)
	read, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(read)) > max {
		return nil, fmt.Errorf("response body exceeds %d bytes", max)
	}
	return read, nil
}

func truncateForLog(body []byte) string {
	if len(body) <= logBodyByteLimit {
		return string(body)
	}
	return string(body[:logBodyByteLimit]) + fmt.Sprintf("... [truncated %d bytes]", len(body)-logBodyByteLimit)
}

type responsesBuilder struct{}

func (b *responsesBuilder) buildRequest(client *agentHTTPClient, messages []model.Message) ([]byte, error) {
	input := make([]map[string]any, 0, len(messages)+1)

	if client.systemPrompt != "" {
		input = append(input, map[string]any{
			"role": "system",
			"content": []map[string]any{
				{"type": "input_text", "text": client.systemPrompt},
			},
		})
	}

	for _, msg := range messages {
		item, err := b.convertModelMessageToInput(msg)
		if err != nil {
			return nil, err
		}
		input = append(input, item)
	}

	reqBody := requestBase{
		Model:      client.model,
		User:       client.user,
		Tools:      client.tools,
		ToolChoice: client.toolChoice,
	}.intoResponses(input)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

func (b *responsesBuilder) convertModelMessageToInput(msg model.Message) (map[string]any, error) {
	var content any
	switch v := msg.Content.(type) {
	case string:
		content = []map[string]any{{"type": "input_text", "text": v}}
	case []model.OpenResponsesContentPart:
		content = b.convertContentPartsToInput(v)
	default:
		return nil, fmt.Errorf("OpenResponses: unsupported content type %T for role %q", msg.Content, msg.Role)
	}

	return map[string]any{
		"role":    msg.Role,
		"content": content,
	}, nil
}

func (b *responsesBuilder) convertContentPartsToInput(parts []model.OpenResponsesContentPart) []map[string]any {
	result := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text":
			result = append(result, map[string]any{"type": "input_text", "text": part.Text})
		case "input_image":
			if part.ImageData != "" {
				result = append(result, map[string]any{"type": "input_image", "image_url": part.ImageData})
			}
		default:
			slog.Warn("OpenResponses: dropping unknown content part type", slog.String("type", part.Type))
		}
	}
	return result
}

type requestBase struct {
	Model      string             `json:"model"`
	User       string             `json:"user,omitempty"`
	Tools      []config.MCPConfig `json:"tools,omitempty"`
	ToolChoice string             `json:"tool_choice,omitempty"`
}

type responsesRequest struct {
	requestBase
	Input []map[string]any `json:"input"`
}

type chatCompletionsRequest struct {
	requestBase
	Messages []model.Message `json:"messages"`
}

func (b requestBase) intoResponses(input []map[string]any) responsesRequest {
	return responsesRequest{requestBase: b, Input: input}
}

func (b requestBase) intoChatCompletions(messages []model.Message) chatCompletionsRequest {
	return chatCompletionsRequest{requestBase: b, Messages: messages}
}

func (b *responsesBuilder) parseResponse(body []byte) (*model.OpenResponsesResponse, error) {
	var rawResp map[string]any
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	modelResp := &model.OpenResponsesResponse{
		ID:        rawString(rawResp, "id"),
		Status:    rawString(rawResp, "status"),
		CreatedAt: rawInt64(rawResp, "created_at"),
		Model:     rawString(rawResp, "model"),
	}

	if output, ok := rawResp["output"].([]any); ok {
		modelResp.Output = make([]model.Output, 0, len(output))
		for _, item := range output {
			if out, ok := b.convertOutputItemToModel(item); ok {
				modelResp.Output = append(modelResp.Output, out)
			}
		}
	}

	if usageData, ok := rawResp["usage"].(map[string]any); ok {
		usage := model.Usage{
			InputTokens:  rawInt(usageData, "input_tokens"),
			OutputTokens: rawInt(usageData, "output_tokens"),
			TotalTokens:  rawInt(usageData, "total_tokens"),
		}
		warnIfUsageMismatch("OpenResponses", usage)
		modelResp.Usage = usage
	}

	return modelResp, nil
}

func (b *responsesBuilder) convertOutputItemToModel(item any) (model.Output, bool) {
	itemMap, ok := item.(map[string]any)
	if !ok {
		return model.Output{}, false
	}

	itemType, _ := itemMap["type"].(string)
	role, _ := itemMap["role"].(string)

	if itemType == "function_call" || itemType == "tool_use" {
		slog.Info("OpenResponses: received tool-call output (not yet propagated to Feishu)", slog.String("type", itemType))
		return model.Output{}, false
	}

	if itemType != "message" || role != "assistant" {
		return model.Output{}, false
	}

	modelOutput := model.Output{Type: itemType, Role: role}

	if content, ok := itemMap["content"].([]any); ok {
		modelOutput.Content = make([]model.Content, 0, len(content))
		for _, c := range content {
			cMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if cMap["type"] != "output_text" {
				continue
			}
			if text, ok := cMap["text"].(string); ok {
				modelOutput.Content = append(modelOutput.Content, model.Content{
					Type: "text",
					Text: text,
				})
			}
		}
	}

	return modelOutput, true
}

type chatCompletionsBuilder struct{}

func (b *chatCompletionsBuilder) buildRequest(client *agentHTTPClient, messages []model.Message) ([]byte, error) {
	reqMessages := make([]model.Message, 0, len(messages)+1)
	if client.systemPrompt != "" {
		reqMessages = append(reqMessages, model.Message{
			Role:    "system",
			Content: client.systemPrompt,
		})
	}
	reqMessages = append(reqMessages, messages...)

	reqBody := requestBase{
		Model:      client.model,
		User:       client.user,
		Tools:      client.tools,
		ToolChoice: client.toolChoice,
	}.intoChatCompletions(reqMessages)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

func (b *chatCompletionsBuilder) parseResponse(body []byte) (*model.ChatCompletionsResponse, error) {
	var rawResp map[string]any
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	modelResp := &model.ChatCompletionsResponse{
		ID:      rawString(rawResp, "id"),
		Object:  rawString(rawResp, "object"),
		Created: rawInt64(rawResp, "created"),
		Model:   rawString(rawResp, "model"),
	}

	if choices, ok := rawResp["choices"].([]any); ok {
		modelResp.Choices = make([]model.Choice, 0, len(choices))
		for i, choiceData := range choices {
			choiceMap, ok := choiceData.(map[string]any)
			if !ok {
				continue
			}
			msgMap := rawMap(choiceMap, "message")
			choice := model.Choice{
				Index:        i,
				FinishReason: rawString(choiceMap, "finish_reason"),
				Message: model.Message{
					Role:    rawString(msgMap, "role"),
					Content: rawString(msgMap, "content"),
				},
			}
			modelResp.Choices = append(modelResp.Choices, choice)
		}
	}

	if usageData, ok := rawResp["usage"].(map[string]any); ok {
		usage := model.Usage{
			InputTokens:  rawInt(usageData, "prompt_tokens"),
			OutputTokens: rawInt(usageData, "completion_tokens"),
			TotalTokens:  rawInt(usageData, "total_tokens"),
		}
		warnIfUsageMismatch("ChatCompletions", usage)
		modelResp.Usage = usage
	}

	return modelResp, nil
}

func warnIfUsageMismatch(apiName string, u model.Usage) {
	if u.TotalTokens > 0 && u.InputTokens+u.OutputTokens != u.TotalTokens {
		slog.Warn("agent token usage mismatch",
			slog.String("api", apiName),
			slog.Int("input", u.InputTokens),
			slog.Int("output", u.OutputTokens),
			slog.Int("total", u.TotalTokens),
		)
	}
}

func rawString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func rawInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	f, _ := m[key].(float64)
	return int(f)
}

func rawInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	f, _ := m[key].(float64)
	return int64(f)
}

func rawMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	mm, _ := m[key].(map[string]any)
	return mm
}
