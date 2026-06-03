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
	"log/slog"
	"net/http"
	"strings"
	"sync"
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
	etcdLoader *config.EtcdConfigLoader
	semaphores map[string]chan struct{}
	agentCache *AgentCache
	active     atomic.Int64
	mu         sync.Mutex
}

func NewCallbackHandler(etcdLoader *config.EtcdConfigLoader) *CallbackHandler {
	return &CallbackHandler{
		etcdLoader: etcdLoader,
		semaphores: make(map[string]chan struct{}),
		agentCache: NewAgentCache(1000, 30*time.Second),
	}
}

func (h *CallbackHandler) getSemaphore(appName string, maxConcurrent int) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	sem, ok := h.semaphores[appName]
	if !ok {
		if maxConcurrent <= 0 {
			maxConcurrent = 100
		}
		sem = make(chan struct{}, maxConcurrent)
		h.semaphores[appName] = sem
	}
	return sem
}

func (h *CallbackHandler) getFeishuClient(app *config.AppConfig) *feishu.Client {
	return feishu.NewClient(app.AppID, app.AppSecret, app.FeishuBaseURL)
}

func (h *CallbackHandler) HandleCallback(c *gin.Context) {
	appName := c.Param("appName")
	ctx := c.Request.Context()
	appCfg, err := h.etcdLoader.GetAppConfigOnly(ctx, appName)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "app config lookup failed"):
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		case strings.Contains(errMsg, "app is disabled"):
			c.JSON(http.StatusForbidden, gin.H{"error": "app is disabled"})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
		}
		return
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error(logger.LogFormatter("failed to read request body: %v", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	body := rawBody
	if appCfg.EncryptKey != "" {
		body, err = h.decryptBody(rawBody, appCfg.EncryptKey)
		if err != nil {
			slog.Error(logger.LogFormatter("decrypt error: %v", err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
			return
		}
	}

	var challengeReq model.URLVerificationRequest
	if err := json.Unmarshal(body, &challengeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if challengeReq.Type == "url_verification" {
		if appCfg.VerifyToken != "" && challengeReq.Token != appCfg.VerifyToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
			return
		}

		h.handleURLVerification(c, body)
		return
	}

	signature := c.GetHeader("X-Lark-Signature")
	timestamp := c.GetHeader("X-Lark-Request-Timestamp")
	nonce := c.GetHeader("X-Lark-Request-Nonce")

	if !h.verifySignature(timestamp, nonce, signature, string(rawBody), appCfg.EncryptKey) {
		slog.Error(logger.LogFormatter("[feishu callback] signature verification failed"))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
		return
	}

	var eventReq model.EventCallbackRequest
	if err := json.Unmarshal(body, &eventReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if appCfg.VerifyToken != "" && eventReq.Header.Token != appCfg.VerifyToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
		return
	}

	userID := eventReq.Event.Sender.SenderID.UserID

	resolvedCfg, err := h.etcdLoader.GetResolvedConfig(ctx, appName, userID)
	if err != nil {
		slog.Error(logger.LogFormatter("[feishu callback] app=%s user=%s error: %v", appName, userID, err))
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "app config lookup failed"):
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		case strings.Contains(errMsg, "app is disabled"):
			c.JSON(http.StatusForbidden, gin.H{"error": "app is disabled"})
		case strings.Contains(errMsg, "user is disabled"):
			c.JSON(http.StatusForbidden, gin.H{"error": "user is disabled"})
		case strings.Contains(errMsg, "no agent configured"):
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no agent configured"})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
		}
		return
	}

	switch eventReq.Header.EventType {
	case "im.message.receive_v1":
		h.handleMessageEvent(c, resolvedCfg, eventReq.Event, appName, userID)
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

func (h *CallbackHandler) handleMessageEvent(c *gin.Context, app *config.AppConfig, msgEvent model.MessageEvent, appName string, userID string) {
	if !shouldProcessMessage(msgEvent.Message, app.BotName) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}

	content := msgEvent.Message.Content
	if msgEvent.Message.MessageType == "text" {
		if key := model.BotMentionKey(msgEvent.Message.Mentions, app.BotName); key != "" {
			content = model.StripBotMention(content, key)
			slog.Info(logger.LogFormatter("[feishu callback] stripped bot mention %q from text in chat %s", key, msgEvent.Message.ChatID))
		}
	}

	contentParts, err := model.ParseMessageContent(content, msgEvent.Message.MessageType)
	if err != nil {
		slog.Error(logger.LogFormatter("failed to parse message content: %v", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0})

	h.active.Add(1)
	go func() {
		defer h.active.Add(-1)
		h.processMessageAsync(appName, app, userID, contentParts, msgEvent.Message.MessageID)
	}()
}

// shouldProcessMessage enforces the rule: in 1-on-1 chats, process every
// message; in non-DM chats (group/channel/thread), only process when a
// mention of type "bot" and Name matching botName is present in the
// message. Returning false lets the handler ack with code=0 without
// invoking the Agent.
func shouldProcessMessage(msg model.MessageInfo, botName string) bool {
	if msg.IsDM() {
		return true
	}
	if model.IsBotMentioned(msg.Mentions, botName) {
		return true
	}
	slog.Debug(logger.LogFormatter("[feishu callback] skipping non-mention in chat_type=%s chat_id=%s botName=%s mentions=%+v", msg.ChatType, msg.ChatID, botName, msg.Mentions))
	return false
}

func (h *CallbackHandler) processMessageAsync(appName string, app *config.AppConfig, userID string, contentParts []model.MessageContentPart, messageID string) {
	semaphore := h.getSemaphore(appName, app.MaxConcurrent)
	semaphore <- struct{}{}
	go func() {
		defer func() {
			h.active.Add(-1)
			<-semaphore
		}()

		agentTimeout := app.Agent.Timeout
		if agentTimeout == 0 {
			agentTimeout = 3600 * time.Second
		}
		agentRetryBaseDelay := app.Agent.RetryBaseDelay
		if agentRetryBaseDelay == 0 {
			agentRetryBaseDelay = 100 * time.Millisecond
		}
		appTimeout := app.Timeout
		if appTimeout == 0 {
			appTimeout = 5 * time.Second
		}
		agentMaxRetries := app.Agent.MaxRetries
		if agentMaxRetries == 0 {
			agentMaxRetries = 3
		}

		key := cacheKey{appName: appName, userID: userID}
		agentClient := h.agentCache.GetOrCreate(context.Background(), key, app, func(cfg *config.AppConfig) agent.Client {
			c, err := agent.NewClient(agent.Config{
				BaseURL:        cfg.Agent.BaseURL,
				APIPath:        cfg.Agent.APIPath,
				Model:          cfg.Agent.Model,
				Token:          cfg.Agent.Token,
				User:           userID,
				Timeout:        agentTimeout,
				MaxRetries:     agentMaxRetries,
				RetryBaseDelay: agentRetryBaseDelay,
				AgentType:      cfg.Agent.Type,
				SystemPrompt:   cfg.Agent.SystemPrompt,
				Tools:          cfg.Agent.Tools,
				ToolChoice:     cfg.Agent.ToolChoice,
			})
			if err != nil {
				slog.Error(logger.LogFormatter("failed to create agent client: %v", err))
				return nil
			}
			return c
		})
		if agentClient == nil {
			return
		}

		app1Ctx, app1Cancel := context.WithTimeout(context.Background(), appTimeout)
		defer app1Cancel()

		feishuClient := h.getFeishuClient(app)
		tenantToken, err := feishuClient.GetTenantAccessToken(app1Ctx)
		if err != nil {
			slog.Error(logger.LogFormatter("failed to get tenant access token: %v", err))
			return
		}

		agentCtx, agentCancel := context.WithTimeout(context.Background(), agentTimeout)
		defer agentCancel()

		replyContent := ""
		filePath := ""
		if app.Agent.Type == "ChatCompletions" {
			chatParts, err := processMessageContentPartsToChatCompletionsContentParts(app1Ctx, contentParts, feishuClient, messageID, tenantToken)
			if err != nil {
				slog.Error(logger.LogFormatter("failed to process content parts: %v", err))
				return
			}
			chatResp, err := agentClient.ChatCompletionsWithContent(agentCtx, chatParts)
			if err != nil {
				slog.Error(logger.LogFormatter("agent API error: %v", err))
				return
			}
			replyContent = chatResp.GetContent()
			filePath = chatResp.GetFilePath()
		} else {
			openParts, err := processMessageContentPartsToOpenResponsesContentParts(app1Ctx, contentParts, feishuClient, messageID, tenantToken)
			if err != nil {
				slog.Error(logger.LogFormatter("failed to process content parts: %v", err))
				return
			}
			openResp, err := agentClient.OpenResponsesWithContent(agentCtx, openParts)
			if err != nil {
				slog.Error(logger.LogFormatter("agent API error: %v", err))
				return
			}
			replyContent = openResp.GetContent()
			filePath = openResp.GetFilePath()
		}

		if replyContent == "" && filePath == "" {
			slog.Error(logger.LogFormatter("empty agent response"))
			return
		}

		app2Ctx, app2Cancel := context.WithTimeout(context.Background(), appTimeout)
		defer app2Cancel()

		accessToken, err := feishuClient.GetAppAccessToken(app2Ctx)
		if err != nil {
			slog.Error(logger.LogFormatter("failed to get access token: %v", err))
			return
		}

		if filePath != "" {
			slog.Debug(logger.LogFormatter("[processMessageAsync] sending image response: url=%s", filePath))
			if err := feishuClient.SendFileMessage(app2Ctx, accessToken, messageID, filePath); err != nil {
				slog.Error(logger.LogFormatter("failed to send image reply: %v, falling back to text", err))
			}
		}
		if replyContent != "" {
			if err := feishuClient.SendTextMessage(app2Ctx, accessToken, messageID, replyContent); err != nil {
				slog.Error(logger.LogFormatter("failed to send reply: %v", err))
				return
			}
		}

		slog.Info(logger.LogFormatter("async message processed successfully: message_id=%s", messageID))
	}()
}

func (h *CallbackHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *CallbackHandler) WaitForCompletion() {
	for h.active.Load() > 0 {
		time.Sleep(10 * time.Millisecond)
	}
}

func processMessageContentPartsToChatCompletionsContentParts(ctx context.Context, contentParts []model.MessageContentPart, feishuClient *feishu.Client, messageID string, tenantToken string) ([]model.ChatCompletionsContentPart, error) {
	processedParts := make([]model.ChatCompletionsContentPart, 0, len(contentParts))
	for _, part := range contentParts {
		if part.Type == "input_text" {
			processedParts = append(processedParts, model.ChatCompletionsContentPart{
				Type: "text",
				Text: part.Text,
			})
		} else if part.Type == "input_image" {
			imageData, err := feishuClient.DownloadMessageResource(ctx, tenantToken, messageID, part.Key)
			if err != nil {
				slog.Error(logger.LogFormatter("failed to download feishu image: %v", err))
				continue
			}
			processedParts = append(processedParts, model.ChatCompletionsContentPart{
				Type: "image_url",
				ImageURL: &model.ChatCompletionsImageURL{
					URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData),
				},
			})
		}
	}
	return processedParts, nil
}

func processMessageContentPartsToOpenResponsesContentParts(ctx context.Context, contentParts []model.MessageContentPart, feishuClient *feishu.Client, messageID string, tenantToken string) ([]model.OpenResponsesContentPart, error) {
	processedParts := make([]model.OpenResponsesContentPart, 0, len(contentParts))
	for _, part := range contentParts {
		if part.Type == "input_text" {
			processedParts = append(processedParts, model.OpenResponsesContentPart{
				Type: "input_text",
				Text: part.Text,
			})
		} else if part.Type == "input_image" {
			imageData, err := feishuClient.DownloadMessageResource(ctx, tenantToken, messageID, part.Key)
			if err != nil {
				slog.Error(logger.LogFormatter("failed to download feishu image: %v", err))
				continue
			}
			processedParts = append(processedParts, model.OpenResponsesContentPart{
				Type:      "input_image",
				ImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData),
			})
		}
	}
	return processedParts, nil
}
