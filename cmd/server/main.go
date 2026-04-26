package main

import (
	"context"
	"log"
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

var (
	configPath string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start Changate server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		r := router.Setup(cfg)

		srv := &http.Server{
			Addr:         cfg.Server.Address(),
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		}

		go func() {
			logger.SetLevel(cfg.LogLevel)
			logger.Info("Starting server on %s", cfg.Server.Address())
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start server: %v", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		logger.Info("Server exited")
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
