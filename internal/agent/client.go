package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
)

const (
	TypeChatCompletions = "ChatCompletions"
	TypeOpenResponses   = "OpenResponses"
)

// Client is the surface area used by the Feishu callback handler.
type Client interface {
	OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error)
	ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error)
}

type Config struct {
	BaseURL        string
	APIPath        string
	Model          string
	Token          string
	User           string
	Timeout        time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
	AgentType      string
	SystemPrompt   string
	Tools          []config.MCPConfig
	ToolChoice     string
}

func NewClient(cfg Config) (Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("agent BaseURL is required")
	}
	return newAgentHTTPClient(cfg)
}
