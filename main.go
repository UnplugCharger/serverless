package main

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"youtube_serverless/config"
	"youtube_serverless/handlers"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()
	
	// Configure logging
	configureLogging(cfg.LogLevel)
	
	log.Info().Msg("Starting YouTube Serverless Platform")
	
	// Create server handler
	serverHandler := handlers.NewServerHandler(cfg)
	
	// Create server mux
	mux := http.NewServeMux()
	
	// Register routes
	serverHandler.RegisterRoutes(mux)
	
	// Create server with timeouts
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	
	// Start server in a goroutine
	go func() {
		log.Info().Msgf("Server listening on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()
	
	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Info().Msg("Shutting down server...")
	
	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	
	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	
	log.Info().Msg("Server exited properly")
}

// configureLogging sets up the logger based on the provided log level
func configureLogging(level string) {
	// Set up pretty console logging
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)
	
	// Set log level
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
