package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"argus-go/internal/domain"
	"argus-go/internal/ingest"
)

// IngestHandler handles HTTP requests for event ingestion.
type IngestHandler struct {
	service *ingest.Service
	logger  *slog.Logger
}

// NewIngestHandler creates a new ingest handler.
func NewIngestHandler(service *ingest.Service, logger *slog.Logger) *IngestHandler {
	return &IngestHandler{
		service: service,
		logger:  logger,
	}
}

// IngestEvent handles POST /v1/events
// Receives an event, validates it, and publishes to the message queue.
// Returns 202 Accepted immediately - processing happens asynchronously.
func (h *IngestHandler) IngestEvent(c *fiber.Ctx) error {
	var event domain.Event
	if err := c.BodyParser(&event); err != nil {
		h.logger.Debug("failed to parse event body", "error", err)
		return BadRequest(c, "invalid request body")
	}

	// Validate the event
	if err := event.Validate(); err != nil {
		h.logger.Debug("event validation failed", "error", err)
		return ValidationError(c, err.Error())
	}

	// Submit event for processing
	if err := h.service.IngestEvent(c.Context(), &event); err != nil {
		h.logger.Error("failed to ingest event", "error", err, "dedupKey", event.DedupKey)
		return InternalError(c, "failed to ingest event")
	}

	h.logger.Debug("event accepted", "dedupKey", event.DedupKey, "action", event.Action)

	// Return 202 Accepted - event will be processed asynchronously
	return Accepted(c, map[string]string{
		"status":   "accepted",
		"dedupKey": event.DedupKey,
	})
}
