package store

import (
	"context"

	"argus-go/internal/domain"
)

// AlertRepository defines the interface for persistent alert storage.
// This is typically backed by PostgreSQL for production use.
type AlertRepository interface {
	// Create stores a new alert.
	Create(ctx context.Context, alert *domain.Alert) error

	// Update modifies an existing alert.
	Update(ctx context.Context, alert *domain.Alert) error

	// GetByID retrieves an alert by its database ID.
	GetByID(ctx context.Context, id string) (*domain.Alert, error)

	// GetByDedupKey retrieves an alert by its deduplication key.
	GetByDedupKey(ctx context.Context, dedupKey string) (*domain.Alert, error)

	// List retrieves alerts matching the filter criteria.
	List(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error)

	// GetChildrenByParent retrieves all child alerts for a given parent dedup key.
	GetChildrenByParent(ctx context.Context, parentDedupKey string) ([]*domain.Alert, error)

	// CountActiveChildren returns the count of active child alerts for a parent.
	CountActiveChildren(ctx context.Context, parentDedupKey string) (int, error)
}

// EventManagerRepository defines the interface for event manager persistence.
type EventManagerRepository interface {
	// Create stores a new event manager.
	Create(ctx context.Context, em *domain.EventManager) error

	// Update modifies an existing event manager.
	Update(ctx context.Context, em *domain.EventManager) error

	// Delete removes an event manager by ID.
	Delete(ctx context.Context, id string) error

	// GetByID retrieves an event manager by its ID.
	GetByID(ctx context.Context, id string) (*domain.EventManager, error)

	// List retrieves all event managers.
	List(ctx context.Context) ([]*domain.EventManager, error)
}

// GroupingRuleRepository defines the interface for grouping rule persistence.
type GroupingRuleRepository interface {
	// Create stores a new grouping rule.
	Create(ctx context.Context, rule *domain.GroupingRule) error

	// Update modifies an existing grouping rule.
	Update(ctx context.Context, rule *domain.GroupingRule) error

	// Delete removes a grouping rule by ID.
	Delete(ctx context.Context, id string) error

	// GetByID retrieves a grouping rule by its ID.
	GetByID(ctx context.Context, id string) (*domain.GroupingRule, error)

	// List retrieves all grouping rules.
	List(ctx context.Context) ([]*domain.GroupingRule, error)
}
