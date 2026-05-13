package agent

import (
	"context"
	"time"

	"github.com/atompi/changate/internal/model"
)

type Client interface {
	OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error)
	OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error)
	ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error)
	ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error)
	GetTimeout() time.Duration
}

// NewClient creates the appropriate agent client based on agentType.
// agentType should be "OpenResponses" or "ChatCompletions".
// Defaults to OpenResponses if empty or unknown.
func NewClient(baseURL, apiPath, model, token, user, conversation string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration, agentType string) Client {
	if agentType == "ChatCompletions" {
		return NewChatCompletionsClient(baseURL, apiPath, model, token, user, conversation, timeout, maxRetries, retryBaseDelay)
	}
	return NewOpenResponsesClient(baseURL, apiPath, model, token, user, conversation, timeout, maxRetries, retryBaseDelay)
}
