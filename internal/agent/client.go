package agent

import (
	"context"
	"time"

	"github.com/atompi/changate/internal/model"
)

type Client interface {
	Responses(ctx context.Context, messages []model.Message) (*model.ResponsesResponse, error)
	ResponsesWithContent(ctx context.Context, contentParts []model.ContentPart) (*model.ResponsesResponse, error)
	GetTimeout() time.Duration
}