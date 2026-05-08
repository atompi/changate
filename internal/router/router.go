package router

import (
	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/handler"
	"github.com/atompi/changate/internal/hermes"
	"github.com/atompi/changate/internal/openclaw"

	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	agentClients := make(map[string]agent.Client)

	for i := range cfg.Apps {
		app := &cfg.Apps[i]
		var client agent.Client

		switch app.Agent.Platform {
		case "openclaw":
			client = openclaw.NewClient(
				app.Agent.BaseURL,
				app.Agent.APIPath,
				app.Agent.Model,
				app.Agent.Token,
				app.Agent.User,
				app.Agent.Timeout,
			)
		default:
			client = hermes.NewClient(
				app.Agent.BaseURL,
				app.Agent.APIPath,
				app.Agent.Model,
				app.Agent.Token,
				app.Agent.User,
				app.Agent.Timeout,
			)
		}

		agentClients[app.Name] = client
	}

	callbackHandler := handler.NewCallbackHandler(cfg.Apps, agentClients)

	r.GET("/health", callbackHandler.HealthCheck)

	r.POST("/feishu/:appName", callbackHandler.HandleCallback)

	return r
}