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

	var agentClient agent.Client

	switch cfg.Agent.Platform {
	case "openclaw":
		agentClient = openclaw.NewClient(
			cfg.Agent.BaseURL,
			cfg.Agent.APIPath,
			cfg.Agent.Model,
			cfg.Agent.Token,
			cfg.Agent.User,
			cfg.Agent.Timeout,
		)
	default:
		agentClient = hermes.NewClient(
			cfg.Agent.BaseURL,
			cfg.Agent.APIPath,
			cfg.Agent.Model,
			cfg.Agent.Token,
			cfg.Agent.User,
			cfg.Agent.Timeout,
		)
	}

	callbackHandler := handler.NewCallbackHandler(cfg.Apps, agentClient, cfg.Agent.Timeout)

	r.GET("/health", callbackHandler.HealthCheck)

	r.POST("/feishu/:appName", callbackHandler.HandleCallback)

	return r
}
