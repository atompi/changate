package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/atompi/changate/internal/model"
	"github.com/openai/openai-go/v3"
)

type chatCompletionsClient struct {
	sdkClient openai.Client
	model     string
	tools     []model.MCPTool
}

func newChatCompletionsClient(sdkClient openai.Client, apiPath, model string, tools []model.MCPTool) *chatCompletionsClient {
	return &chatCompletionsClient{
		sdkClient: sdkClient,
		model:     model,
		tools:     tools,
	}
}

func (c *chatCompletionsClient) ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error) {
	maxIterations := 10
	executor := NewMCPToolExecutor(30 * time.Second)

	for i := 0; i < maxIterations; i++ {
		sdkMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
		for _, msg := range messages {
			sdkMsg := convertModelMessageToSDK(msg)
			if sdkMsg != nil {
				sdkMessages = append(sdkMessages, *sdkMsg)
			}
		}

		sdkTools := convertMCPToolsToSDKChat(c.tools)

		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(c.model),
			Messages: sdkMessages,
		}
		if len(sdkTools) > 0 {
			params.Tools = sdkTools
		}

		resp, err := c.sdkClient.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("chat completions failed: %w", err)
		}

		modelResp := convertSDKChatCompletionToModel(resp)

		if !modelResp.HasToolCalls() {
			return modelResp, nil
		}

		toolCalls := modelResp.GetToolCalls()
		if len(toolCalls) == 0 {
			return modelResp, nil
		}

		assistantMsg := model.Message{
			Role:      "assistant",
			Content:   "",
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMsg)

		for _, tc := range toolCalls {
			tool := c.findToolByName(tc.Name)
			if tool == nil {
				messages = append(messages, model.Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Error: tool not found: %s", tc.Name),
					ToolCallID: tc.ID,
				})
				continue
			}

			result, err := executor.ExecuteTool(ctx, *tool, tc)
			if err != nil {
				result = fmt.Sprintf("Error executing tool: %v", err)
			}

			messages = append(messages, model.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return nil, fmt.Errorf("max tool call iterations exceeded")
}

func (c *chatCompletionsClient) findToolByName(name string) *model.MCPTool {
	for i := range c.tools {
		if c.tools[i].ServerLabel == name {
			return &c.tools[i]
		}
	}
	return nil
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
	return 0
}

func convertModelMessageToSDK(msg model.Message) *openai.ChatCompletionMessageParamUnion {
	content := msg.Content

	if parts, ok := content.([]model.ChatCompletionsContentPart); ok && len(parts) > 0 {
		contentParts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(parts))
		for _, part := range parts {
			if part.Type == "text" {
				contentParts = append(contentParts, openai.TextContentPart(part.Text))
			} else if part.Type == "image_url" && part.ImageURL != nil {
				contentParts = append(contentParts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    part.ImageURL.URL,
					Detail: part.ImageURL.Detail,
				}))
			}
		}
		if len(contentParts) > 0 {
			msg := openai.UserMessage(contentParts)
			return &msg
		}
	}

	contentStr, ok := content.(string)
	if !ok {
		return nil
	}

	switch msg.Role {
	case "user":
		msg := openai.UserMessage(contentStr)
		return &msg
	case "assistant":
		msg := openai.AssistantMessage(contentStr)
		return &msg
	case "system":
		msg := openai.SystemMessage(contentStr)
		return &msg
	case "developer":
		msg := openai.DeveloperMessage(contentStr)
		return &msg
	case "tool":
		toolMsg := openai.ToolMessage(contentStr, msg.ToolCallID)
		return &toolMsg
	default:
		msg := openai.UserMessage(contentStr)
		return &msg
	}
}

func convertMCPToolsToSDKChat(tools []model.MCPTool) []openai.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	sdkTools := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		if tool.Type == "mcp" {
			sdkTools = append(sdkTools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name:        tool.ServerLabel,
				Description: openai.String(fmt.Sprintf("MCP: %s (%s)", tool.ServerLabel, tool.ServerURL)),
				Parameters: openai.FunctionParameters{
					"type": "object",
				},
			}))
		}
	}
	return sdkTools
}

func convertSDKChatCompletionToModel(resp *openai.ChatCompletion) *model.ChatCompletionsResponse {
	if resp == nil {
		return nil
	}

	modelResp := &model.ChatCompletionsResponse{
		ID:      resp.ID,
		Object:  string(resp.Object),
		Created: resp.Created,
		Model:   resp.Model,
	}

	if len(resp.Choices) > 0 {
		modelResp.Choices = make([]model.Choice, 0, len(resp.Choices))
		for _, choice := range resp.Choices {
			modelChoice := model.Choice{
				Index:        int(choice.Index),
				FinishReason: string(choice.FinishReason),
			}
			if choice.Message.Content != "" {
				modelChoice.Message = model.Message{
					Role:    string(choice.Message.Role),
					Content: choice.Message.Content,
				}
			}
			if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
				tc := make([]model.ToolCall, 0, len(choice.Message.ToolCalls))
				for _, toolCall := range choice.Message.ToolCalls {
					tc = append(tc, model.ToolCall{
						ID:        toolCall.ID,
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					})
				}
				modelChoice.ToolCalls = tc
			}
			modelResp.Choices = append(modelResp.Choices, modelChoice)
		}
	}

	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 || resp.Usage.TotalTokens > 0 {
		modelResp.Usage = model.Usage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		}
	}

	return modelResp
}
