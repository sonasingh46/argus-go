package api

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"argus-go/internal/config"
)

// Server represents the HTTP server with all configured routes and middleware.
type Server struct {
	app    *fiber.App
	config *config.ServerConfig
	logger *slog.Logger

	// Handlers
	eventManagerHandler *EventManagerHandler
	groupingRuleHandler *GroupingRuleHandler
	alertHandler        *AlertHandler
	ingestHandler       *IngestHandler
}

// ServerDeps contains all dependencies required to create a new Server.
type ServerDeps struct {
	Config              *config.ServerConfig
	Logger              *slog.Logger
	EventManagerHandler *EventManagerHandler
	GroupingRuleHandler *GroupingRuleHandler
	AlertHandler        *AlertHandler
	IngestHandler       *IngestHandler
}

// NewServer creates a new HTTP server with all routes configured.
func NewServer(deps ServerDeps) *Server {
	// Create Fiber app with optimized settings for high throughput
	app := fiber.New(fiber.Config{
		// Disable startup message for cleaner logs
		DisableStartupMessage: true,
		// Enable strict routing for consistency
		StrictRouting: true,
		// Case sensitive routing
		CaseSensitive: true,
		// Read timeout from config
		ReadTimeout: deps.Config.ReadTimeout,
		// Write timeout from config
		WriteTimeout: deps.Config.WriteTimeout,
		// Idle timeout from config
		IdleTimeout: deps.Config.IdleTimeout,
		// Custom error handler
		ErrorHandler: customErrorHandler,
	})

	s := &Server{
		app:                 app,
		config:              deps.Config,
		logger:              deps.Logger,
		eventManagerHandler: deps.EventManagerHandler,
		groupingRuleHandler: deps.GroupingRuleHandler,
		alertHandler:        deps.AlertHandler,
		ingestHandler:       deps.IngestHandler,
	}

	// Register middleware
	s.registerMiddleware()

	// Register routes
	s.registerRoutes()

	return s
}

// registerMiddleware sets up all middleware for the server.
func (s *Server) registerMiddleware() {
	// Recovery middleware to handle panics
	s.app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	// Request ID middleware for tracing
	s.app.Use(requestid.New())

	// Logger middleware for request logging
	s.app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path} | ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
}

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes() {
	// Health check endpoint (outside versioned API)
	s.app.Get("/healthz", s.healthCheck)

	// Prometheus metrics endpoint
	s.app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// API v1 routes
	v1 := s.app.Group("/v1")

	// Event ingestion
	v1.Post("/events", s.ingestHandler.IngestEvent)

	// Event Manager CRUD
	v1.Post("/event-managers", s.eventManagerHandler.Create)
	v1.Get("/event-managers", s.eventManagerHandler.List)
	v1.Get("/event-managers/:id", s.eventManagerHandler.GetByID)
	v1.Put("/event-managers/:id", s.eventManagerHandler.Update)
	v1.Delete("/event-managers/:id", s.eventManagerHandler.Delete)

	// Grouping Rules CRUD
	v1.Post("/grouping-rules", s.groupingRuleHandler.Create)
	v1.Get("/grouping-rules", s.groupingRuleHandler.List)
	v1.Get("/grouping-rules/:id", s.groupingRuleHandler.GetByID)
	v1.Put("/grouping-rules/:id", s.groupingRuleHandler.Update)
	v1.Delete("/grouping-rules/:id", s.groupingRuleHandler.Delete)

	// Alerts (read-only for MVP)
	v1.Get("/alerts", s.alertHandler.List)
	v1.Get("/alerts/:dedupKey", s.alertHandler.GetByDedupKey)
	v1.Get("/alerts/:dedupKey/children", s.alertHandler.GetChildren)
}

// healthCheck returns the health status of the service.
func (s *Server) healthCheck(c *fiber.Ctx) error {
	return Success(c, map[string]string{
		"status": "healthy",
	})
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	addr := s.config.Address()
	s.logger.Info("starting HTTP server", "address", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.app.ShutdownWithContext(ctx)
}

// customErrorHandler handles errors returned from handlers.
func customErrorHandler(c *fiber.Ctx, err error) error {
	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		return Error(c, e.Code, ErrCodeInternalError, e.Message)
	}

	// Default to internal server error
	return InternalError(c, fmt.Sprintf("unexpected error: %v", err))
}
