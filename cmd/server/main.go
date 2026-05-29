package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/router"
	"github.com/atompi/changate/pkg/logger"

	"github.com/spf13/cobra"
)

var configPath string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start Changate server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(configPath)
		if err != nil {
			slog.Error(logger.LogFormatter("Failed to load config: %v", err))
		}

		logCfg := config.NewLogConfig(cfg.Log)
		logLogger := logger.Init(logCfg.ToOptions()...)
		slog.SetDefault(logLogger)

		result := router.Setup(cfg)

		srv := &http.Server{
			Addr:         cfg.Server.Address(),
			Handler:      result.Engine,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		}

		go func() {
			slog.Info(logger.LogFormatter("Starting server on %s", cfg.Server.Address()))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error(logger.LogFormatter("Failed to start server: %v", err))
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info(logger.LogFormatter("Shutting down server..."))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error(logger.LogFormatter("Server forced to shutdown: %v", err))
		}

		slog.Info(logger.LogFormatter("Waiting for in-flight requests..."))
		result.Handler.WaitForCompletion()

		slog.Info(logger.LogFormatter("Server exited"))
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVar(&configPath, "config", "config/config.yaml", "path to config file")
}

var rootCmd = &cobra.Command{
	Use:   "changate",
	Short: "Channel gateway for Feishu and Hermes agent",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
