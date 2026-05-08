package model

import "strings"

// GetContent extracts text content from a ResponsesResponse.
// It handles both plain text and content parts formats.
func (r *ResponsesResponse) GetContent() string {
	if len(r.Output) == 0 {
		return ""
	}

	var replyText string
	for _, output := range r.Output {
		if output.Type == "message" && output.Role == "assistant" {
			for _, part := range output.Content {
				replyText += part.Text
			}
		}
	}

	return replyText
}

// GetFilePath extracts a file path from the response content.
// If the content contains "MEDIA:/opt/data/cache/screenshots/browser_screenshot_xxx.png",
// it extracts and returns that file path. Otherwise returns empty string.
func (r *ResponsesResponse) GetFilePath() string {
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

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponsesResponse struct {
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

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
