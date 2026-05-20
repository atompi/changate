package router

import (
	"log/slog"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/etcd"
	"github.com/atompi/changate/internal/handler"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

type RouterResult struct {
	Engine  *gin.Engine
	Handler *handler.CallbackHandler
}

func Setup(cfg *config.Config) *RouterResult {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(sloggin.New(slog.Default()))
	r.Use(gin.Recovery())

	etcdClient, err := etcd.NewClient(&cfg.Etcd)
	if err != nil {
		panic("failed to create etcd client: " + err.Error())
	}

	etcdLoader := config.NewEtcdConfigLoader(etcdClient, cfg.Etcd.RootPath)

	callbackHandler := handler.NewCallbackHandler(etcdLoader)

	r.GET("/health", callbackHandler.HealthCheck)

	r.POST("/feishu/:appName", callbackHandler.HandleCallback)

	return &RouterResult{
		Engine:  r,
		Handler: callbackHandler,
	}
}
