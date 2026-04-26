package model

import (
	"testing"
)

func TestParseMessageContent_Text(t *testing.T) {
	content := `{"text":"hello world"}`
	parts, err := ParseMessageContent(content)
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("ParseMessageContent() returned %d parts, want 1", len(parts))
	}
	if parts[0].Type != "text" {
		t.Errorf("parts[0].Type = %v, want text", parts[0].Type)
	}
	if parts[0].Text != "hello world" {
		t.Errorf("parts[0].Text = %v, want %v", parts[0].Text, "hello world")
	}
}

func TestParseMessageContent_Multimodal(t *testing.T) {
	content := `[{"type":"text","text":"What is in this image?"},{"type":"image_url","image_url":{"url":"https://example.com/cat.png","detail":"high"}}]`
	parts, err := ParseMessageContent(content)
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("ParseMessageContent() returned %d parts, want 2", len(parts))
	}
	if parts[0].Type != "text" {
		t.Errorf("parts[0].Type = %v, want text", parts[0].Type)
	}
	if parts[1].Type != "image_url" {
		t.Errorf("parts[1].Type = %v, want image_url", parts[1].Type)
	}
}

func TestParseMessageContent_InvalidJSON(t *testing.T) {
	content := `not json`
	_, err := ParseMessageContent(content)
	if err == nil {
		t.Error("ParseMessageContent() should return error for invalid JSON")
	}
}

func TestChatCompletionRequest_BuildMessages(t *testing.T) {
	req := &ChatCompletionRequest{}
	userContent := []ContentPart{
		{Type: "text", Text: "hello"},
	}

	req.BuildMessages(userContent, "You are helpful")

	if len(req.Messages) != 2 {
		t.Fatalf("BuildMessages() created %d messages, want 2", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %v, want system", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "user" {
		t.Errorf("Messages[1].Role = %v, want user", req.Messages[1].Role)
	}
}

func TestChatCompletionResponse_GetContent(t *testing.T) {
	resp := &ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Content: "Hello! How can I help you?",
				},
			},
		},
	}

	content := resp.GetContent()
	if content != "Hello! How can I help you?" {
		t.Errorf("GetContent() = %v, want %v", content, "Hello! How can I help you?")
	}
}

func TestChatCompletionResponse_IsError(t *testing.T) {
	resp := &ChatCompletionResponse{
		Choices: []Choice{},
	}

	if !resp.IsError() {
		t.Error("IsError() should return true for empty choices")
	}

	resp.Choices = []Choice{{}}
	if resp.IsError() {
		t.Error("IsError() should return false for non-empty choices")
	}
}
