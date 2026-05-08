package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

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
}

func NewCallbackHandler(apps []config.AppConfig, agentClients map[string]agent.Client) *CallbackHandler {
	appMap := make(map[string]*config.AppConfig)
	for i := range apps {
		appMap[apps[i].Name] = &apps[i]
	}

	return &CallbackHandler{
		apps:         appMap,
		agentClients: agentClients,
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
		body, err = h.decryptBody(body, app.EncryptKey)
		if err != nil {
			logger.Error("decrypt error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
			return
		}
		logger.Debug("[feishu callback] app=%s decrypted_body=%s", appName, string(body))
	}

	var baseReq struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &baseReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if app.VerifyToken != "" && baseReq.Token != "" && baseReq.Token != app.VerifyToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
		return
	}

	switch baseReq.Type {
	case "url_verification":
		h.handleURLVerification(c, body)
	case "":
		h.handleMessageEvent(c, app, body)
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

func (h *CallbackHandler) verifySignature(timestamp, signature, body, encryptKey string) bool {
	if encryptKey == "" {
		return true
	}

	mac := hmac.New(sha256.New, []byte(encryptKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(body))

	expectedSig := hex.EncodeToString(mac.Sum(nil))
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

func (h *CallbackHandler) handleMessageEvent(c *gin.Context, app *config.AppConfig, body []byte) {
	signature := c.GetHeader("X-Lark-Signature")
	timestamp := c.GetHeader("X-Lark-Timestamp")

	if signature != "" && timestamp != "" && !h.verifySignature(timestamp, signature, string(body), app.EncryptKey) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
		return
	}

	var event model.EventCallbackRequest
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if event.Header.EventType != "im.message.receive_v1" {
		c.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}

	var msgEvent model.MessageEvent
	if err := json.Unmarshal(event.Event, &msgEvent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse event"})
		return
	}

	contentParts, err := model.ParseMessageContent(msgEvent.Message.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0})

	go h.processMessageAsync(app.Name, app, contentParts, msgEvent.Message.MessageID)
}

func (h *CallbackHandler) processMessageAsync(appName string, app *config.AppConfig, contentParts interface{}, messageID string) {
	agentClient, ok := h.agentClients[appName]
	if !ok {
		logger.Error("agent client not found for app: %s", appName)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), app.Agent.Timeout)
	defer cancel()

	agentResp, err := agentClient.ChatCompletionWithContent(ctx, contentParts)
	if err != nil {
		logger.Error("agent API error: %v", err)
		return
	}

	replyContent := agent.GetContent(agentResp)
	if replyContent == "" {
		logger.Error("empty agent response")
		return
	}

	feishuClient := h.getFeishuClient(app)

	accessToken, err := feishuClient.GetAppAccessToken(ctx)
	if err != nil {
		logger.Error("failed to get access token: %v", err)
		return
	}

	if err := feishuClient.SendTextMessage(ctx, accessToken, messageID, replyContent); err != nil {
		logger.Error("failed to send reply: %v", err)
		return
	}

	logger.Info("async message processed successfully: message_id=%s", messageID)
}