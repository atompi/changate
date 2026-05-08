// Package feishu provides a client for interacting with the Feishu/Lark Open Platform API.
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/atompi/changate/pkg/logger"
	"github.com/atompi/changate/pkg/retry"
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

// replyMessageReq is the request body for replying to a message.
type replyMessageReq struct {
	MsgType string `json:"msg_type"`
	Content string `json:"content"`
}

// replyMessageResp is the response from the reply message API.
type replyMessageResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ReplyMessage sends a reply to a specific message in a Feishu chat.
func (c *Client) ReplyMessage(ctx context.Context, accessToken, messageID, msgType, content string) error {
	url := fmt.Sprintf("%s/open-apis/im/v1/messages/%s/reply", c.baseURL, messageID)

	reqBody := replyMessageReq{
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

	var resp *http.Response
	err = retry.Do(ctx, retry.Config{
		MaxRetries: 2,
		BaseDelay:  100 * time.Millisecond,
		BeforeRetry: func(attempt int, delay time.Duration) {
			logger.Debug("[feishu reply] retrying in %v (attempt %d/3)", delay, attempt+1)
		},
	}, func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			logger.Debug("[feishu reply] request failed: %v", err)
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		logger.Debug("[feishu reply response] status=%d body=%s", resp.StatusCode, string(respBody))
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode < 500 {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d", retry.ErrTransient, resp.StatusCode)
	})
	if err != nil {
		return fmt.Errorf("failed to send request after 3 attempts: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var replyResp replyMessageResp
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

func (c *Client) SendFileMessage(ctx context.Context, appAccessToken, messageID, filePath string) error {
	var fileData []byte
	var err error

	tenantToken, err := c.GetTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}
	fileData, err = os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fileName := filePath
	fileType := "stream"

	fileKey, err := c.UploadMessageResource(ctx, tenantToken, fileData, fileName, fileType)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	content := map[string]string{"file_key": fileKey}
	contentBytes, _ := json.Marshal(content)
	return c.ReplyMessage(ctx, appAccessToken, messageID, "file", string(contentBytes))
}

// getAccessTokenReq is the request body for getting an app access token.
type getAccessTokenReq struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// getAccessTokenResp is the response from the app access token API.
type getAccessTokenResp struct {
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

	reqBody := getAccessTokenReq{
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

	var tokenResp getAccessTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("feishu API error: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.AppAccessToken, nil
}

func (c *Client) GetTenantAccessToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/open-apis/auth/v3/tenant_access_token/internal", c.baseURL)

	reqBody := getAccessTokenReq{
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

	var tokenResp getAccessTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("feishu API error: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.TenantAccessToken, nil
}

func (c *Client) UploadMessageResource(ctx context.Context, accessToken string, fileData []byte, fileName, fileType string) (string, error) {
	url := fmt.Sprintf("%s/open-apis/im/v1/files", c.baseURL)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	writer.WriteField("file_name", fileName)
	writer.WriteField("file_type", fileType)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	part.Write(fileData)

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	logger.Debug("[feishu upload message resource] url=%s file_name=%s file_type=%s size=%d", url, fileName, fileType, len(fileData))

	var resp *http.Response
	err = retry.Do(ctx, retry.Config{
		MaxRetries: 2,
		BaseDelay:  100 * time.Millisecond,
	}, func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			return fmt.Errorf("%w: failed to send request: %w", retry.ErrTransient, err)
		}

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: status %d", retry.ErrTransient, resp.StatusCode)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload message resource after retries: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("[feishu upload message resource response] status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed: status %d", resp.StatusCode)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("feishu error: code=%d, msg=%s", result.Code, result.Msg)
	}

	return result.Data.FileKey, nil
}

// DownloadMessageResource downloads a resource file from a Feishu message.
// Use this for images in messages - it requires message_id, file_key, and type query parameter.
func (c *Client) DownloadMessageResource(ctx context.Context, accessToken, messageID, fileKey string) ([]byte, error) {
	url := fmt.Sprintf("%s/open-apis/im/v1/messages/%s/resources/%s?type=image", c.baseURL, messageID, fileKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	logger.Debug("[feishu download message resource] url=%s message_id=%s file_key=%s", url, messageID, fileKey)

	var resp *http.Response
	err = retry.Do(ctx, retry.Config{
		MaxRetries: 2,
		BaseDelay:  100 * time.Millisecond,
	}, func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			return fmt.Errorf("%w: failed to send request: %w", retry.ErrTransient, err)
		}
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: status %d", retry.ErrTransient, resp.StatusCode)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download message resource after retries: %w", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource data: %w", err)
	}

	defer resp.Body.Close()

	logger.Debug("[feishu download message resource] downloaded size=%d", len(data))
	return data, nil
}
