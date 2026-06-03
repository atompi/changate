// Package model provides data types for Feishu event callbacks and agent messages.
package model

import (
	"encoding/json"
	"fmt"
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
	Schema string       `json:"schema"`
	Header EventHeader  `json:"header"`
	Event  MessageEvent `json:"event"`
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

// SenderID identifies the message sender.
type SenderID struct {
	UnionID string `json:"union_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	OpenID  string `json:"open_id,omitempty"`
}

// MessageInfo contains the message content and metadata.
type MessageInfo struct {
	MessageID   string    `json:"message_id"`
	ChatID      string    `json:"chat_id"`
	ChatType    string    `json:"chat_type"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	Mentions    []Mention `json:"mentions,omitempty"`
}

// IsDM reports whether the message was sent in a 1-on-1 (direct) chat.
func (m *MessageInfo) IsDM() bool {
	return m.ChatType == "dm" || m.ChatType == "p2p"
}

// Mention is a Feishu @mention entry. The Key is the in-text placeholder
// (e.g. "@_user_1") that should be stripped from text before forwarding to
// the Agent; ID identifies the mentioned user or bot; Type discriminates
// "user" vs "bot"; Name is the display name (matched against app.bot_name
// to detect a mention of this app's bot).
type Mention struct {
	Key       string    `json:"key"`
	ID        MentionID `json:"id"`
	Name      string    `json:"name"`
	TenantKey string    `json:"tenant_key"`
	Type      string    `json:"mentioned_type,omitempty"`
}

// MentionID is the identity block inside a Mention. Feishu only populates
// open_id/user_id/union_id here; the mention kind ("user" vs "bot") lives on
// the parent Mention.Type, and the bot's display name lives on Mention.Name.
type MentionID struct {
	OpenID  string `json:"open_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	UnionID string `json:"union_id,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseMessageContent parses message content into content parts based on message type.
// Supported types: text, image, post
func ParseMessageContent(content string, messageType string) ([]MessageContentPart, error) {
	switch messageType {
	case "text":
		var text struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &text); err != nil {
			return nil, fmt.Errorf("failed to parse text content: %w", err)
		}
		if text.Text == "" {
			return nil, fmt.Errorf("empty text content")
		}
		return []MessageContentPart{{
			Type: "input_text",
			Text: text.Text,
		}}, nil

	case "image":
		var img struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(content), &img); err != nil {
			return nil, fmt.Errorf("failed to parse image content: %w", err)
		}
		if img.ImageKey == "" {
			return nil, fmt.Errorf("empty image_key in image message")
		}
		return []MessageContentPart{{
			Type: "input_image",
			Key:  img.ImageKey,
		}}, nil

	case "post":
		var richText struct {
			Content []any `json:"content"`
		}
		if err := json.Unmarshal([]byte(content), &richText); err != nil {
			return nil, fmt.Errorf("failed to parse post content: %w", err)
		}
		if len(richText.Content) == 0 {
			return nil, fmt.Errorf("empty post content")
		}
		parts := []MessageContentPart{}
		for _, blockIfc := range richText.Content {
			block, ok := blockIfc.([]any)
			if !ok {
				continue
			}
			for _, itemIfc := range block {
				item, ok := itemIfc.(map[string]any)
				if !ok {
					continue
				}
				tagVal := item["tag"]
				tag, ok := tagVal.(string)
				if !ok {
					continue
				}
				switch tag {
				case "img":
					keyVal := item["image_key"]
					imageKey, ok := keyVal.(string)
					if ok && imageKey != "" {
						parts = append(parts, MessageContentPart{
							Type: "input_image",
							Key:  imageKey,
						})
					}
				case "text":
					textVal := item["text"]
					text, ok := textVal.(string)
					if ok && text != "" {
						parts = append(parts, MessageContentPart{
							Type: "input_text",
							Text: text,
						})
					}
				}
			}
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("no valid content in post message")
		}
		return parts, nil

	default:
		return nil, fmt.Errorf("unsupported message type: %s", messageType)
	}
}
