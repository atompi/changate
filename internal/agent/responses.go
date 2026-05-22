package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/atompi/changate/internal/model"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type responsesClient struct {
	sdkClient openai.Client
	model     string
	tools     []model.MCPTool
}

func newOpenResponsesClient(sdkClient openai.Client, apiPath, model string, tools []model.MCPTool) *responsesClient {
	return &responsesClient{
		sdkClient: sdkClient,
		model:     model,
		tools:     tools,
	}
}

func (c *responsesClient) OpenResponses(ctx context.Context, messages []model.Message) (*model.OpenResponsesResponse, error) {
	maxIterations := 10
	executor := NewMCPToolExecutor(30 * time.Second)
	var previousResponseID string
	var isFirstCall = true

	for i := 0; i < maxIterations; i++ {
		var sdkInput []responses.ResponseInputItemUnionParam

		if isFirstCall {
			sdkInput = make([]responses.ResponseInputItemUnionParam, 0, len(messages))
			for _, msg := range messages {
				input := convertModelMessageToSDKResponse(msg)
				if input.OfMessage != nil {
					sdkInput = append(sdkInput, input)
				}
			}
			isFirstCall = false
		}

		sdkTools := convertMCPToolsToSDKResponse(c.tools)

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(c.model),
		}
		if previousResponseID != "" {
			params.PreviousResponseID = openai.Opt(previousResponseID)
		}
		if len(sdkInput) > 0 {
			params.Input = responses.ResponseNewParamsInputUnion{
				OfInputItemList: sdkInput,
			}
		}
		if len(sdkTools) > 0 {
			params.Tools = sdkTools
			params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
				OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
			}
		}

		resp, err := c.sdkClient.Responses.New(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("responses API failed: %w", err)
		}

		modelResp := convertSDKResponseToModel(resp)
		previousResponseID = modelResp.ID

		if !modelResp.HasToolCalls() {
			return modelResp, nil
		}

		toolCalls := modelResp.GetToolCalls()
		if len(toolCalls) == 0 {
			return modelResp, nil
		}

		sdkInput = make([]responses.ResponseInputItemUnionParam, 0, len(toolCalls))
		for _, tc := range toolCalls {
			tool := c.findToolByName(tc.Name)
			var result string
			if tool == nil {
				result = fmt.Sprintf("Error: tool not found: %s", tc.Name)
			} else {
				var err error
				result, err = executor.ExecuteTool(ctx, *tool, tc)
				if err != nil {
					result = fmt.Sprintf("Error executing tool: %v", err)
				}
			}

			sdkInput = append(sdkInput, responses.ResponseInputItemParamOfFunctionCallOutput(tc.ID, result))
		}
	}

	return nil, fmt.Errorf("max tool call iterations exceeded")
}

func (c *responsesClient) findToolByName(name string) *model.MCPTool {
	for i := range c.tools {
		if c.tools[i].ServerLabel == name {
			return &c.tools[i]
		}
	}
	return nil
}

func (c *responsesClient) OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error) {
	reqInput := model.Message{
		Role:    "user",
		Content: contentParts,
	}
	messages := []model.Message{reqInput}
	return c.OpenResponses(ctx, messages)
}

func (c *responsesClient) GetTimeout() time.Duration {
	return 0
}

func (c *responsesClient) ChatCompletions(ctx context.Context, messages []model.Message) (*model.ChatCompletionsResponse, error) {
	return nil, fmt.Errorf("ChatCompletions not supported for OpenResponses client")
}

func (c *responsesClient) ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error) {
	return nil, fmt.Errorf("ChatCompletions not supported for OpenResponses client")
}

func convertModelMessageToSDKResponse(msg model.Message) responses.ResponseInputItemUnionParam {
	content := msg.Content

	if parts, ok := content.([]model.OpenResponsesContentPart); ok && len(parts) > 0 {
		contentList := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
		for _, part := range parts {
			if part.Type == "input_text" {
				contentList = append(contentList, responses.ResponseInputContentUnionParam{
					OfInputText: &responses.ResponseInputTextParam{
						Text: part.Text,
					},
				})
			} else if part.Type == "input_image" && part.ImageURL != "" {
				contentList = append(contentList, responses.ResponseInputContentUnionParam{
					OfInputImage: &responses.ResponseInputImageParam{
						ImageURL: openai.Opt(part.ImageURL),
					},
				})
			}
		}
		if len(contentList) > 0 {
			return responses.ResponseInputItemParamOfMessage(contentList, responses.EasyInputMessageRoleUser)
		}
	}

	contentStr, ok := content.(string)
	if !ok {
		return responses.ResponseInputItemUnionParam{}
	}

	return responses.ResponseInputItemParamOfMessage(
		responses.ResponseInputMessageContentListParam{
			responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{
					Text: contentStr,
				},
			},
		},
		responses.EasyInputMessageRoleUser,
	)
}

func convertMCPToolsToSDKResponse(tools []model.MCPTool) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	sdkTools := make([]responses.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		if tool.Type == "mcp" {
			mcpParam := responses.ToolMcpParam{
				ServerLabel: tool.ServerLabel,
			}
			if tool.ServerURL != "" {
				mcpParam.ServerURL = openai.String(tool.ServerURL)
			}
			if tool.Token != "" {
				mcpParam.Authorization = openai.String(tool.Token)
			}
			sdkTools = append(sdkTools, responses.ToolUnionParam{
				OfMcp: &mcpParam,
			})
		}
	}
	return sdkTools
}

func convertSDKResponseToModel(resp *responses.Response) *model.OpenResponsesResponse {
	if resp == nil {
		return nil
	}

	modelResp := &model.OpenResponsesResponse{
		ID:     resp.ID,
		Status: string(resp.Status),
	}

	if resp.Model != "" {
		modelResp.Model = string(resp.Model)
	}

	if len(resp.Output) > 0 {
		modelResp.Output = make([]model.Output, 0, len(resp.Output))
		for _, item := range resp.Output {
			output := convertSDKOutputItemToModel(item)
			if output != nil {
				modelResp.Output = append(modelResp.Output, *output)
			}
		}
	}

	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 || resp.Usage.TotalTokens > 0 {
		modelResp.Usage = model.Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		}
	}

	return modelResp
}

func convertSDKOutputItemToModel(item responses.ResponseOutputItemUnion) *model.Output {
	modelOutput := &model.Output{}

	switch item.Type {
	case "message":
		msg := item.AsMessage()
		modelOutput.Type = "message"
		modelOutput.Role = string(msg.Role)
		if len(msg.Content) > 0 {
			modelOutput.Content = make([]model.Content, 0, len(msg.Content))
			for _, c := range msg.Content {
				if c.Type == "output_text" {
					modelOutput.Content = append(modelOutput.Content, model.Content{
						Type: "text",
						Text: c.Text,
					})
				}
			}
		}
	case "function_call":
		fc := item.AsFunctionCall()
		modelOutput.Type = "function_call"
		modelOutput.Name = fc.Name
		modelOutput.CallId = fc.CallID
		modelOutput.Arguments = fc.Arguments
	}

	return modelOutput
}
