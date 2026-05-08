package router

import (
	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/handler"

	"github.com/gin-gonic/gin"
)

type RouterResult struct {
	Engine *gin.Engine
	Handler *handler.CallbackHandler
}

func Setup(cfg *config.Config) *RouterResult {
	r := gin.Default()

	agentClients := make(map[string]agent.Client)

	for i := range cfg.Apps {
		app := &cfg.Apps[i]
		client := agent.NewClient(
			app.Agent.BaseURL,
			app.Agent.APIPath,
			app.Agent.Model,
			app.Agent.Token,
			app.Agent.User,
			app.Agent.Conversation,
			app.Agent.Timeout,
			app.Agent.MaxRetries,
			app.Agent.RetryBaseDelay,
		)
		agentClients[app.Name] = client
	}

	callbackHandler := handler.NewCallbackHandler(cfg.Apps, agentClients)

	r.GET("/health", callbackHandler.HealthCheck)

	r.POST("/feishu/:appName", callbackHandler.HandleCallback)

	return &RouterResult{
		Engine: r,
		Handler: callbackHandler,
	}
}