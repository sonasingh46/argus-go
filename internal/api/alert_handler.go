package api

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"argus-go/internal/domain"
	"argus-go/internal/store"
)

// AlertHandler handles HTTP requests for alert operations.
// For MVP, alerts are read-only through the API.
type AlertHandler struct {
	repo   store.AlertRepository
	logger *slog.Logger
}

// NewAlertHandler creates a new alert handler.
func NewAlertHandler(repo store.AlertRepository, logger *slog.Logger) *AlertHandler {
	return &AlertHandler{
		repo:   repo,
		logger: logger,
	}
}

// List handles GET /v1/alerts
// Returns alerts matching query parameters.
func (h *AlertHandler) List(c *fiber.Ctx) error {
	// Parse query parameters for filtering
	filter := domain.AlertFilter{
		EventManagerID: c.Query("event_manager_id"),
	}

	// Parse status filter
	if status := c.Query("status"); status != "" {
		filter.Status = domain.AlertStatus(status)
	}

	// Parse type filter
	if alertType := c.Query("type"); alertType != "" {
		filter.Type = domain.AlertType(alertType)
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	// Default limit if not specified
	if filter.Limit == 0 {
		filter.Limit = 100
	}

	alerts, err := h.repo.List(c.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list alerts", "error", err)
		return InternalError(c, "failed to list alerts")
	}

	return Success(c, alerts)
}

// GetByDedupKey handles GET /v1/alerts/:dedupKey
// Returns a single alert by its deduplication key.
func (h *AlertHandler) GetByDedupKey(c *fiber.Ctx) error {
	dedupKey := c.Params("dedupKey")
	if dedupKey == "" {
		return BadRequest(c, "dedupKey is required")
	}

	alert, err := h.repo.GetByDedupKey(c.Context(), dedupKey)
	if err != nil {
		if errors.Is(err, domain.ErrAlertNotFound) {
			return NotFound(c, "alert not found")
		}
		h.logger.Error("failed to get alert", "dedupKey", dedupKey, "error", err)
		return InternalError(c, "failed to get alert")
	}

	return Success(c, alert)
}

// GetChildren handles GET /v1/alerts/:dedupKey/children
// Returns all child alerts for a given parent alert.
func (h *AlertHandler) GetChildren(c *fiber.Ctx) error {
	dedupKey := c.Params("dedupKey")
	if dedupKey == "" {
		return BadRequest(c, "dedupKey is required")
	}

	// First verify the parent exists
	parent, err := h.repo.GetByDedupKey(c.Context(), dedupKey)
	if err != nil {
		if errors.Is(err, domain.ErrAlertNotFound) {
			return NotFound(c, "alert not found")
		}
		h.logger.Error("failed to get alert", "dedupKey", dedupKey, "error", err)
		return InternalError(c, "failed to get alert")
	}

	// Verify it's a parent alert
	if !parent.IsParent() {
		return BadRequest(c, "alert is not a parent alert")
	}

	// Get children
	children, err := h.repo.GetChildrenByParent(c.Context(), dedupKey)
	if err != nil {
		h.logger.Error("failed to get children", "parentDedupKey", dedupKey, "error", err)
		return InternalError(c, "failed to get children")
	}

	return Success(c, children)
}
