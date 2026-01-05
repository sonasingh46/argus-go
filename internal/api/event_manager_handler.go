package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"argus-go/internal/domain"
	"argus-go/internal/store"
)

// EventManagerHandler handles HTTP requests for event manager operations.
type EventManagerHandler struct {
	repo   store.EventManagerRepository
	logger *slog.Logger
}

// NewEventManagerHandler creates a new event manager handler.
func NewEventManagerHandler(repo store.EventManagerRepository, logger *slog.Logger) *EventManagerHandler {
	return &EventManagerHandler{
		repo:   repo,
		logger: logger,
	}
}

// Create handles POST /v1/event-managers
// Creates a new event manager.
func (h *EventManagerHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateEventManagerRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Debug("failed to parse request body", "error", err)
		return BadRequest(c, "invalid request body")
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		h.logger.Debug("validation failed", "error", err)
		return ValidationError(c, err.Error())
	}

	// Generate ID and create the event manager
	id := uuid.New().String()
	em := req.ToEventManager(id)

	// Persist to repository
	if err := h.repo.Create(c.Context(), em); err != nil {
		if errors.Is(err, domain.ErrEventManagerAlreadyExists) {
			return Conflict(c, "event manager already exists")
		}
		h.logger.Error("failed to create event manager", "error", err)
		return InternalError(c, "failed to create event manager")
	}

	h.logger.Info("created event manager", "id", em.ID, "name", em.Name)
	return Created(c, em)
}

// List handles GET /v1/event-managers
// Returns all event managers.
func (h *EventManagerHandler) List(c *fiber.Ctx) error {
	eventManagers, err := h.repo.List(c.Context())
	if err != nil {
		h.logger.Error("failed to list event managers", "error", err)
		return InternalError(c, "failed to list event managers")
	}

	return Success(c, eventManagers)
}

// GetByID handles GET /v1/event-managers/:id
// Returns a single event manager by ID.
func (h *EventManagerHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	em, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrEventManagerNotFound) {
			return NotFound(c, "event manager not found")
		}
		h.logger.Error("failed to get event manager", "id", id, "error", err)
		return InternalError(c, "failed to get event manager")
	}

	return Success(c, em)
}

// Update handles PUT /v1/event-managers/:id
// Updates an existing event manager.
func (h *EventManagerHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	var req domain.UpdateEventManagerRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Debug("failed to parse request body", "error", err)
		return BadRequest(c, "invalid request body")
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		h.logger.Debug("validation failed", "error", err)
		return ValidationError(c, err.Error())
	}

	// Fetch existing event manager
	em, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrEventManagerNotFound) {
			return NotFound(c, "event manager not found")
		}
		h.logger.Error("failed to get event manager", "id", id, "error", err)
		return InternalError(c, "failed to get event manager")
	}

	// Apply updates
	req.ApplyTo(em)

	// Persist changes
	if err := h.repo.Update(c.Context(), em); err != nil {
		h.logger.Error("failed to update event manager", "id", id, "error", err)
		return InternalError(c, "failed to update event manager")
	}

	h.logger.Info("updated event manager", "id", em.ID)
	return Success(c, em)
}

// Delete handles DELETE /v1/event-managers/:id
// Deletes an event manager.
func (h *EventManagerHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	if err := h.repo.Delete(c.Context(), id); err != nil {
		if errors.Is(err, domain.ErrEventManagerNotFound) {
			return NotFound(c, "event manager not found")
		}
		h.logger.Error("failed to delete event manager", "id", id, "error", err)
		return InternalError(c, "failed to delete event manager")
	}

	h.logger.Info("deleted event manager", "id", id)
	return NoContent(c)
}
