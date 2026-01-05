package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"argus-go/internal/domain"
	"argus-go/internal/store"
)

// GroupingRuleHandler handles HTTP requests for grouping rule operations.
type GroupingRuleHandler struct {
	repo   store.GroupingRuleRepository
	logger *slog.Logger
}

// NewGroupingRuleHandler creates a new grouping rule handler.
func NewGroupingRuleHandler(repo store.GroupingRuleRepository, logger *slog.Logger) *GroupingRuleHandler {
	return &GroupingRuleHandler{
		repo:   repo,
		logger: logger,
	}
}

// Create handles POST /v1/grouping-rules
// Creates a new grouping rule.
func (h *GroupingRuleHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateGroupingRuleRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Debug("failed to parse request body", "error", err)
		return BadRequest(c, "invalid request body")
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		h.logger.Debug("validation failed", "error", err)
		return ValidationError(c, err.Error())
	}

	// Generate ID and create the grouping rule
	id := uuid.New().String()
	rule := req.ToGroupingRule(id)

	// Persist to repository
	if err := h.repo.Create(c.Context(), rule); err != nil {
		h.logger.Error("failed to create grouping rule", "error", err)
		return InternalError(c, "failed to create grouping rule")
	}

	h.logger.Info("created grouping rule", "id", rule.ID, "name", rule.Name)
	return Created(c, rule)
}

// List handles GET /v1/grouping-rules
// Returns all grouping rules.
func (h *GroupingRuleHandler) List(c *fiber.Ctx) error {
	rules, err := h.repo.List(c.Context())
	if err != nil {
		h.logger.Error("failed to list grouping rules", "error", err)
		return InternalError(c, "failed to list grouping rules")
	}

	return Success(c, rules)
}

// GetByID handles GET /v1/grouping-rules/:id
// Returns a single grouping rule by ID.
func (h *GroupingRuleHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	rule, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrGroupingRuleNotFound) {
			return NotFound(c, "grouping rule not found")
		}
		h.logger.Error("failed to get grouping rule", "id", id, "error", err)
		return InternalError(c, "failed to get grouping rule")
	}

	return Success(c, rule)
}

// Update handles PUT /v1/grouping-rules/:id
// Updates an existing grouping rule.
func (h *GroupingRuleHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	var req domain.UpdateGroupingRuleRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Debug("failed to parse request body", "error", err)
		return BadRequest(c, "invalid request body")
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		h.logger.Debug("validation failed", "error", err)
		return ValidationError(c, err.Error())
	}

	// Fetch existing grouping rule
	rule, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrGroupingRuleNotFound) {
			return NotFound(c, "grouping rule not found")
		}
		h.logger.Error("failed to get grouping rule", "id", id, "error", err)
		return InternalError(c, "failed to get grouping rule")
	}

	// Apply updates
	req.ApplyTo(rule)

	// Persist changes
	if err := h.repo.Update(c.Context(), rule); err != nil {
		h.logger.Error("failed to update grouping rule", "id", id, "error", err)
		return InternalError(c, "failed to update grouping rule")
	}

	h.logger.Info("updated grouping rule", "id", rule.ID)
	return Success(c, rule)
}

// Delete handles DELETE /v1/grouping-rules/:id
// Deletes a grouping rule.
func (h *GroupingRuleHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id is required")
	}

	if err := h.repo.Delete(c.Context(), id); err != nil {
		if errors.Is(err, domain.ErrGroupingRuleNotFound) {
			return NotFound(c, "grouping rule not found")
		}
		h.logger.Error("failed to delete grouping rule", "id", id, "error", err)
		return InternalError(c, "failed to delete grouping rule")
	}

	h.logger.Info("deleted grouping rule", "id", id)
	return NoContent(c)
}
