// Package feishu provides a client for interacting with the Feishu/Lark Open Platform API.
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atompi/changate/pkg/logger"
)

// Client is used to interact with Feishu Open Platform APIs.
type Client struct {
	appID     string
	appSecret string
	client    *http.Client
	baseURL   string
}

// NewClient creates a new Feishu API client.
func NewClient(appID, appSecret, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://open.feishu.cn"
	}
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// ReplyMessageReq is the request body for replying to a message.
type ReplyMessageReq struct {
	MsgType string `json:"msg_type"`
	Content string `json:"content"`
}

// ReplyMessageResp is the response from the reply message API.
type ReplyMessageResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ReplyMessage sends a reply to a specific message in a Feishu chat.
func (c *Client) ReplyMessage(ctx context.Context, accessToken, messageID, msgType, content string) error {
	url := fmt.Sprintf("%s/open-apis/im/v1/messages/%s/reply", c.baseURL, messageID)

	reqBody := ReplyMessageReq{
		MsgType: msgType,
		Content: content,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debug("[feishu reply] url=%s body=%s", url, string(jsonBody))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("[feishu reply response] status=%d body=%s", resp.StatusCode, string(respBody))
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var replyResp ReplyMessageResp
	if err := json.Unmarshal(respBody, &replyResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if replyResp.Code != 0 {
		return fmt.Errorf("feishu API error: code=%d, msg=%s", replyResp.Code, replyResp.Msg)
	}

	return nil
}

// SendTextMessage sends a text message reply to a Feishu message.
func (c *Client) SendTextMessage(ctx context.Context, accessToken, messageID, text string) error {
	contentMap := map[string]string{"text": text}
	contentBytes, err := json.Marshal(contentMap)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	return c.ReplyMessage(ctx, accessToken, messageID, "text", string(contentBytes))
}

// SendInteractiveMessage sends an interactive card message reply to a Feishu message.
func (c *Client) SendInteractiveMessage(ctx context.Context, accessToken, messageID, cardContent string) error {
	contentMap := map[string]string{"card": cardContent}
	contentBytes, err := json.Marshal(contentMap)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	return c.ReplyMessage(ctx, accessToken, messageID, "interactive", string(contentBytes))
}

// GetAccessTokenReq is the request body for getting an app access token.
type GetAccessTokenReq struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// GetAccessTokenResp is the response from the app access token API.
type GetAccessTokenResp struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	AppAccessToken    string `json:"app_access_token"`
	Expire            int    `json:"expire"`
	Retry             bool   `json:"retry"`
	TenantAccessToken string `json:"tenant_access_token"`
	TokenType         string `json:"token_type"`
}

// GetAppAccessToken obtains an app access token from Feishu.
// The token is required for making API calls to Feishu.
func (c *Client) GetAppAccessToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/open-apis/auth/v3/app_access_token/internal", c.baseURL)

	reqBody := GetAccessTokenReq{
		AppID:     c.appID,
		AppSecret: c.appSecret,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResp GetAccessTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("feishu API error: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.AppAccessToken, nil
}