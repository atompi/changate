package model

import (
	"testing"
)

func TestParseMessageContent_Text(t *testing.T) {
	content := `{"text":"hello world"}`
	parts, err := ParseMessageContent(content, "text")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("ParseMessageContent() returned %d parts, want 1", len(parts))
	}
	if parts[0].Type != "input_text" {
		t.Errorf("parts[0].Type = %v, want input_text", parts[0].Type)
	}
	if parts[0].Text != "hello world" {
		t.Errorf("parts[0].Text = %v, want %v", parts[0].Text, "hello world")
	}
}

func TestParseMessageContent_InvalidJSON(t *testing.T) {
	content := `not json`
	_, err := ParseMessageContent(content, "text")
	if err == nil {
		t.Error("ParseMessageContent() should return error for invalid JSON")
	}
}

func TestResponsesResponse_GetContent_StringContent(t *testing.T) {
	resp := &ResponsesResponse{
		Output: []Output{
			{
				Type:   "message",
				Role:   "assistant",
				Content: []Content{
					{Type: "text", Text: "simple string response"},
				},
			},
		},
	}

	content := resp.GetContent()
	if content != "simple string response" {
		t.Errorf("GetContent() = %q, want %q", content, "simple string response")
	}
}

func TestResponsesResponse_GetContent_EmptyOutput(t *testing.T) {
	resp := &ResponsesResponse{
		Output: []Output{},
	}

	content := resp.GetContent()
	if content != "" {
		t.Errorf("GetContent() = %q, want empty string", content)
	}
}

func TestResponsesResponse_GetFilePath_WithMediaPrefix(t *testing.T) {
	resp := &ResponsesResponse{
		Output: []Output{
			{
				Type:   "message",
				Role:   "assistant",
				Content: []Content{
					{Type: "text", Text: "MEDIA:/opt/data/cache/screenshots/browser_screenshot_8dacf3bc0000409baf524ca3110e170d.png\n"},
				},
			},
		},
	}

	path := resp.GetFilePath()
	if path != "/opt/data/cache/screenshots/browser_screenshot_8dacf3bc0000409baf524ca3110e170d.png" {
		t.Errorf("GetFilePath() = %q, want %q", path, "/opt/data/cache/screenshots/browser_screenshot_8dacf3bc0000409baf524ca3110e170d.png")
	}
}

func TestResponsesResponse_GetFilePath_WithoutMediaPrefix(t *testing.T) {
	resp := &ResponsesResponse{
		Output: []Output{
			{
				Type:   "message",
				Role:   "assistant",
				Content: []Content{
					{Type: "text", Text: "https://example.com/image.png"},
				},
			},
		},
	}

	path := resp.GetFilePath()
	if path != "" {
		t.Errorf("GetFilePath() = %q, want empty string when no MEDIA: prefix", path)
	}
}

func TestResponsesResponse_GetFilePath_Empty(t *testing.T) {
	resp := &ResponsesResponse{
		Output: []Output{},
	}

	url := resp.GetFilePath()
	if url != "" {
		t.Errorf("GetFilePath() = %q, want empty string", url)
	}
}

func TestParseMessageContent_FeishuRichText_ImageOnly(t *testing.T) {
	content := `{"title":"","content":[[{"tag":"img","image_key":"img_v3_0211j_ee5508f3-30cb-444c-bf23-3c0956933ffg","width":904,"height":362}]]}`
	parts, err := ParseMessageContent(content, "post")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Type != "input_image" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_image")
	}
	if parts[0].ImageURL == "" {
		t.Fatal("parts[0].ImageURL is empty")
	}
	if parts[0].ImageURL != "img_v3_0211j_ee5508f3-30cb-444c-bf23-3c0956933ffg" {
		t.Errorf("parts[0].ImageURL = %q, want %q", parts[0].ImageURL, "img_v3_0211j_ee5508f3-30cb-444c-bf23-3c0956933ffg")
	}
}

func TestParseMessageContent_FeishuRichText_TextOnly(t *testing.T) {
	content := `{"title":"","content":[[{"tag":"text","text":"这张图片里有什么？","style":[]}]]}`
	parts, err := ParseMessageContent(content, "post")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Type != "input_text" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_text")
	}
	if parts[0].Text != "这张图片里有什么？" {
		t.Errorf("parts[0].Text = %q, want %q", parts[0].Text, "这张图片里有什么？")
	}
}

func TestParseMessageContent_FeishuRichText_ImageAndText(t *testing.T) {
	content := `{"title":"","content":[[{"tag":"img","image_key":"img_v3_0211j_ee5508f3-30cb-444c-bf23-3c0956933ffg","width":904,"height":362}],[{"tag":"text","text":"这张图片里有什么？","style":[]}]]}`
	parts, err := ParseMessageContent(content, "post")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2", len(parts))
	}
	if parts[0].Type != "input_image" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_image")
	}
	if parts[0].ImageURL == "" || parts[0].ImageURL != "img_v3_0211j_ee5508f3-30cb-444c-bf23-3c0956933ffg" {
		t.Errorf("parts[0].ImageURL = %v, want ...", parts[0].ImageURL)
	}
	if parts[1].Type != "input_text" {
		t.Errorf("parts[1].Type = %q, want %q", parts[1].Type, "input_text")
	}
	if parts[1].Text != "这张图片里有什么？" {
		t.Errorf("parts[1].Text = %q, want %q", parts[1].Text, "这张图片里有什么？")
	}
}

func TestParseMessageContent_FeishuImageMessage(t *testing.T) {
	content := `{"image_key":"img_v3_0211j_98ce9879-d624-40ba-b871-d22d1d56e8ag"}`
	parts, err := ParseMessageContent(content, "image")
	if err != nil {
		t.Fatalf("ParseMessageContent() error = %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Type != "input_image" {
		t.Errorf("parts[0].Type = %q, want %q", parts[0].Type, "input_image")
	}
	if parts[0].ImageURL == "" {
		t.Fatal("parts[0].ImageURL is empty")
	}
	if parts[0].ImageURL != "img_v3_0211j_98ce9879-d624-40ba-b871-d22d1d56e8ag" {
		t.Errorf("parts[0].ImageURL = %q, want %q", parts[0].ImageURL, "img_v3_0211j_98ce9879-d624-40ba-b871-d22d1d56e8ag")
	}
}

func TestParseMessageContent_UnsupportedType(t *testing.T) {
	content := `{"text":"hello"}`
	_, err := ParseMessageContent(content, "audio")
	if err == nil {
		t.Error("ParseMessageContent() should return error for unsupported message type")
	}
	if err != nil && err.Error() != "unsupported message type: audio" {
		t.Errorf("error = %q, want %q", err.Error(), "unsupported message type: audio")
	}
}
