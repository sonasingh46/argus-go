// Package main is the entry point for the ArgusGo alert management service.
// It initializes all components and starts the HTTP server and event processor.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"argus-go/internal/api"
	"argus-go/internal/config"
	"argus-go/internal/ingest"
	"argus-go/internal/notification"
	"argus-go/internal/processor"
	"argus-go/internal/queue/memory"
	storemem "argus-go/internal/store/memory"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config/config.yaml", "path to configuration file")
	flag.Parse()

	// Initialize logger
	logger := initLogger()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err, "path", *configPath)
		os.Exit(1)
	}

	logger.Info("configuration loaded", "path", *configPath)

	// Initialize dependencies
	deps, cleanup := initDependencies(cfg, logger)
	defer cleanup()

	// Create context that listens for shutdown signals
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start processor in background
	go func() {
		if err := deps.processor.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("processor error", "error", err)
			cancel()
		}
	}()

	// Start HTTP server
	go func() {
		if err := deps.server.Start(); err != nil {
			logger.Error("server error", "error", err)
			cancel()
		}
	}()

	logger.Info("ArgusGo started",
		"address", cfg.Server.Address(),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.WriteTimeout)
	defer shutdownCancel()

	if err := deps.server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	if err := deps.processor.Stop(); err != nil {
		logger.Error("processor shutdown error", "error", err)
	}

	logger.Info("ArgusGo stopped")
}

// dependencies holds all initialized service dependencies.
type dependencies struct {
	server    *api.Server
	processor *processor.Service
}

// initDependencies creates and wires all service dependencies.
// Returns the dependencies and a cleanup function.
func initDependencies(cfg *config.Config, logger *slog.Logger) (*dependencies, func()) {
	// Initialize in-memory stores (for MVP)
	// In production, these would be Redis/PostgreSQL implementations
	stateStore := storemem.NewStateStore()
	alertRepo := storemem.NewAlertRepository()
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()

	// Initialize in-memory queue (for MVP)
	// In production, this would be Kafka
	messageQueue := memory.NewQueue(10000) // 10k message buffer

	// Initialize notification service (stubbed for MVP)
	notifier := notification.NewStubNotifier(logger)

	// Initialize ingest service
	ingestService := ingest.NewService(
		messageQueue,
		eventManagerRepo,
		groupingRuleRepo,
		logger,
	)

	// Initialize processor service
	processorService := processor.NewService(
		messageQueue,
		stateStore,
		alertRepo,
		eventManagerRepo,
		groupingRuleRepo,
		notifier,
		logger,
	)

	// Initialize API handlers
	eventManagerHandler := api.NewEventManagerHandler(eventManagerRepo, logger)
	groupingRuleHandler := api.NewGroupingRuleHandler(groupingRuleRepo, logger)
	alertHandler := api.NewAlertHandler(alertRepo, logger)
	ingestHandler := api.NewIngestHandler(ingestService, logger)

	// Initialize HTTP server
	server := api.NewServer(api.ServerDeps{
		Config:              &cfg.Server,
		Logger:              logger,
		EventManagerHandler: eventManagerHandler,
		GroupingRuleHandler: groupingRuleHandler,
		AlertHandler:        alertHandler,
		IngestHandler:       ingestHandler,
	})

	// Return dependencies and cleanup function
	cleanup := func() {
		_ = messageQueue.Close()
		_ = stateStore.Close()
	}

	return &dependencies{
		server:    server,
		processor: processorService,
	}, cleanup
}

// initLogger creates and configures the application logger.
func initLogger() *slog.Logger {
	// Use JSON handler for structured logging
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
