package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/feishu"
	"github.com/atompi/changate/internal/model"
	"github.com/atompi/changate/pkg/crypto"
	"github.com/atompi/changate/pkg/logger"

	"github.com/gin-gonic/gin"
)

type CallbackHandler struct {
	apps         map[string]*config.AppConfig
	agentClients map[string]agent.Client
	semaphores   map[string]chan struct{}
	active       atomic.Int64
}

func NewCallbackHandler(apps []config.AppConfig, agentClients map[string]agent.Client) *CallbackHandler {
	appMap := make(map[string]*config.AppConfig)
	semaphores := make(map[string]chan struct{})
	for i := range apps {
		app := apps[i]
		appMap[app.Name] = &app

		maxConcurrent := app.MaxConcurrent
		if maxConcurrent <= 0 {
			maxConcurrent = 100
		}
		semaphores[app.Name] = make(chan struct{}, maxConcurrent)
	}

	return &CallbackHandler{
		apps:         appMap,
		agentClients: agentClients,
		semaphores:   semaphores,
	}
}

func (h *CallbackHandler) getFeishuClient(app *config.AppConfig) *feishu.Client {
	return feishu.NewClient(app.AppID, app.AppSecret, app.FeishuBaseURL)
}

func (h *CallbackHandler) HandleCallback(c *gin.Context) {
	appName := c.Param("appName")

	app, ok := h.apps[appName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	logger.Debug("[feishu callback] app=%s body=%s", appName, string(body))

	if app.EncryptKey != "" {
		signature := c.GetHeader("X-Lark-Signature")
		timestamp := c.GetHeader("X-Lark-Request-Timestamp")
		nonce := c.GetHeader("X-Lark-Request-Nonce")

		logger.Debug("[feishu callback] signature verification: timestamp=%s nonce=%s signature=%s", timestamp, nonce, signature)

		if !h.verifySignature(timestamp, nonce, signature, string(body), app.EncryptKey) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
			return
		}

		body, err = h.decryptBody(body, app.EncryptKey)
		if err != nil {
			logger.Error("decrypt error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
			return
		}
		logger.Debug("[feishu callback] app=%s decrypted_body=%s", appName, string(body))
	}

	var challengeReq model.URLVerificationRequest
	if err := json.Unmarshal(body, &challengeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if challengeReq.Type == "url_verification" {
		if app.VerifyToken != "" && challengeReq.Token != app.VerifyToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
			return
		}
		h.handleURLVerification(c, body)
		return
	}

	var eventReq model.EventCallbackRequest
	if err := json.Unmarshal(body, &eventReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if app.VerifyToken != "" && eventReq.Header.Token != app.VerifyToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
		return
	}

	switch eventReq.Header.EventType {
	case "im.message.receive_v1":
		h.handleMessageEvent(c, app, eventReq.Event)
	default:
		c.JSON(http.StatusOK, gin.H{"code": 0})
	}
}

func (h *CallbackHandler) decryptBody(body []byte, encryptKey string) ([]byte, error) {
	var encryptReq struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(body, &encryptReq); err != nil {
		return nil, err
	}

	if encryptReq.Encrypt == "" {
		return body, nil
	}

	decrypted, err := crypto.DecryptAES256CBC(encryptReq.Encrypt, encryptKey)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

func (h *CallbackHandler) verifySignature(timestamp, nonce, signature, body, encryptKey string) bool {
	if encryptKey == "" {
		return true
	}

	if signature == "" || timestamp == "" || nonce == "" {
		return false
	}

	var b strings.Builder
	b.WriteString(timestamp)
	b.WriteString(nonce)
	b.WriteString(encryptKey)
	b.WriteString(body)

	hash := sha256.New()
	hash.Write([]byte(b.String()))
	expectedSig := hex.EncodeToString(hash.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

func (h *CallbackHandler) handleURLVerification(c *gin.Context, body []byte) {
	var req model.URLVerificationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	c.JSON(http.StatusOK, model.URLVerificationResponse{
		Challenge: req.Challenge,
	})
}

func (h *CallbackHandler) handleMessageEvent(c *gin.Context, app *config.AppConfig, eventData json.RawMessage) {
	var msgEvent model.MessageEvent
	if err := json.Unmarshal(eventData, &msgEvent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse event"})
		return
	}

	contentParts, err := model.ParseMessageContent(msgEvent.Message.Content, msgEvent.Message.MessageType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse content"})
		return
	}

	logger.Debug("[handleMessageEvent] parsed event: message_id=%s content_parts=%v", msgEvent.Message.MessageID, contentParts)

	c.JSON(http.StatusOK, gin.H{"code": 0})

	h.active.Add(1)
	go func() {
		defer h.active.Add(-1)
		h.processMessageAsync(app.Name, app, contentParts, msgEvent.Message.MessageID)
	}()
}

func (h *CallbackHandler) processMessageAsync(appName string, app *config.AppConfig, contentParts []model.ContentPart, messageID string) {
	h.active.Add(1)
	semaphore := h.semaphores[appName]
	semaphore <- struct{}{}
	go func() {
		defer func() {
			h.active.Add(-1)
			<-semaphore
		}()

		agentClient, ok := h.agentClients[appName]
		if !ok {
			logger.Error("agent client not found for app: %s", appName)
			return
		}

		timeout := app.Agent.Timeout
		if timeout == 0 {
			timeout = 120 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		feishuClient := h.getFeishuClient(app)
		tenantToken, err := feishuClient.GetTenantAccessToken(ctx)
		if err != nil {
			logger.Error("failed to get tenant access token: %v", err)
			return
		}

		processedParts := make([]model.ContentPart, len(contentParts))
		for i, part := range contentParts {
			processedParts[i] = part
			if part.Type == "input_image" {
				imageKey := part.ImageURL
				imageData, err := feishuClient.DownloadMessageResource(ctx, tenantToken, messageID, imageKey)
				if err != nil {
					logger.Error("failed to download feishu image: %v", err)
					continue
				}
				base64Data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData)
				processedParts[i] = model.ContentPart{
					Type:     "input_image",
					ImageURL: base64Data,
				}
			}
		}

		agentResp, err := agentClient.ResponsesWithContent(ctx, processedParts)
		if err != nil {
			logger.Error("agent API error: %v", err)
			return
		}

		replyContent := agentResp.GetContent()
		filePath := agentResp.GetFilePath()

		accessToken, err := feishuClient.GetAppAccessToken(ctx)
		if err != nil {
			logger.Error("failed to get access token: %v", err)
			return
		}

		if filePath != "" {
			logger.Debug("[processMessageAsync] sending image response: url=%s", filePath)
			if err := feishuClient.SendFileMessage(ctx, accessToken, messageID, filePath); err != nil {
				logger.Error("failed to send image reply: %v, falling back to text", err)
				if replyContent != "" {
					if err := feishuClient.SendTextMessage(ctx, accessToken, messageID, replyContent); err != nil {
						logger.Error("failed to send text reply: %v", err)
						return
					}
				}
			} else {
				logger.Info("image reply sent successfully: message_id=%s", messageID)
				if replyContent != "" {
					if err := feishuClient.SendTextMessage(ctx, accessToken, messageID, replyContent); err != nil {
						logger.Error("failed to send text reply: %v", err)
						return
					}
				}
			}
		} else if replyContent != "" {
			if err := feishuClient.SendTextMessage(ctx, accessToken, messageID, replyContent); err != nil {
				logger.Error("failed to send reply: %v", err)
				return
			}
		} else {
			logger.Error("empty agent response")
			return
		}

		logger.Info("async message processed successfully: message_id=%s", messageID)
	}()
}

func (h *CallbackHandler) WaitForCompletion() {
	for h.active.Load() > 0 {
		time.Sleep(10 * time.Millisecond)
	}
}
