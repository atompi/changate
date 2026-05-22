package agent

import (
	"context"
	"time"

	"github.com/atompi/changate/internal/model"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client interface {
	OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error)
	OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error)
	ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error)
	ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error)
	GetTimeout() time.Duration
}

func NewClient(baseURL, apiPath, model, token, user, conversation string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration, agentType string, tools []model.MCPTool) Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if apiPath == "" {
		if agentType == "ChatCompletions" {
			apiPath = "/v1/chat/completions"
		} else {
			apiPath = "/v1/responses"
		}
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

	sdkClient := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(token),
		option.WithMaxRetries(maxRetries),
		option.WithRequestTimeout(timeout),
	)

	if agentType == "ChatCompletions" {
		return newChatCompletionsClient(sdkClient, apiPath, model, tools)
	}
	return newOpenResponsesClient(sdkClient, apiPath, model, tools)
}
