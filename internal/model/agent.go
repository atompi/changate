package model

import (
	"strings"
	"unicode"
)

type ResponsesResponse = OpenResponsesResponse

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type OpenResponsesResponse struct {
	ID              string   `json:"id"`
	Object          string   `json:"object"`
	Status          string   `json:"status"`
	CreatedAt       int64    `json:"created_at"`
	Model           string   `json:"model"`
	Output          []Output `json:"output"`
	Usage           Usage    `json:"usage"`
	MaxOutputTokens int      `json:"max_output_tokens"`
}

type Output struct {
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Arguments string    `json:"arguments"`
	CallId    string    `json:"call_id"`
	Output    string    `json:"output"`
	Role      string    `json:"role"`
	Content   []Content `json:"content"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatCompletionsResponse struct {
	ID              string   `json:"id"`
	Object          string   `json:"object"`
	Created         int64    `json:"created"`
	Model           string   `json:"model"`
	Choices         []Choice `json:"choices"`
	Usage           Usage    `json:"usage"`
	MaxOutputTokens int      `json:"max_output_tokens"`
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
	replyText := r.GetContent()
	if replyText == "" {
		return ""
	}
	const mediaPrefix = "MEDIA:"
	if strings.Contains(replyText, mediaPrefix) {
		idx := strings.Index(replyText, mediaPrefix)
		pathStart := idx + len(mediaPrefix)
		pathEnd := strings.IndexFunc(replyText[pathStart:], unicode.IsSpace)
		if pathEnd == -1 {
			pathEnd = len(replyText)
		} else {
			pathEnd += pathStart
		}
		return replyText[pathStart:pathEnd]
	}
	return ""
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
	replyText := r.GetContent()
	if replyText == "" {
		return ""
	}
	const mediaPrefix = "MEDIA:"
	if strings.Contains(replyText, mediaPrefix) {
		idx := strings.Index(replyText, mediaPrefix)
		pathStart := idx + len(mediaPrefix)
		pathEnd := strings.Index(replyText[pathStart:], "\n")
		if pathEnd == -1 {
			pathEnd = len(replyText)
		} else {
			pathEnd += pathStart
		}
		return replyText[pathStart:pathEnd]
	}
	return ""
}
