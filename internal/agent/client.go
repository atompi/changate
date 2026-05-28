package agent

import (
	"context"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
)

type Client interface {
	OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error)
	OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error)
	ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error)
	ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error)
	GetTimeout() time.Duration
}

func NewClient(baseURL, apiPath, model, token, user string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration, agentType string, systemPrompt string, tools []config.MCPConfig) Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
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

	var builder requestBuilder
	if agentType == "ChatCompletions" {
		builder = &chatCompletionsBuilder{}
	} else {
		builder = &responsesBuilder{}
	}

	return newAgentHTTPClient(baseURL, apiPath, model, token, user, systemPrompt, tools, timeout, maxRetries, retryBaseDelay, builder)
}