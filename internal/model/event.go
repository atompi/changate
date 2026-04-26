// Package model provides data types for Feishu event callbacks and agent messages.
package model

import (
	"encoding/json"
	"strings"
)

// URLVerificationRequest is the request body for URL verification from Feishu.
type URLVerificationRequest struct {
	Challenge string `json:"challenge"`
	Type      string `json:"type"`
	Token     string `json:"token"`
}

// URLVerificationResponse is the response body for URL verification.
type URLVerificationResponse struct {
	Challenge string `json:"challenge"`
}

// EventCallbackRequest represents the incoming event callback from Feishu.
type EventCallbackRequest struct {
	Schema string          `json:"schema"`
	Header EventHeader     `json:"header"`
	Event  json.RawMessage `json:"event"`
}

// EventHeader contains metadata about the event.
type EventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// MessageEvent represents the message event data from Feishu.
type MessageEvent struct {
	Sender  SenderInfo  `json:"sender"`
	Message MessageInfo `json:"message"`
}

// SenderInfo contains information about the message sender.
type SenderInfo struct {
	SenderID   SenderID `json:"sender_id"`
	SenderType string   `json:"sender_type"`
	TenantKey  string   `json:"tenant_key"`
}

// SenderID contains the user's ID information.
type SenderID struct {
	UnionID string `json:"union_id"`
	UserID  string `json:"user_id"`
	OpenID  string `json:"open_id"`
}

// MessageInfo contains details about the message.
type MessageInfo struct {
	MessageID   string    `json:"message_id"`
	RootID      string    `json:"root_id"`
	ParentID    string    `json:"parent_id"`
	CreateTime  string    `json:"create_time"`
	ChatID      string    `json:"chat_id"`
	ChatType    string    `json:"chat_type"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	Mentions    []Mention `json:"mentions"`
}

// Mention represents a user mentioned in a message.
type Mention struct {
	Key       string   `json:"key"`
	ID        SenderID `json:"id"`
	Name      string   `json:"name"`
	TenantKey string   `json:"tenant_key"`
}

// TextContent represents a simple text message content.
type TextContent struct {
	Text string `json:"text"`
}

// ImageContent represents an image message content with optional text.
type ImageContent struct {
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// ContentPart represents a content block in a multimodal message.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL with optional detail setting.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ParseMessageContent parses the message content string into content parts.
// It handles both simple text format {"text":"..."} and multimodal format
// with content parts like [{"type":"text","text":"..."}].
func ParseMessageContent(content string) ([]ContentPart, error) {
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}

	var parts []ContentPart

	// Check if it's already an array (multimodal format)
	if strings.HasPrefix(string(raw), "[") {
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil, err
		}
		return parts, nil
	}

	// Try parsing as simple text content
	var textContent TextContent
	if err := json.Unmarshal(raw, &textContent); err == nil && textContent.Text != "" {
		parts = append(parts, ContentPart{
			Type: "text",
			Text: textContent.Text,
		})
		return parts, nil
	}

	// Try parsing as image content
	var imageContent ImageContent
	if err := json.Unmarshal(raw, &imageContent); err == nil {
		if imageContent.Text != "" {
			parts = append(parts, ContentPart{
				Type: "text",
				Text: imageContent.Text,
			})
		}
		if imageContent.ImageURL != "" {
			parts = append(parts, ContentPart{
				Type: "image_url",
				ImageURL: &ImageURL{
					URL:    imageContent.ImageURL,
					Detail: "high",
				},
			})
		}
		return parts, nil
	}

	return nil, nil
}