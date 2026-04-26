package model

import (
	"strings"
)

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (r *ChatCompletionRequest) BuildMessages(userContent []ContentPart, systemPrompt string) {
	r.Messages = []Message{}

	if systemPrompt != "" {
		r.Messages = append(r.Messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	var content interface{}
	if len(userContent) == 1 && userContent[0].Type == "text" {
		content = userContent[0].Text
	} else {
		content = userContent
	}

	r.Messages = append(r.Messages, Message{
		Role:    "user",
		Content: content,
	})
}

func (r *ChatCompletionResponse) GetContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	switch c := r.Choices[0].Message.Content.(type) {
	case string:
		return c
	case []ContentPart:
		var sb strings.Builder
		for _, part := range c {
			if part.Text != "" {
				sb.WriteString(part.Text)
			}
		}
		return sb.String()
	}
	return ""
}

func (r *ChatCompletionResponse) IsError() bool {
	return r.Choices == nil || len(r.Choices) == 0
}

func BuildHermesRequest(model string, userContent []ContentPart, stream bool) *ChatCompletionRequest {
	req := &ChatCompletionRequest{
		Model:  model,
		Stream: stream,
	}
	req.BuildMessages(userContent, "")
	return req
}