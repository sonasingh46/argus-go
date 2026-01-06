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
	"argus-go/internal/queue"
	kafkaqueue "argus-go/internal/queue/kafka"
	memoryqueue "argus-go/internal/queue/memory"
	"argus-go/internal/store"
	memorystor "argus-go/internal/store/memory"
	postgresstor "argus-go/internal/store/postgres"
	redisstor "argus-go/internal/store/redis"
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

	logger.Info("configuration loaded",
		"path", *configPath,
		"storage_mode", cfg.Storage.Mode,
	)

	// Initialize dependencies based on storage mode
	deps, cleanup, err := initDependencies(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
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
		"storage_mode", cfg.Storage.Mode,
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

// initDependencies creates and wires all service dependencies based on config.
// Returns the dependencies and a cleanup function.
func initDependencies(cfg *config.Config, logger *slog.Logger) (*dependencies, func(), error) {
	var (
		stateStore       store.StateStore
		alertRepo        store.AlertRepository
		eventManagerRepo store.EventManagerRepository
		groupingRuleRepo store.GroupingRuleRepository
		producer         queue.Producer
		consumer         queue.Consumer
		cleanupFuncs     []func()
	)

	if cfg.Storage.UseMemory() {
		// Initialize in-memory implementations
		logger.Info("initializing in-memory storage")

		memStateStore := memorystor.NewStateStore()
		stateStore = memStateStore
		cleanupFuncs = append(cleanupFuncs, func() { _ = memStateStore.Close() })

		alertRepo = memorystor.NewAlertRepository()
		eventManagerRepo = memorystor.NewEventManagerRepository()
		groupingRuleRepo = memorystor.NewGroupingRuleRepository()

		memQueue := memoryqueue.NewQueue(10000)
		producer = memQueue
		consumer = memQueue
		cleanupFuncs = append(cleanupFuncs, func() { _ = memQueue.Close() })
	} else {
		// Initialize real storage implementations
		logger.Info("initializing production storage (Kafka, Redis, PostgreSQL)")

		// Initialize PostgreSQL
		ctx := context.Background()
		db, err := postgresstor.NewDB(ctx, &cfg.Postgres)
		if err != nil {
			return nil, nil, err
		}
		cleanupFuncs = append(cleanupFuncs, db.Close)

		// Run migrations
		if err := db.RunMigrations(ctx); err != nil {
			return nil, nil, err
		}
		logger.Info("database migrations completed")

		alertRepo = postgresstor.NewAlertRepository(db)
		eventManagerRepo = postgresstor.NewEventManagerRepository(db)
		groupingRuleRepo = postgresstor.NewGroupingRuleRepository(db)

		// Initialize Redis
		redisStore, err := redisstor.NewStateStore(&cfg.Redis)
		if err != nil {
			return nil, nil, err
		}
		stateStore = redisStore
		cleanupFuncs = append(cleanupFuncs, func() { _ = redisStore.Close() })

		// Initialize Kafka
		kafkaProducer := kafkaqueue.NewProducer(&cfg.Kafka)
		producer = kafkaProducer
		cleanupFuncs = append(cleanupFuncs, func() { _ = kafkaProducer.Close() })

		kafkaConsumer := kafkaqueue.NewConsumer(&cfg.Kafka, logger)
		consumer = kafkaConsumer
		cleanupFuncs = append(cleanupFuncs, func() { _ = kafkaConsumer.Close() })
	}

	// Initialize notification service (stubbed for now)
	notifier := notification.NewStubNotifier(logger)

	// Initialize ingest service
	ingestService := ingest.NewService(
		producer,
		eventManagerRepo,
		groupingRuleRepo,
		logger,
	)

	// Initialize processor service
	processorService := processor.NewService(
		consumer,
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

	// Build cleanup function
	cleanup := func() {
		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	}

	return &dependencies{
		server:    server,
		processor: processorService,
	}, cleanup, nil
}

// initLogger creates and configures the application logger.
func initLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
