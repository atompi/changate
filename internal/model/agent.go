package model

import (
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MessageContentPart is the gateway-internal representation of a Feishu message part.
// Type discriminates the payload: "input_text" → Text, "input_image" → ImageData.
type MessageContentPart struct {
	Type      string
	Text      string
	Key       string
	ImageData string
}

type ChatCompletionsImageURL struct {
	URL string `json:"url,omitempty"`
}

type ChatCompletionsContentPart struct {
	Type     string                   `json:"type"`
	Text     string                   `json:"text,omitempty"`
	ImageURL *ChatCompletionsImageURL `json:"image_url,omitempty"`
}

// OpenResponsesContentPart mirrors the OpenResponses API request shape.
// ImageData is a "data:image/...;base64,..." URL (same wire format as ChatCompletions).
type OpenResponsesContentPart struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ImageData string `json:"image_url,omitempty"`
}

type OpenResponsesResponse struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	CreatedAt int64    `json:"created_at"`
	Model     string   `json:"model"`
	Output    []Output `json:"output"`
	Usage     Usage    `json:"usage"`
}

type Output struct {
	Type    string    `json:"type"`
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatCompletionsResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (r *OpenResponsesResponse) GetContent() string {
	if len(r.Output) == 0 {
		return ""
	}

	var replyText strings.Builder
	for _, output := range r.Output {
		if output.Type == "message" && output.Role == "assistant" {
			for _, part := range output.Content {
				replyText.WriteString(part.Text)
			}
		}
	}

	return replyText.String()
}

func (r *OpenResponsesResponse) GetFilePath() string {
	return extractMediaPath(r.GetContent(), " \t\n")
}

func (r *ChatCompletionsResponse) GetContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	var replyText strings.Builder
	for _, choice := range r.Choices {
		if choice.Message.Role == "assistant" {
			if text, ok := choice.Message.Content.(string); ok {
				replyText.WriteString("\n")
				replyText.WriteString(text)
			}
		}
	}
	return replyText.String()
}

func (r *ChatCompletionsResponse) GetFilePath() string {
	return extractMediaPath(r.GetContent(), "\n")
}

func extractMediaPath(replyText, separators string) string {
	if replyText == "" {
		return ""
	}
	const mediaPrefix = "MEDIA:"
	idx := strings.Index(replyText, mediaPrefix)
	if idx == -1 {
		return ""
	}
	pathStart := idx + len(mediaPrefix)
	pathEnd := -1
	for i, r := range replyText[pathStart:] {
		if strings.ContainsRune(separators, r) {
			pathEnd = pathStart + i
			break
		}
	}
	if pathEnd == -1 {
		pathEnd = len(replyText)
	}
	return replyText[pathStart:pathEnd]
}
